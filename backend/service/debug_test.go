package service

import (
	"database/sql"
	"encoding/json"
	"github.com/pichub/backend/model"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func debugFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[p] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestDebugRecordsHistoryWithoutChangingCacheOrPool(t *testing.T) {
	st, images, path, upstream := newTestImageStore(t)
	cached, err := images.DownloadAndStore(upstream.URL, upstream.URL, 42, "existing", nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	file, _, _ := images.GetImage(cached.FileID)
	png, _ := os.ReadFile(file)
	var selectedCalls, otherCalls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api":
			selectedCalls.Add(1)
			if r.URL.Query().Get("test") != "1" || r.Header.Get("X-Test") != "configured" {
				t.Error("request options missing")
			}
			if fail.Load() {
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"url": "/image"})
		case "/image":
			w.Header().Set("Content-Type", "image/png")
			w.Write(png)
		case "/other":
			otherCalls.Add(1)
			w.Header().Set("Content-Type", "image/png")
			w.Write(png)
		}
	}))
	defer server.Close()
	src := addHealthSource(t, st, model.Source{Name: "debug", URL: server.URL + "/api", RespType: "json", Enabled: false, Status: "error", DefaultQuery: "test=1", Headers: map[string]string{"X-Test": "configured"}})
	addHealthSource(t, st, model.Source{Name: "other", URL: server.URL + "/other", Enabled: true, Status: "normal", Weight: 50})
	settings, _ := st.GetSettings()
	settings.ProxyMode = true
	settings.PoolSize = 4
	st.UpdateSettings(settings)
	e := &Engine{store: st, imageStore: images, distPool: NewDistributionPool(), httpClient: server.Client(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker()}
	e.distPool.Push(&PoolEntry{FileID: cached.FileID, SourceID: 42, ImageID: cached.ID})
	before := debugFiles(t, images.cacheDir)
	if _, err := e.SetDebugSource(src.ID); err != nil {
		t.Fatal(err)
	}
	result, _, err := e.RandomImage("absent", "json", "vertical", "")
	if err != nil || result.SourceID != src.ID || !result.Debug || result.FileID != "" {
		t.Fatalf("wrong debug result %+v %v", result, err)
	}
	result, code, err := e.RandomImage("", "", "", "")
	if err != nil || code != 200 || string(result.ImageData) != string(png) {
		t.Fatalf("memory proxy failed %v", err)
	}
	e.replenishPool()
	fail.Store(true)
	if _, _, err = e.RandomImage("", "json", "", ""); err == nil {
		t.Fatal("failed source accepted")
	}
	if selectedCalls.Load() != 3 || otherCalls.Load() != 0 {
		t.Fatal("fallback or prefetch ran")
	}
	if e.distPool.Size() != 1 || !reflect.DeepEqual(before, debugFiles(t, images.cacheDir)) {
		t.Fatal("cache or pool changed")
	}
	e.ClearDebugSource()
	settings, _ = st.GetSettings()
	settings.ProxyMode = false
	st.UpdateSettings(settings)
	result, _, err = e.RandomImage("", "json", "", "")
	if err != nil || result.Debug || otherCalls.Load() != 1 {
		t.Fatalf("normal mode failed %v", err)
	}
	if (&Engine{store: st}).DebugSource().Enabled {
		t.Fatal("state persisted")
	}
	if _, err = e.SetDebugSource(99999); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT image_url, image_id, file_id FROM image_history WHERE source_id = ?", src.ID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var url, file string
		var imageID int64
		if err := rows.Scan(&url, &imageID, &file); err != nil {
			t.Fatal(err)
		}
		if url != server.URL+"/image" || imageID != 0 || file != "" {
			t.Fatalf("wrong debug history %s %d %s", url, imageID, file)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if count != 2 {
		t.Fatalf("debug history count=%d, want 2 successes and no failure", count)
	}
	var total int
	if err := db.QueryRow("SELECT COALESCE(SUM(count),0) FROM stats_requests").Scan(&total); err != nil || total != 3 {
		t.Fatalf("request stats=%d err=%v", total, err)
	}
}
func TestDebugSwitchWaitsForDistribution(t *testing.T) {
	st := healthTestStore(t)
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-release; w.WriteHeader(500) }))
	defer server.Close()
	src := addHealthSource(t, st, model.Source{URL: server.URL, Name: "source", Enabled: true, Status: "normal", Weight: 50})
	e := &Engine{store: st, httpClient: server.Client()}
	finished := make(chan struct{})
	go func() { e.RandomImage("", "json", "", ""); close(finished) }()
	<-entered
	switched := make(chan struct{})
	go func() { e.SetDebugSource(src.ID); close(switched) }()
	select {
	case <-switched:
		t.Fatal("switch did not wait")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	<-finished
	select {
	case <-switched:
	case <-time.After(time.Second):
		t.Fatal("switch stuck")
	}
}
