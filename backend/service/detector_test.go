package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIsURLStringRelative(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/a.jpg": true,
		"http://example.com/a.jpg":  true,
		"/i/123456.jpg":             true,
		"/api/random":               true,
		"data.urls.local":           false,
		"12345":                     false,
		"":                          false,
	}
	for in, want := range cases {
		if got := isURLString(in); got != want {
			t.Errorf("isURLString(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestHintPriorityPrefersDirectOverProxy(t *testing.T) {
	local := hintPriority("data.urls.local")
	proxy := hintPriority("data.urls.proxy")
	origin := hintPriority("data.origin")
	url := hintPriority("data.url")
	if local <= proxy {
		t.Errorf("expected local(%d) to outrank proxy(%d)", local, proxy)
	}
	if origin <= proxy {
		t.Errorf("expected origin(%d) to outrank proxy(%d)", origin, proxy)
	}
	if url < local {
		t.Errorf("expected url(%d) to be >= local(%d)", url, local)
	}
}

func TestDetectURLRejectsUnsafeProductionTargets(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1/", "https://10.0.0.1/", "http://[::1]/", "http://[::ffff:127.0.0.1]/",
		"http://169.254.1.1/", "http://224.0.0.1/", "ftp://example.com/", "http://user:pass@example.com/",
	} {
		if _, err := DetectURL(target); err == nil {
			t.Errorf("DetectURL(%q) succeeded; want rejection", target)
		}
	}
}

func TestDetectURLUsesInjectedClientForLocalFunctionalTest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"image_url":"https://cdn.example/image.jpg"}`))
	}))
	defer server.Close()

	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{}).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	result, err := detectURLWithClient(server.URL, client, allowLocalTestURL)
	if err != nil {
		t.Fatalf("detectURLWithClient: %v", err)
	}
	if result.RespType != "json" || result.FinalURL != server.URL || len(result.URLHints) != 1 || result.URLHints[0] != "image_url" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDetectTransportRejectsPrivateDNSAnswerBeforeDial(t *testing.T) {
	var resolved atomic.Int32
	var dialed atomic.Int32
	transport := newDetectTransportWithResolverAndDial(func(context.Context, string) ([]netip.Addr, error) {
		resolved.Add(1)
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}, func(context.Context, string, string) (net.Conn, error) {
		dialed.Add(1)
		return nil, fmt.Errorf("unexpected dial")
	})
	defer transport.CloseIdleConnections()

	_, err := transport.DialContext(context.Background(), "tcp", "rebind.example:80")
	if err == nil {
		t.Fatal("DialContext succeeded for a private DNS answer")
	}
	if resolved.Load() != 1 {
		t.Fatalf("resolver called %d times, want 1", resolved.Load())
	}
	if dialed.Load() != 0 {
		t.Fatalf("dial called %d times for a blocked answer, want 0", dialed.Load())
	}
}

func TestDetectTransportPinsDialToValidatedDNSAddress(t *testing.T) {
	var dialAddress string
	transport := newDetectTransportWithResolverAndDial(func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}, func(_ context.Context, network, address string) (net.Conn, error) {
		dialAddress = address
		client, server := net.Pipe()
		_ = server.Close()
		return client, nil
	})
	defer transport.CloseIdleConnections()
	conn, err := transport.DialContext(context.Background(), "tcp", "rebind.example:443")
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	_ = conn.Close()
	if dialAddress != "93.184.216.34:443" {
		t.Fatalf("dialed %q, want the validated DNS address", dialAddress)
	}
}

func TestDetectURLValidatesRedirectTargetBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Redirect(w, nil, "http://127.0.0.1/private", http.StatusFound)
	}))
	defer server.Close()

	// Use a public-looking hostname at the HTTP layer, while the injected dialer
	// routes it to the local test server. The redirect still goes through the
	// production URL validator and must be rejected before a second request.
	_, serverPort, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if strings.HasPrefix(address, "redirect-test.example:") {
			address = net.JoinHostPort("127.0.0.1", serverPort)
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	target := fmt.Sprintf("http://redirect-test.example:%s/start", serverPort)
	if _, err := detectURLWithClient(target, client, validateDetectURL); err == nil {
		t.Fatal("redirect to loopback succeeded; want rejection")
	}
	if requests.Load() != 1 {
		t.Fatalf("server received %d requests, want exactly the initial request", requests.Load())
	}
}

func TestDetectURLRejectsCredentialRedirectBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Redirect(w, nil, "http://user:secret@redirect-target.example/private", http.StatusFound)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if strings.HasPrefix(address, "redirect-test.example:") {
			address = net.JoinHostPort("127.0.0.1", port)
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	defer transport.CloseIdleConnections()
	target := fmt.Sprintf("http://redirect-test.example:%s/start", port)
	if _, err := detectURLWithClient(target, &http.Client{Transport: transport}, validateDetectURL); err == nil {
		t.Fatal("redirect containing credentials succeeded; want rejection")
	}
	if requests.Load() != 1 {
		t.Fatalf("server received %d requests, want exactly the initial request", requests.Load())
	}
}

func allowLocalTestURL(_ context.Context, target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil {
		return fmt.Errorf("invalid test URL")
	}
	return nil
}

func TestDetectURLRejectsOversizedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat(" ", maxDetectBodyBytes+1)))
	}))
	defer server.Close()
	result, err := detectURLWithClient(server.URL, server.Client(), allowLocalTestURL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Error, "exceeds") || result.BodyTree != nil {
		t.Fatalf("oversized response was accepted: %+v", result)
	}
}

func TestDetectTransportRejectsMixedDNSAnswers(t *testing.T) {
	dialed := false
	transport := newDetectTransportWithResolverAndDial(func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("10.0.0.1")}, nil
	}, func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, fmt.Errorf("unexpected dial")
	})
	if _, err := transport.DialContext(context.Background(), "tcp", "mixed.example:80"); err == nil || dialed {
		t.Fatalf("mixed DNS answers: err=%v, dialed=%v", err, dialed)
	}
}
