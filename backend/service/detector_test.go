package service

import "testing"

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
