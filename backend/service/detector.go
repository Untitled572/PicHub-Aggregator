package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type DetectResult struct {
	RespType string            `json:"resp_type"`
	Headers  map[string]string `json:"headers"`
	BodyTree interface{}       `json:"body_tree,omitempty"`
	URLHints []string          `json:"url_hints,omitempty"`
	FinalURL string            `json:"final_url,omitempty"`
	Error    string            `json:"error,omitempty"`
}

const (
	maxDetectRedirects = 5
	maxDetectBodyBytes = 2 << 20
	detectTimeout      = 10 * time.Second
)

func DetectURL(targetURL string) (*DetectResult, error) {
	client := &http.Client{Transport: newDetectTransport(), Timeout: detectTimeout}
	return detectURLWithClient(targetURL, client, validateDetectURL)
}

// detectURLWithClient allows tests to inject a local-only dialer while keeping
// production URL validation and redirect handling explicit.
func detectURLWithClient(targetURL string, client *http.Client, validate func(context.Context, *url.URL) error) (*DetectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), detectTimeout)
	defer cancel()
	clientCopy := *client
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	result := &DetectResult{
		RespType: "unknown",
		Headers:  make(map[string]string),
	}

	current := targetURL
	redirects := 0

	for {
		parsed, err := url.Parse(current)
		if err != nil {
			return nil, fmt.Errorf("parse URL: %w", err)
		}
		if err := validate(ctx, parsed); err != nil {
			return nil, fmt.Errorf("unsafe URL: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("User-Agent", "PicHub-Aggregator/1.0")

		resp, err := clientCopy.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// 跟随重定向, 以最终响应判定类型
		loc := resp.Header.Get("Location")
		if isRedirectStatus(resp.StatusCode) && loc != "" {
			resp.Body.Close()
			nextURL := resolveURL(current, loc)
			parsedNext, err := url.Parse(nextURL)
			if err != nil {
				return nil, fmt.Errorf("parse redirect URL: %w", err)
			}
			if err := validate(ctx, parsedNext); err != nil {
				return nil, fmt.Errorf("unsafe redirect URL: %w", err)
			}
			redirects++
			if redirects > maxDetectRedirects {
				result.RespType = "redirect"
				result.URLHints = []string{parsedNext.String()}
				result.FinalURL = current
				return result, nil
			}
			current = parsedNext.String()
			continue
		}

		defer resp.Body.Close()

		for k, v := range resp.Header {
			if len(v) > 0 {
				result.Headers[k] = v[0]
			}
		}
		result.FinalURL = current

		ct := resp.Header.Get("Content-Type")

		// 3xx 但无 Location: 保留现有行为, 直接读 body
		if isRedirectStatus(resp.StatusCode) {
			result.RespType = "redirect"
			body, err := readDetectBody(resp.Body)
			if err != nil {
				result.Error = fmt.Sprintf("read body: %v", err)
				return result, nil
			}
			text := strings.TrimSpace(string(body))
			if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
				result.URLHints = []string{text}
			}
			return result, nil
		}

		// 最终为图片: 若经历重定向则归类 redirect (直链源), 否则 image
		if strings.HasPrefix(ct, "image/") {
			if redirects > 0 {
				result.RespType = "redirect"
			} else {
				result.RespType = "image"
			}
			result.URLHints = []string{current}
			return result, nil
		}

		if strings.HasPrefix(ct, "application/json") {
			result.RespType = "json"
			body, err := readDetectBody(resp.Body)
			if err != nil {
				result.Error = fmt.Sprintf("read body: %v", err)
				return result, nil
			}
			if json.Valid(body) {
				var bodyTree interface{}
				if err := json.Unmarshal(body, &bodyTree); err != nil {
					result.Error = fmt.Sprintf("json parse: %v", err)
					return result, nil
				}
				result.BodyTree = bodyTree
				result.URLHints = findURLFields("", body, 0)
			}
			return result, nil
		}

		body, err := readDetectBody(resp.Body)
		if err != nil {
			result.Error = fmt.Sprintf("read body: %v", err)
			return result, nil
		}
		if len(body) > 0 && json.Valid(body) {
			result.RespType = "json"
			var bodyTree interface{}
			if err := json.Unmarshal(body, &bodyTree); err != nil {
				result.Error = fmt.Sprintf("json parse: %v", err)
				return result, nil
			}
			result.BodyTree = bodyTree
			result.URLHints = findURLFields("", body, 0)
			return result, nil
		}

		return result, nil
	}
}

func readDetectBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxDetectBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDetectBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxDetectBodyBytes)
	}
	return data, nil
}

var blockedDetectPrefixes = func() []netip.Prefix {
	raw := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "::ffff:0:0/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/23",
		"2001:db8::/32", "2002::/16", "3fff::/20", "fc00::/7", "fe80::/10", "ff00::/8",
	}
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, value := range raw {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}()

func validateDetectURL(ctx context.Context, target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") {
		return fmt.Errorf("only http and https URLs are allowed")
	}
	if target.User != nil {
		return fmt.Errorf("URL credentials are not allowed")
	}
	if target.Hostname() == "" {
		return fmt.Errorf("URL host is required")
	}
	if ip, err := netip.ParseAddr(target.Hostname()); err == nil {
		return validateDetectIP(ip)
	}
	return nil
}

func validateDetectIP(ip netip.Addr) error {
	if ip.Is4In6() {
		return fmt.Errorf("IPv4-mapped IPv6 addresses are not allowed")
	}
	if ip.Zone() != "" {
		return fmt.Errorf("scoped IP addresses are not allowed")
	}
	if !ip.IsValid() || !ip.IsGlobalUnicast() {
		return fmt.Errorf("non-global IP address is not allowed")
	}
	for _, prefix := range blockedDetectPrefixes {
		if prefix.Contains(ip) {
			return fmt.Errorf("special-use IP address is not allowed")
		}
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return fmt.Errorf("non-global IPv6 address is not allowed")
	}
	return nil
}

func newDetectTransport() *http.Transport {
	return newDetectTransportWithResolver(func(ctx context.Context, host string) ([]netip.Addr, error) {
		return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	})
}

func newDetectTransportWithResolver(resolve func(context.Context, string) ([]netip.Addr, error)) *http.Transport {
	return newDetectTransportWithResolverAndDial(resolve, (&net.Dialer{}).DialContext)
}

func newDetectTransportWithResolverAndDial(resolve func(context.Context, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) *http.Transport {
	return &http.Transport{
		Proxy: nil, // Prevent environment proxy settings from bypassing destination checks.
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if ip, err := netip.ParseAddr(host); err == nil {
				if err := validateDetectIP(ip); err != nil {
					return nil, err
				}
				return dial(ctx, network, net.JoinHostPort(ip.String(), port))
			}
			ips, err := resolve(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve destination: %w", err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("destination has no IP addresses")
			}
			// Validate the complete DNS answer set before choosing any candidate.
			// This prevents an allowed answer from masking a private one.
			for _, ip := range ips {
				if err := validateDetectIP(ip); err != nil {
					return nil, err
				}
			}
			var lastErr error
			for _, ip := range ips {
				conn, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
		DisableKeepAlives: true,
	}
}

func isRedirectStatus(code int) bool {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308
}

func findURLFields(prefix string, data []byte, depth int) []string {
	if depth > 10 {
		return nil
	}
	var hints []string
	result := gjson.GetBytes(data, "@this")
	result.ForEach(func(key, value gjson.Result) bool {
		var fullPath string
		if prefix == "" {
			fullPath = key.String()
		} else {
			fullPath = prefix + "." + key.String()
		}
		if value.IsObject() || value.IsArray() {
			subHints := findURLFields(fullPath, []byte(value.Raw), depth+1)
			hints = append(hints, subHints...)
		} else if value.Type == gjson.String {
			str := value.String()
			if isURLString(str) {
				hints = append(hints, fullPath)
			}
		}
		return true
	})
	sort.SliceStable(hints, func(i, j int) bool {
		return hintPriority(hints[i]) > hintPriority(hints[j])
	})
	return hints
}

func isURLString(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "/")
}

func hintPriority(path string) int {
	lower := strings.ToLower(path)
	parts := strings.Split(lower, ".")
	key := ""
	if len(parts) > 0 {
		key = parts[len(parts)-1]
	}
	// 去除引号: 0.['url'] → url
	key = strings.Trim(key, "'\"[]")

	// 直接图片/本地资源优先
	switch {
	case key == "url" || key == "img_url" || key == "image_url":
		return 10
	case key == "local" || key == "origin" || key == "original" || key == "src" || key == "file" || key == "file_url":
		return 9
	case key == "image" || key == "img" || key == "download" || key == "pic" || key == "thumbnail":
		return 8
	case strings.Contains(key, "img") || strings.Contains(key, "image") || strings.Contains(key, "src"):
		return 7
	case key == "detail" || key == "page" || key == "link" || key == "href":
		return 3
	case key == "proxy" || strings.Contains(key, "proxy"):
		return 2
	}

	return 5
}
