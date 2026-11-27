package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

func TestCuratorImageIdentityPersistsForLiveAndPrefetchedImages(t *testing.T) {
	st, images, dbPath, imageServer := newTestImageStore(t)
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/missing" {
			fmt.Fprintf(w, `{"url":%q,"illust_id":"123456789"}`, imageServer.URL)
			return
		}
		fmt.Fprintf(w, `{"url":%q,"illust_id":"123456789","page_index":0}`, imageServer.URL)
	}))
	defer source.Close()

	if _, err := st.UpdateCuratorFeedbackConfig(false, source.URL, nil); err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProxyMode, settings.PoolSize = true, 1
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	e := &Engine{
		store: st, imageStore: images, httpClient: source.Client(), distPool: NewDistributionPool(),
		demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker(),
	}
	pixiv := addHealthSource(t, st, model.Source{Name: "Curator", URL: source.URL, RespType: "json", Enabled: true, Weight: 1})
	live, _, err := e.RandomImage("", "json", "", "")
	if err != nil {
		t.Fatal(err)
	}
	e.replenishPool()
	if e.distPool.Size() != 1 {
		t.Fatal("expected a prefetched image")
	}
	prefetched, _, err := e.RandomImage("", "json", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if live.ImageID == 0 || prefetched.ImageID == live.ImageID {
		t.Fatalf("expected distinct image IDs, live=%d, prefetched=%d", live.ImageID, prefetched.ImageID)
	}
	if err := st.DeleteImageByFileID(prefetched.FileID); err != nil {
		t.Fatal(err)
	}
	pixiv.URL = source.URL + "/missing"
	if err := st.UpdateSource(&pixiv); err != nil {
		t.Fatal(err)
	}
	missing, _, err := e.RandomImage("", "json", "", "")
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, result := range []*Result{live, prefetched} {
		identity, err := reopened.GetCuratorImageIdentity(result.ImageID)
		if err != nil || identity == nil || identity.Origin != source.URL || identity.IllustID != "123456789" ||
			identity.PageIndex == nil || *identity.PageIndex != 0 {
			t.Fatalf("image %d identity = %+v, %v", result.ImageID, identity, err)
		}
	}
	identity, err := reopened.GetCuratorImageIdentity(missing.ImageID)
	if err != nil || identity == nil || identity.Origin != source.URL || identity.PageIndex != nil {
		t.Fatalf("missing page should still mark the Curator source, got %+v, %v", identity, err)
	}
}
