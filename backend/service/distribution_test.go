package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pichub/backend/model"
)

func TestDistributionCancellationStopsImageDownload(t *testing.T) {
	st := healthTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image" {
			close(started)
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"url":"/image"}`)
	}))
	defer upstream.Close()
	addHealthSource(t, st, model.Source{Name: "source", URL: upstream.URL, RespType: "json", Enabled: true, Weight: 1})
	e := &Engine{store: st, imageStore: NewImageStore(st, t.TempDir(), nil), httpClient: upstream.Client(), demandTracker: NewDemandTracker()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, _, err := e.RandomImageContext(ctx, "", "json", "", ""); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("image download never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("download continued after client canceled")
	}
}

func TestReplenishUsesPoolStockInsteadOfHistoricalCache(t *testing.T) {
	st, images, _, upstream := newTestImageStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode, settings.PoolSize = true, 2
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	src := addHealthSource(t, st, model.Source{Name: "source", URL: upstream.URL, Enabled: true, Weight: 1})
	if _, err := images.DownloadAndStore(upstream.URL, upstream.URL, src.ID, src.Name, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	e := &Engine{store: st, imageStore: images, httpClient: upstream.Client(), distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker()}
	e.replenishPool()
	if got := e.distPool.SourceCounts()[src.ID]; got != 1 {
		t.Fatalf("prefetch stock = %d, want 1 despite existing historical cache", got)
	}
}

func TestPrefetchPrioritizesColdTagWithIsolatedInventory(t *testing.T) {
	st, images, _, upstream := newTestImageStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode, settings.PoolSize = true, 120
	settings.BoundTags = []string{"hot"}
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateTags([]model.Tag{{ID: "hot", Name: "Hot"}, {ID: "quiet", Name: "Quiet", Exclusive: true}}); err != nil {
		t.Fatal(err)
	}
	hot := addHealthSource(t, st, model.Source{Name: "hot", URL: upstream.URL, Enabled: true, Weight: 1, Categories: []string{"hot"}})
	quiet := addHealthSource(t, st, model.Source{Name: "quiet", URL: upstream.URL, Enabled: true, Weight: 1, Categories: []string{"quiet"}})
	e := &Engine{store: st, imageStore: images, httpClient: upstream.Client(), distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker()}
	for i := 0; i < 16; i++ {
		img, err := images.DownloadAndStore(upstream.URL, upstream.URL, hot.ID, hot.Name, hot.Categories, nil, true)
		if err != nil {
			t.Fatal(err)
		}
		e.distPool.Push(&PoolEntry{FileID: img.FileID, ImageID: img.ID, SourceID: hot.ID, Categories: hot.Categories})
	}
	for i := 0; i < 15; i++ {
		// Move past the request interval without making the test sleep.
		e.nextPrefetch = time.Time{}
		e.prefetchCooldown = nil
		e.replenishPool()
	}
	counts := e.distPool.SourceCounts()
	if counts[quiet.ID] != 15 || counts[hot.ID] != 16 {
		t.Fatalf("wrong reserved stock: %v", counts)
	}
	for i := 0; i < 15; i++ {
		result, _, err := e.RandomImage("quiet", "json", "", "")
		if err != nil || result.SourceID != quiet.ID || result.FileID == "" {
			t.Fatalf("quiet request %d: result=%+v err=%v", i, result, err)
		}
	}
	if e.distPool.SourceCounts()[hot.ID] != 16 {
		t.Fatal("cold tag consumed normal inventory")
	}
}

func TestPrefetchRateLimitAlsoPausesBranchesOnSameHost(t *testing.T) {
	st := healthTestStore(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	first := model.Source{ID: 1, URL: srv.URL + "/one", Weight: 1}
	other := model.Source{ID: 2, URL: srv.URL + "/two", Weight: 1}
	e := &Engine{store: st, httpClient: srv.Client()}
	e.fetchPrefetch([]model.Source{first}, nil, nil, 15)
	e.fetchPrefetch([]model.Source{other}, nil, nil, 15)
	if calls != 1 {
		t.Fatalf("upstream calls=%d, want 1", calls)
	}
	if remaining := time.Until(e.prefetchCooldown[prefetchKeys(first)[1]]); remaining < 119*time.Second {
		t.Fatalf("Retry-After not respected: %v", remaining)
	}
}
