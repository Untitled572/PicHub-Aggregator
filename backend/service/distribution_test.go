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
