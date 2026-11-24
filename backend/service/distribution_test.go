package service

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
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

func TestEmptyDirectionPoolTriesOtherBranchAfterTimeout(t *testing.T) {
	st := healthTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode, settings.PoolSize = true, 120
	settings.Timeout = 8000
	settings.MinResolution = "0"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	slowStarted := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("branch") {
		case "slow":
			slowStarted <- struct{}{}
			<-r.Context().Done()
		case "working":
			w.Header().Set("Content-Type", "image/png")
			if err := png.Encode(w, image.NewRGBA(image.Rect(0, 0, 2, 4))); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	addHealthSource(t, st, model.Source{
		Name: "source", URL: upstream.URL, RespType: "direct", Enabled: true, Categories: []string{"horizontal"},
		Params: []model.QueryParam{
			{Key: "branch", Value: "working", Categories: []string{"vertical"}},
			{Key: "branch", Value: "slow", Categories: []string{"vertical"}, Weight: 1},
		},
	})
	e := &Engine{
		store: st, imageStore: NewImageStore(st, t.TempDir(), nil), httpClient: upstream.Client(),
		distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, status, err := e.RandomImageContext(ctx, "vertical", "json", "vertical", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(slowStarted) != 1 {
		t.Fatal("expected the higher-weight slow branch to be tried first")
	}
	if status != http.StatusOK || result.LocalURL == "" || result.Orientation != "vertical" {
		t.Fatalf("got status %d and result %+v", status, result)
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
	e.fetchPrefetch(context.Background(), first, time.Second)
	if _, ok := e.pickPrefetchSource([]model.Source{other}, nil, nil, 15, time.Now(), func(model.Source) bool { return true }); ok {
		t.Fatal("same-host source bypassed upstream Retry-After")
	}
	if calls != 1 {
		t.Fatalf("upstream calls=%d, want 1", calls)
	}
	if remaining := time.Until(e.prefetchCooldown[prefetchKeys(first)[1]]); remaining < 119*time.Second {
		t.Fatalf("Retry-After not respected: %v", remaining)
	}
}

func TestUnlimitedPrefetchSkipsNormalPacingButRespectsUpstreamRefusal(t *testing.T) {
	st := healthTestStore(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer upstream.Close()
	e := &Engine{store: st, httpClient: upstream.Client()}
	src := model.Source{ID: 1, Name: "self-hosted", URL: upstream.URL, Weight: 1}
	always := func(model.Source) bool { return true }
	now := time.Now()
	e.reservePrefetchSource(src, now)
	delete(e.prefetchInFlight, src.ID)
	e.ordinaryPrefetchInFlight = false
	if _, ok := e.pickPrefetchSource([]model.Source{src}, nil, nil, 15, now, always); ok {
		t.Fatal("limited source started a second request")
	}
	src.PrefetchUnlimited = true
	if _, ok := e.pickPrefetchSource([]model.Source{src}, nil, nil, 15, now, always); !ok {
		t.Fatal("unlimited source was blocked by normal pacing")
	}
	e.fetchPrefetch(context.Background(), src, time.Second)
	if _, ok := e.pickPrefetchSource([]model.Source{src}, nil, nil, 15, now.Add(time.Second), always); ok {
		t.Fatal("unlimited source ignored HTTP 429")
	}
}

func TestUnlimitedPrefetchRunsParallelWithoutConsumingOrdinaryPacing(t *testing.T) {
	st, images, _, picture := newTestImageStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode, settings.PoolSize, settings.Timeout = true, 12, 8000
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	started := make(chan string, 8)
	ordinaryRelease := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- r.URL.Path
		if r.URL.Path == "/ordinary" {
			select {
			case <-ordinaryRelease:
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"url":%q}`, picture.URL)
	}))
	defer upstream.Close()
	for _, name := range []string{"ordinary", "unlimited-a", "unlimited-b"} {
		addHealthSource(t, st, model.Source{
			Name: name, URL: upstream.URL + "/" + name, Enabled: true, Weight: 1,
			RespType: "json", PrefetchUnlimited: name != "ordinary",
		})
	}
	e := &Engine{
		store: st, imageStore: images, httpClient: upstream.Client(), distPool: NewDistributionPool(),
		demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker(),
	}
	now := time.Now()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		e.replenishPoolAt(now)
	}()
	defer func() {
		close(ordinaryRelease)
		<-finished
	}()
	checkRequests := func(want ...string) {
		t.Helper()
		set := make(map[string]bool, len(want))
		for _, path := range want {
			set["/"+path] = true
		}
		for range want {
			select {
			case got := <-started:
				if !set[got] {
					t.Fatalf("unexpected request %q", got)
				}
				delete(set, got)
			case <-time.After(2 * time.Second):
				t.Fatalf("still waiting for requests: %v", set)
			}
		}
	}
	checkRequests("ordinary", "unlimited-a", "unlimited-b")
	deadline := time.Now().Add(2 * time.Second)
	for {
		e.replenishMu.Lock()
		busy := len(e.prefetchInFlight)
		e.replenishMu.Unlock()
		if busy == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fast requests did not finish independently of the slow ordinary lane: %d active", busy)
		}
		time.Sleep(5 * time.Millisecond)
	}

	e.replenishPoolAt(now.Add(500 * time.Millisecond))
	if len(started) != 0 {
		t.Fatal("requests started before their one-second interval")
	}
	e.replenishPoolAt(now.Add(time.Second))
	checkRequests("unlimited-a", "unlimited-b")
	e.replenishMu.Lock()
	ordinaryNext := e.nextPrefetch
	e.replenishMu.Unlock()
	if !ordinaryNext.Equal(now.Add(prefetchInterval)) {
		t.Fatalf("unlimited requests changed ordinary deadline to %v", ordinaryNext)
	}
	if e.distPool.Size() != 4 {
		t.Fatalf("got %d cached images while ordinary request remains active, want 4", e.distPool.Size())
	}
	select {
	case <-finished:
		t.Fatal("ordinary request finished before release")
	default:
	}
}

func TestPrefetchDiscardsImageAfterSourceBranchChanges(t *testing.T) {
	st, images, _, picture := newTestImageStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode, settings.PoolSize = true, 4
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"url":%q}`, picture.URL)
	}))
	defer upstream.Close()
	src := addHealthSource(t, st, model.Source{
		Name: "prefetch", URL: upstream.URL, Enabled: true, RespType: "json", Weight: 1,
	})
	hc := NewHealthChecker(st)
	e := &Engine{
		store: st, imageStore: images, httpClient: upstream.Client(), healthChecker: hc,
		distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker(),
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		e.replenishPool()
	}()
	updated := func() error {
		defer close(release)
		select {
		case <-started:
		case <-time.After(time.Second):
			return errors.New("prefetch did not start")
		}
		src.DefaultQuery = "changed=1"
		return st.UpdateSource(&src)
	}()
	<-finished
	if updated != nil {
		t.Fatal(updated)
	}
	live, err := st.GetPooledFileIDSet()
	if e.distPool.Size() != 0 || len(hc.GetLastResult()) != 0 || err != nil || len(live) != 0 {
		t.Fatalf("stale prefetch updated distribution or health: stock=%d live=%+v health=%+v err=%v",
			e.distPool.Size(), live, hc.GetLastResult(), err)
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
