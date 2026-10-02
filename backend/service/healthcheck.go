package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
	"github.com/tidwall/gjson"
)

type HealthChecker struct {
	store      *store.Store
	client     *http.Client
	mu         sync.Mutex
	ticker     *time.Ticker
	quit       chan struct{}
	inFlight   chan struct{}
	lastResult []HealthResult
	lastRunAt  time.Time
}

type HealthResult struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	URL                string    `json:"url"`
	StatusCode         int       `json:"status_code"`
	LatencyMs          int64     `json:"latency_ms"`
	Available          bool      `json:"available"`
	Error              string    `json:"error,omitempty"`
	CheckedAt          time.Time `json:"checked_at"`
	CheckedEndpoints   int       `json:"checked_endpoints"`
	AvailableEndpoints int       `json:"available_endpoints"`
}

func NewHealthChecker(st *store.Store, proxyConfig ...*ProxyConfig) *HealthChecker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if len(proxyConfig) > 0 && proxyConfig[0] != nil {
		transport.Proxy = proxyConfig[0].Proxy
	}
	return &HealthChecker{store: st, client: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, quit: make(chan struct{})}
}

func (hc *HealthChecker) Start() {
	go hc.CheckAll()
	interval := 360
	if settings, err := hc.store.GetSettings(); err == nil && settings.HealthCheckInterval > 0 {
		interval = settings.HealthCheckInterval
	}
	hc.ticker = time.NewTicker(time.Duration(interval) * time.Minute)
	go func() {
		for {
			select {
			case <-hc.ticker.C:
				hc.CheckAll()
			case <-hc.quit:
				return
			}
		}
	}()
}
func (hc *HealthChecker) Stop() {
	if hc.ticker != nil {
		hc.ticker.Stop()
	}
	close(hc.quit)
	hc.client.CloseIdleConnections()
}

// Concurrent callers share the same run instead of probing every upstream again.
func (hc *HealthChecker) CheckAll() []HealthResult {
	hc.mu.Lock()
	if done := hc.inFlight; done != nil {
		hc.mu.Unlock()
		<-done
		return hc.GetLastResult()
	}
	done := make(chan struct{})
	hc.inFlight = done
	hc.mu.Unlock()
	defer func() { hc.mu.Lock(); hc.inFlight = nil; close(done); hc.mu.Unlock() }()
	sources, err := hc.store.ListSources()
	if err != nil {
		return hc.GetLastResult()
	}
	enabled := make([]model.Source, 0, len(sources))
	for _, src := range sources {
		if src.Enabled {
			enabled = append(enabled, src)
		}
	}
	results := make([]HealthResult, len(enabled))
	jobs := make(chan int)
	var workers sync.WaitGroup
	count := min(4, len(enabled))
	for i := 0; i < count; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				results[index] = hc.checkSource(enabled[index])
			}
		}()
	}
	for index := range enabled {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	hc.mu.Lock()
	hc.lastResult = results
	hc.lastRunAt = time.Now()
	hc.mu.Unlock()
	available := 0
	for _, r := range results {
		if r.Available {
			available++
		}
	}
	logger.System("health check: %d/%d enabled sources passed", available, len(results))
	return results
}
func (hc *HealthChecker) GetLastResult() []HealthResult {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if hc.lastResult == nil {
		return nil
	}
	return append([]HealthResult{}, hc.lastResult...)
}
func (hc *HealthChecker) LastRunAt() time.Time {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.lastRunAt
}

func healthTargets(src model.Source) []string {
	targets := []string{appendDefaultQuery(src.URL, src.DefaultQuery)}
	seen := map[string]bool{targets[0]: true}
	for _, param := range src.Params {
		if strings.TrimSpace(param.Key) == "" {
			continue
		}
		target, _ := resolveSubEndpoint(src.URL, strings.TrimSpace(param.Key), strings.TrimSpace(param.Value))
		target = appendDefaultQuery(target, src.DefaultQuery)
		if !seen[target] {
			targets = append(targets, target)
			seen[target] = true
		}
	}
	return targets
}

func (hc *HealthChecker) checkSource(src model.Source) HealthResult {
	result := HealthResult{ID: src.ID, Name: src.Name, URL: src.URL, CheckedAt: time.Now()}
	var failures []string
	for _, target := range healthTargets(src) {
		probe := hc.checkEndpoint(src, target)
		result.CheckedEndpoints++
		if probe.Available {
			result.AvailableEndpoints++
			if !result.Available {
				result.URL = target
				result.StatusCode = probe.StatusCode
				result.LatencyMs = probe.LatencyMs
			}
			result.Available = true
		} else {
			failures = append(failures, probe.Error)
			if !result.Available {
				result.URL = target
				result.StatusCode = probe.StatusCode
				result.LatencyMs = probe.LatencyMs
			}
		}
	}
	if len(failures) > 0 {
		result.Error = failures[0]
		if result.CheckedEndpoints > 1 {
			result.Error = fmt.Sprintf("%d/%d 个地址检测通过；首个失败原因：%s", result.AvailableEndpoints, result.CheckedEndpoints, failures[0])
		}
	}
	if err := hc.store.RecordHealthResult(src.ID, result.Available, result.LatencyMs); err != nil {
		logger.Error("record health result for source %d: %v", src.ID, err)
	}
	return result
}

func (hc *HealthChecker) checkEndpoint(src model.Source, target string) HealthResult {
	start := time.Now()
	result := HealthResult{URL: target}
	src.URL = target
	timeout := 3 * time.Second
	if settings, err := hc.store.GetSettings(); err == nil && settings.Timeout > 0 {
		timeout = time.Duration(settings.Timeout) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := newSourceRequest(ctx, src, "")
	if err != nil {
		result.Error = "地址格式不正确"
		return result
	}
	resp, err := hc.client.Do(req)
	if err != nil {
		result.LatencyMs = time.Since(start).Milliseconds()
		result.Error = fmt.Sprintf("请求失败：%v", err)
		return result
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	err = validateHealthResponse(resp, src)
	result.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Available = true
	}
	return result
}

// Validate the API response, not the download of every image returned by that API.
func validateHealthResponse(resp *http.Response, src model.Source) error {
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && !isRedirectStatus(resp.StatusCode) {
		return fmt.Errorf("HTTP %d：%s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	if isRedirectStatus(resp.StatusCode) && resp.Header.Get("Location") != "" {
		return validateHealthURL(src.URL, resp.Header.Get("Location"))
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/") {
		return nil
	}
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return fmt.Errorf("读取响应失败：%v", err)
	}
	if len(body) > maxBody {
		return fmt.Errorf("响应超过 1 MiB，未继续检测")
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") || src.RespType == "json" {
		path := src.JsonPath
		if path == "" {
			path = "url"
		}
		value := gjson.GetBytes(body, path)
		if !gjson.ValidBytes(body) || value.Type != gjson.String || strings.TrimSpace(value.String()) == "" {
			return fmt.Errorf("JSON 字段 %s 中没有图片地址", path)
		}
		return validateHealthURL(src.URL, value.String())
	}
	text := strings.TrimSpace(string(body))
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return validateHealthURL(src.URL, text)
	}
	return fmt.Errorf("响应不是图片，也没有可用的图片地址")
}
func validateHealthURL(base, target string) error {
	resolved := resolveURL(base, target)
	u, err := url.Parse(resolved)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("返回的图片地址不是有效的 HTTP/HTTPS URL")
	}
	return nil
}
