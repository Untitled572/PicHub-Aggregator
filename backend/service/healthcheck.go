package service

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

type HealthChecker struct {
	store      *store.Store
	client     *http.Client
	mu         sync.Mutex
	ticker     *time.Ticker
	quit       chan struct{}
	lastResult []HealthResult
	lastRunAt  time.Time
}

type HealthResult struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	Available  bool   `json:"available"`
	Error      string `json:"error,omitempty"`
}

func NewHealthChecker(st *store.Store) *HealthChecker {
	return &HealthChecker{
		store: st,
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		quit: make(chan struct{}),
	}
}

func (hc *HealthChecker) Start() {
	hc.CheckAll()
	hc.ticker = time.NewTicker(5 * time.Minute)
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
}

func (hc *HealthChecker) CheckAll() []HealthResult {
	sources, err := hc.store.ListSources()
	if err != nil {
		return nil
	}

	results := make([]HealthResult, 0, len(sources))
	for _, src := range sources {
		result := hc.checkSource(src)
		results = append(results, result)
	}

	hc.mu.Lock()
	hc.lastResult = results
	hc.lastRunAt = time.Now()
	hc.mu.Unlock()

	return results
}

func (hc *HealthChecker) GetLastResult() []HealthResult {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.lastResult
}

func (hc *HealthChecker) LastRunAt() time.Time {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.lastRunAt
}

func (hc *HealthChecker) checkSource(src model.Source) HealthResult {
	start := time.Now()
	resp, err := hc.client.Get(src.URL)
	latency := time.Since(start).Milliseconds()

	result := HealthResult{
		ID:        src.ID,
		Name:      src.Name,
		URL:       src.URL,
		LatencyMs: latency,
	}

	if err != nil {
		result.Error = err.Error()
		result.Available = false
		hc.store.IncrementFailCount(src.ID)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		result.Available = true
		hc.store.ResetFailCount(src.ID)
	} else {
		result.Available = false
		hc.store.IncrementFailCount(src.ID)
	}

	latencyMs := latency
	if latencyMs == 0 {
		latencyMs = 1
	}

	src.AvgLatency = (src.AvgLatency + latencyMs) / 2
	src.SuccessRate = calculateSuccessRate(src, result.Available)
	if err := hc.store.UpdateSource(&src); err != nil {
		result.Error = fmt.Sprintf("update failed: %v", err)
	}

	return result
}

func calculateSuccessRate(src model.Source, success bool) float64 {
	if success {
		return min(100.0, src.SuccessRate+5.0)
	}
	return max(0.0, src.SuccessRate-10.0)
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
