package service

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/pichub/backend/model"
	"image"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	result, _, err := e.RandomImage("", "json", "", "")
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

func TestDebugHonorsCategoryBranchesAndOrientation(t *testing.T) {
	st := healthTestStore(t)
	var horizontal, vertical bytes.Buffer
	png.Encode(&horizontal, image.NewRGBA(image.Rect(0, 0, 4, 2)))
	png.Encode(&vertical, image.NewRGBA(image.Rect(0, 0, 2, 4)))
	var calls, otherCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api":
			calls.Add(1)
			if r.URL.Query().Get("default") != "yes" {
				t.Error("default parameter lost")
			}
			w.Header().Set("Content-Type", "application/json")
			target := "/horizontal"
			if r.URL.Query().Get("type") == "phone" {
				target = "/vertical"
			}
			json.NewEncoder(w).Encode(map[string]string{"url": target})
		case "/horizontal":
			w.Header().Set("Content-Type", "image/png")
			w.Write(horizontal.Bytes())
		case "/vertical":
			w.Header().Set("Content-Type", "image/png")
			w.Write(vertical.Bytes())
		case "/other":
			otherCalls.Add(1)
			w.Header().Set("Content-Type", "image/png")
			w.Write(vertical.Bytes())
		}
	}))
	defer upstream.Close()
	src := addHealthSource(t, st, model.Source{Name: "debug", URL: upstream.URL + "/api", Enabled: false, Status: "error", RespType: "json", Categories: []string{"pc"}, DefaultQuery: "default=yes", Params: []model.QueryParam{{Key: "type", Value: "phone", Categories: []string{"phone"}, Weight: 10}}})
	addHealthSource(t, st, model.Source{Name: "other", URL: upstream.URL + "/other", Enabled: true, Status: "normal", Categories: []string{"absent"}})
	e := &Engine{store: st, httpClient: upstream.Client()}
	e.SetDebugSource(src.ID)
	for _, tc := range []struct {
		category, orientation, suffix string
		success                       bool
	}{
		{"phone", "vertical", "/vertical", true}, {"pc", "horizontal", "/horizontal", true},
		{"phone", "horizontal", "", false}, {"pc", "vertical", "", false}, {"absent", "vertical", "", false},
	} {
		result, code, err := e.RandomImage(tc.category, "json", tc.orientation, "")
		if !tc.success {
			if err == nil {
				t.Fatalf("unexpected success for %s %s", tc.category, tc.orientation)
			}
			continue
		}
		if err != nil || code != 200 || result.URL != upstream.URL+tc.suffix || result.Orientation != tc.orientation || result.SourceID != src.ID || len(result.ImageData) != 0 {
			t.Fatalf("category=%s orientation=%s result=%+v err=%v", tc.category, tc.orientation, result, err)
		}
	}
	if calls.Load() != 4 || otherCalls.Load() != 0 {
		t.Fatalf("wrong selection calls=%d other=%d", calls.Load(), otherCalls.Load())
	}
}

func TestDebugStatusDoesNotWaitForSwitch(t *testing.T) {
	st := healthTestStore(t)
	src := addHealthSource(t, st, model.Source{Name: "selected", URL: "https://example.test/image"})
	e := &Engine{store: st}
	e.modeMu.RLock()
	done := make(chan struct{})
	go func() { e.SetDebugSource(src.ID); close(done) }()
	deadline := time.Now().Add(time.Second)
	for e.debugSwitches.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stateCh := make(chan DebugSourceState, 1)
	go func() { stateCh <- e.DebugSource() }()
	select {
	case state := <-stateCh:
		if !state.Switching || state.Enabled {
			t.Error("pending switch state incorrect")
		}
	case <-time.After(time.Second):
		t.Error("status waited on the distribution lock")
	}
	e.modeMu.RUnlock()
	<-done
	state := e.DebugSource()
	if !state.Enabled || state.Switching || state.SourceName != "selected" {
		t.Fatalf("committed state %+v", state)
	}
}
func TestDebugReportsNoMatchAndRedactsCredentials(t *testing.T) {
	st := healthTestStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":{"code":"NO_MATCH","message":"Not Found"}}`))
	}))
	defer server.Close()
	src := addHealthSource(t, st, model.Source{Name: "r18 source", URL: server.URL + "/random?token=secret&format=json&r18=1"})
	e := &Engine{store: st, httpClient: server.Client()}
	e.SetDebugSource(src.ID)
	_, _, err := e.RandomImage("", "json", "", "")
	if err == nil || !strings.Contains(err.Error(), "NO_MATCH") || !strings.Contains(err.Error(), "r18=1") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("wrong debug error: %v", err)
	}
	if strings.Contains(debugRequestAddress("https://name:password@example.test/random?token=secret&r18=1"), "password") {
		t.Fatal("credentials exposed")
	}
}
