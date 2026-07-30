package service

import (
	"net/http"
	"net/url"
	"sync"
)

type ProxyConfig struct {
	mu        sync.RWMutex
	enabled   bool
	rawURL    string
	parsedURL *url.URL
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		parsedURL: nil,
	}
}

func (pc *ProxyConfig) Update(enabled bool, urlStr string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.enabled = enabled
	pc.rawURL = urlStr
	if urlStr != "" {
		pc.parsedURL, _ = url.Parse(urlStr)
	} else {
		pc.parsedURL = nil
	}
}

func (pc *ProxyConfig) Proxy(req *http.Request) (*url.URL, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if pc.enabled && pc.parsedURL != nil {
		return pc.parsedURL, nil
	}
	return http.ProxyFromEnvironment(req)
}
