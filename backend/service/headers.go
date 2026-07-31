package service

import (
	"net/http"
	"net/url"
	"strings"
)

const defaultBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

func hasHeader(headers map[string]string, name string) bool {
	for k := range headers {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

func buildImageRequestHeaders(sourceURL string, headers map[string]string) http.Header {
	h := make(http.Header)

	h.Set("User-Agent", defaultBrowserUA)

	referer := ""
	if headers != nil {
		referer = headers["Referer"]
	}
	if referer == "" {
		if u, err := url.Parse(sourceURL); err == nil && u.Host != "" && u.Scheme != "" {
			referer = u.Scheme + "://" + u.Host
		}
	}
	if referer != "" {
		h.Set("Referer", referer)
	}

	if headers != nil {
		for k, v := range headers {
			if strings.EqualFold(k, "Referer") || strings.EqualFold(k, "User-Agent") {
				continue
			}
			h.Set(k, v)
		}
	}
	return h
}
