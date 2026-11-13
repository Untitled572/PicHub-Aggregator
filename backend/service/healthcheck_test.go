package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

func healthTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}
func addHealthSource(t *testing.T, st *store.Store, src model.Source) model.Source {
	t.Helper()
	id, err := st.CreateSource(&src)
	if err != nil {
		t.Fatal(err)
	}
	src.ID = id
	return src
}
func TestHealthResponseValidation(t *testing.T) {
	cases := []struct {
		name               string
		code               int
		ct, body, location string
		src                model.Source
		want               bool
	}{
		{name: "image", code: 200, ct: "image/jpeg", want: true},
		{name: "redirect", code: 302, location: "/photo.jpg", want: true},
		{name: "json path", code: 200, ct: "application/json", body: `{"data":{"image":"/photo.jpg"}}`, src: model.Source{RespType: "json", JsonPath: "data.image"}, want: true},
		{name: "404 is failure", code: 404, ct: "image/jpeg"},
		{name: "auth required", code: 401},
		{name: "rate limited", code: 429},
		{name: "server error", code: 500},
		{name: "html landing page", code: 200, ct: "text/html", body: "<html>Home</html>"},
		{name: "json empty url", code: 200, ct: "application/json", body: `{"url":""}`},
		{name: "json wrong path", code: 200, ct: "application/json", body: `{"error":"unauthorized"}`},
		{name: "json object url", code: 200, ct: "application/json", body: `{"url":{"a":1}}`},
		{name: "unsupported scheme", code: 302, location: "file:///etc/passwd"},
		{name: "oversized", code: 200, ct: "application/json", body: strings.Repeat(" ", (1<<20)+1)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if tt.ct != "" {
				w.Header().Set("Content-Type", tt.ct)
			}
			if tt.location != "" {
				w.Header().Set("Location", tt.location)
			}
			w.WriteHeader(tt.code)
			w.WriteString(tt.body)
			tt.src.URL = "https://example.test/api"
			err := validateHealthResponse(w.Result(), tt.src)
			if (err == nil) != tt.want {
				t.Fatalf("err=%v want available=%v", err, tt.want)
			}
		})
	}
}
func TestHealthUsesConfiguredRequestAndBranches(t *testing.T) {
	st := healthTestStore(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" || r.Header.Get("User-Agent") != "health-test" || r.URL.Query().Get("format") != "json" {
			t.Error("configured headers/default query missing")
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/base" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"url":"/image.jpg"}}`)
	}))
	defer upstream.Close()
	src := addHealthSource(t, st, model.Source{Name: "branches", URL: upstream.URL + "/base", DefaultQuery: "format=json", RespType: "json", JsonPath: "data.url", Enabled: true, FailCount: 3, Status: "error", Headers: map[string]string{"Authorization": "Bearer test", "User-Agent": "health-test"}, Params: []model.QueryParam{{Key: "/working"}, {Key: "/working"}}})
	result := NewHealthChecker(st).checkSource(src)
	if !result.Available || result.CheckedEndpoints != 2 || result.AvailableEndpoints != 1 || result.Error == "" {
		t.Fatalf("result=%+v", result)
	}
	saved, _ := st.GetSource(src.ID)
	if saved.FailCount != 0 || saved.Status != "normal" {
		t.Fatalf("source=%+v", saved)
	}
}
func TestHealthFailureKeepsEditsAndIncrementsCount(t *testing.T) {
	st := healthTestStore(t)
	var id int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current, _ := st.GetSource(id)
		current.Name = "edited during probe"
		current.Enabled = false
		current.Headers = map[string]string{"X-Edited": "yes"}
		if err := st.UpdateSource(current); err != nil {
			t.Error(err)
		}
		w.WriteHeader(500)
	}))
	defer upstream.Close()
	src := addHealthSource(t, st, model.Source{Name: "original", URL: upstream.URL, Enabled: true, FailCount: 2, SuccessRate: 80})
	id = src.ID
	result := NewHealthChecker(st).checkSource(src)
	if result.Available {
		t.Fatal("500 passed")
	}
	saved, _ := st.GetSource(id)
	if saved.FailCount != 3 || saved.Enabled || saved.Name != "edited during probe" || saved.Headers["X-Edited"] != "yes" {
		t.Fatalf("source=%+v", saved)
	}
}
func TestHealthUsesProxy(t *testing.T) {
	st := healthTestStore(t)
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.Header().Set("Content-Type", "image/png") }))
	defer proxy.Close()
	cfg := NewProxyConfig()
	cfg.Update(true, proxy.URL)
	src := addHealthSource(t, st, model.Source{URL: "http://unreachable.invalid/image", Enabled: true})
	r := NewHealthChecker(st, cfg).checkSource(src)
	if !r.Available || hits.Load() != 1 {
		t.Fatalf("result=%+v hits=%d", r, hits.Load())
	}
}
func TestHealthSkipsDisabledAndSharesConcurrentRun(t *testing.T) {
	st := healthTestStore(t)
	var hits atomic.Int32
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		w.Header().Set("Content-Type", "image/jpeg")
	}))
	defer upstream.Close()
	addHealthSource(t, st, model.Source{URL: upstream.URL, Enabled: false})
	addHealthSource(t, st, model.Source{URL: upstream.URL, Enabled: true})
	hc := NewHealthChecker(st)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); hc.CheckAll() }()
	<-entered
	started := make(chan struct{})
	wg.Add(1)
	go func() { defer wg.Done(); close(started); hc.CheckAll() }()
	<-started
	time.Sleep(30 * time.Millisecond)
	close(release)
	wg.Wait()
	if hits.Load() != 1 || len(hc.GetLastResult()) != 1 {
		t.Fatalf("hits=%d results=%d", hits.Load(), len(hc.GetLastResult()))
	}
}
func TestNewSourceRequestDefaultUA(t *testing.T) {
	req, err := newSourceRequest(context.Background(), model.Source{URL: "https://example.test"}, "")
	if err != nil || req.Header.Get("User-Agent") != defaultBrowserUA {
		t.Fatalf("req=%v err=%v", req, err)
	}
}

func setupPrefetch(t *testing.T, st *store.Store, images *ImageStore, hc *HealthChecker, apiURL string) *Engine {
	t.Helper()
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode = true
	settings.PoolSize = 1
	settings.MinResolution = "0"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	addHealthSource(t, st, model.Source{Name: "prefetch", URL: apiURL, Enabled: true, Weight: 1, Status: "normal"})
	e := NewEngine(st, nil, images, nil, nil, hc)
	e.httpClient = http.DefaultClient
	return e
}

func TestPrefetchHealthSuccessAppearsInHealthResults(t *testing.T) {
	st, images, _, server := newTestImageStore(t)
	hc := NewHealthChecker(st)
	e := setupPrefetch(t, st, images, hc, server.URL)
	e.replenishPool()
	results := hc.GetLastResult()
	if e.distPool.Size() != 1 || len(results) != 1 || !results[0].Available || results[0].Origin != "cache" || results[0].Reason != "分发池预取成功" || results[0].CheckedAt.IsZero() {
		t.Fatalf("pool=%d results=%+v", e.distPool.Size(), results)
	}
	if results[0].LatencyMs < 0 {
		t.Fatalf("invalid elapsed time: %+v", results[0])
	}
	saved, err := st.GetSource(results[0].ID)
	if err != nil || saved.FailCount != 0 || saved.Status != "normal" {
		t.Fatalf("source=%+v err=%v", saved, err)
	}
	encoded, err := json.Marshal(results[0])
	if err != nil || !strings.Contains(string(encoded), `"origin":"cache"`) {
		t.Fatalf("cache origin not serialized: %s err=%v", encoded, err)
	}
}

func TestPrefetchHealthFailureAndNoSampleCases(t *testing.T) {
	t.Run("download validation failure", func(t *testing.T) {
		st := healthTestStore(t)
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api" {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"url":"/image"}`)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "not an image")
		}))
		defer api.Close()
		imageStore := NewImageStore(st, filepath.Join(t.TempDir(), "cache"), nil)
		hc := NewHealthChecker(st)
		e := setupPrefetch(t, st, imageStore, hc, api.URL+"/api")
		e.replenishPool()
		results := hc.GetLastResult()
		if e.distPool.Size() != 0 || len(results) != 1 || results[0].Available || results[0].Origin != "cache" || !strings.Contains(results[0].Error, "校验图片失败") {
			t.Fatalf("pool=%d results=%+v", e.distPool.Size(), results)
		}
		saved, _ := st.GetSource(results[0].ID)
		if saved.FailCount != 1 {
			t.Fatalf("failed cache validation did not update health fields: %+v", saved)
		}
	})
	t.Run("no candidate", func(t *testing.T) {
		st := healthTestStore(t)
		settings, _ := st.GetSettings()
		settings.ProxyMode = true
		settings.PoolSize = 1
		st.UpdateSettings(settings)
		hc := NewHealthChecker(st)
		e := NewEngine(st, nil, nil, nil, nil, hc)
		e.replenishPool()
		if got := hc.GetLastResult(); len(got) != 0 {
			t.Fatalf("no candidate recorded sample: %+v", got)
		}
	})
	t.Run("existing pool has capacity", func(t *testing.T) {
		st := healthTestStore(t)
		settings, _ := st.GetSettings()
		settings.ProxyMode = true
		settings.PoolSize = 1
		st.UpdateSettings(settings)
		hc := NewHealthChecker(st)
		e := NewEngine(st, nil, nil, nil, nil, hc)
		e.distPool.Push(&PoolEntry{FileID: "already-there", SourceID: 1, SourceName: "old pool"})
		result, _, err := e.randomImage("", "json", "", "")
		if err != nil || result == nil || result.SourceName != "old pool" {
			t.Fatalf("old pool hit failed: result=%+v err=%v", result, err)
		}
		if got := hc.GetLastResult(); len(got) != 0 {
			t.Fatalf("old pool hit recorded sample: %+v", got)
		}
	})
}

func TestCheckAllKeepsNewerCacheResult(t *testing.T) {
	st := healthTestStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "image/png") }))
	defer server.Close()
	src := addHealthSource(t, st, model.Source{Name: "probe", URL: server.URL, Enabled: true})
	hc := NewHealthChecker(st)
	newer := HealthResult{ID: src.ID, Name: src.Name, URL: server.URL, Available: true, CheckedAt: time.Now().Add(time.Second), Origin: "cache", Reason: "分发池预取成功"}
	hc.RecordCacheResult(newer)
	hc.CheckAll()
	got := hc.GetLastResult()
	if len(got) != 1 || got[0].Origin != "cache" || got[0].Reason != newer.Reason {
		t.Fatalf("active run overwrote cache result: %+v", got)
	}
}

func TestDisabledPeriodicHealthKeepsStartupCheck(t *testing.T) {
	st := healthTestStore(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "image/png") }))
	defer upstream.Close()
	addHealthSource(t, st, model.Source{Name: "source", URL: upstream.URL, Enabled: true})
	settings, _ := st.GetSettings()
	settings.HealthCheckInterval = -1
	st.UpdateSettings(settings)
	hc := NewHealthChecker(st)
	hc.Start()
	defer hc.Stop()
	if hc.ticker != nil {
		t.Fatal("-1 created periodic ticker")
	}
	deadline := time.Now().Add(time.Second)
	for hc.GetLastResult() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(hc.GetLastResult()) != 1 {
		t.Fatal("startup check was removed")
	}
	settings, _ = st.GetSettings()
	settings.HealthCheckInterval = 15
	st.UpdateSettings(settings)
	positive := NewHealthChecker(st)
	positive.Start()
	defer positive.Stop()
	if positive.ticker == nil {
		t.Fatal("positive interval no longer schedules checks")
	}
	deadline = time.Now().Add(time.Second)
	for positive.GetLastResult() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	positive.CheckAll()
	hc.CheckAll()
}
