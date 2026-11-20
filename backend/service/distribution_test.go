package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
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
	for i := 0; i < 120; i++ {
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
		e.prefetchPace = nil
		e.replenishPool()
	}
	status, err := e.PrefetchStatus()
	if err != nil || status.Count != 120 || status.Target != 120 {
		t.Fatalf("wrong status: %+v err=%v", status, err)
	}
	var quietTag PrefetchTagStock
	for _, tag := range status.ByTag {
		if tag.ID == "quiet" {
			quietTag = tag
		}
	}
	if quietTag.Count != 15 || quietTag.Target != 15 || quietTag.Square != 15 {
		t.Fatalf("wrong cold tag status: %+v", quietTag)
	}
	counts := e.distPool.SourceCounts()
	if counts[quiet.ID] != 15 || counts[hot.ID] != 105 {
		t.Fatalf("wrong reserved stock: %v", counts)
	}
	for i := 0; i < 15; i++ {
		result, _, err := e.RandomImage("quiet", "json", "", "")
		if err != nil || result.SourceID != quiet.ID || result.FileID == "" {
			t.Fatalf("quiet request %d: result=%+v err=%v", i, result, err)
		}
	}
	if e.distPool.SourceCounts()[hot.ID] != 105 {
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

func TestUnlimitedPrefetchSkipsSpacingButRespectsUpstreamRefusal(t *testing.T) {
	st := healthTestStore(t)
	calls := make(chan struct{}, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls <- struct{}{}
		if len(calls) > 1 {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()
	e := &Engine{store: st, httpClient: upstream.Client()}
	src := model.Source{ID: 1, Name: "self-hosted", URL: upstream.URL, Weight: 1}
	e.fetchPrefetch([]model.Source{src}, nil, nil, 15)
	e.fetchPrefetch([]model.Source{src}, nil, nil, 15)
	if len(calls) != 1 {
		t.Fatal("limited source started a second request")
	}
	src.PrefetchUnlimited = true
	e.fetchPrefetch([]model.Source{src}, nil, nil, 15)
	if len(calls) != 2 {
		t.Fatal("unlimited source was blocked by normal pacing")
	}
	e.fetchPrefetch([]model.Source{src}, nil, nil, 15)
	if len(calls) != 2 {
		t.Fatal("unlimited source ignored HTTP 429")
	}
}

func TestRestorePrefetchAfterReopeningDatabase(t *testing.T) {
	st, images, dbPath, upstream := newTestImageStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode = true
	settings.PoolSize = 2
	if err = st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	normal := addHealthSource(t, st, model.Source{Name: "current", URL: upstream.URL, Enabled: true})
	disabled := addHealthSource(t, st, model.Source{Name: "disabled", URL: upstream.URL})
	stopped := addHealthSource(t, st, model.Source{Name: "stopped", URL: upstream.URL, Enabled: true, Status: "error"})
	cache := func(src model.Source, pooled bool) *CachedImageInfo {
		t.Helper()
		info, err := images.DownloadAndStore(upstream.URL, upstream.URL, src.ID, src.Name, []string{"sample"}, nil, pooled)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	older := cache(normal, true)
	first := cache(normal, true)
	second := cache(normal, true)
	consumed := cache(normal, false)
	disabledImage := cache(disabled, true)
	stoppedImage := cache(stopped, true)
	missing := cache(normal, true)
	missingPath, _, err := images.GetImage(missing.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(missingPath); err != nil {
		t.Fatal(err)
	}
	saved := cache(normal, true)
	if err = images.SaveImage(saved.ID); err != nil {
		t.Fatal(err)
	}
	// Check using a new store and empty engine, as on a process restart.
	reopened, err := store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	e := &Engine{store: reopened, imageStore: NewImageStore(reopened, images.cacheDir, nil), distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker()}
	if err = e.restorePrefetchPool(); err != nil {
		t.Fatal(err)
	}
	if got := e.distPool.Size(); got != 2 {
		t.Fatalf("got %d restored entries, want 2", got)
	}
	ready, err := reopened.GetPooledFileIDSet()
	if err != nil || len(ready) != 2 || !ready[first.FileID] || !ready[second.FileID] {
		t.Fatalf("ready=%v err=%v", ready, err)
	}
	for _, id := range []string{older.FileID, consumed.FileID, disabledImage.FileID, stoppedImage.FileID, saved.FileID} {
		if _, _, err := e.imageStore.GetImage(id); err != nil {
			t.Fatalf("existing cached file %q removed: %v", id, err)
		}
	}
	for _, want := range []string{first.FileID, second.FileID} {
		actual, _, err := e.RandomImage("sample", "json", "square", "")
		if err != nil || actual == nil || actual.FileID != want {
			t.Fatalf("restored request: got %+v, want %s, err=%v", actual, want, err)
		}
	}
}
