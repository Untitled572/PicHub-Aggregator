package service

import (
	"bytes"
	"context"
	"github.com/pichub/backend/model"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAdaptiveSelectionAndIsolation(t *testing.T) {
	tags := []model.Tag{{ID: "r18", Exclusive: true}}
	sources := []model.Source{
		{ID: 1, URL: "https://example.test/random", Enabled: true, Categories: []string{"adaptive"}},
		{ID: 2, URL: "https://example.test/random", Enabled: true, Params: []model.QueryParam{
			{Key: "orientation", Value: "landscape", Categories: []string{"horizontal"}},
			{Key: "orientation", Value: "portrait", Categories: []string{"vertical"}},
			{Key: "r18", Value: "1", Categories: []string{"r18"}},
		}},
		{ID: 3, URL: "https://example.test/plain", Enabled: true},
	}
	for _, orientation := range []string{"horizontal", "vertical"} {
		got := filterRequestSources(sources, []string{"adaptive"}, tags, orientation)
		if len(got) != 3 {
			t.Fatalf("%s candidates=%+v", orientation, got)
		}
		for _, src := range got {
			if src.ID == 2 && !hasCategory(src.Categories, orientation) {
				t.Fatalf("wrong branch: %+v", src)
			}
			if hasCategory(src.Categories, "r18") {
				t.Fatal("exclusive branch leaked")
			}
		}
	}
	if !matchRequestCategories(nil, []string{"adaptive"}, tags, true, false) {
		t.Fatal("untagged excluded")
	}
	if matchRequestCategories([]string{"r18"}, []string{"adaptive"}, tags, true, false) {
		t.Fatal("exclusive cache leaked")
	}
	if !matchRequestCategories([]string{"r18"}, []string{"adaptive", "r18"}, tags, false, true) {
		t.Fatal("explicit exclusive excluded")
	}
	if matchRequestCategories([]string{"horizontal"}, []string{"adaptive", "scenery"}, tags, false, false) {
		t.Fatal("other category ignored")
	}
	// Prefetch retains literal tag selection and does not multiply health samples.
	if matchCategories(nil, []string{"adaptive"}, tags, true, false) {
		t.Fatal("prefetch semantics changed")
	}
}

func TestAdaptiveDebugAndNormal(t *testing.T) {
	for _, mode := range []string{"normal", "debug"} {
		for _, kind := range []string{"native", "branches", "plain"} {
			for _, mobile := range []bool{false, true} {
				t.Run(mode+"/"+kind+map[bool]string{true: "/phone", false: "/desktop"}[mobile], func(t *testing.T) {
					st := healthTestStore(t)
					ua := "Mozilla/5.0 (Windows NT 10.0)"
					want := "horizontal"
					if mobile {
						ua = "Mozilla/5.0 (iPhone; Mobile)"
						want = "vertical"
					}
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if kind == "native" && r.Header.Get("User-Agent") != ua {
							t.Errorf("UA lost: %s", r.Header.Get("User-Agent"))
						}
						orientation := want
						if kind == "branches" {
							switch r.URL.Query().Get("orientation") {
							case "landscape":
								orientation = "horizontal"
							case "portrait":
								orientation = "vertical"
							default:
								t.Error("base selected instead of direction branch")
							}
						}
						width, height := 4, 2
						if orientation == "vertical" {
							width, height = 2, 4
						}
						w.Header().Set("Content-Type", "image/png")
						png.Encode(w, image.NewRGBA(image.Rect(0, 0, width, height)))
					}))
					defer srv.Close()
					src := model.Source{Name: "adaptive-test", URL: srv.URL, Enabled: true, Weight: 1, RespType: "image", Headers: map[string]string{"User-Agent": "configured"}}
					if kind == "native" {
						src.Categories = []string{"adaptive"}
					}
					if kind == "branches" {
						src.Params = []model.QueryParam{{Key: "orientation", Value: "landscape", Categories: []string{"horizontal"}}, {Key: "orientation", Value: "portrait", Categories: []string{"vertical"}}}
					}
					src = addHealthSource(t, st, src)
					cfg, _ := st.GetSettings()
					cfg.ProxyMode = false
					cfg.Timeout = 3000
					st.UpdateSettings(cfg)
					e := &Engine{store: st, httpClient: srv.Client(), distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker()}
					if mode == "debug" {
						e.SetDebugSource(src.ID)
					}
					result, code, err := e.RandomImage("adaptive", "json", "", ua)
					if err != nil || code != 200 || result == nil {
						t.Fatalf("result=%+v code=%d err=%v", result, code, err)
					}
				})
			}
		}
	}
}

func TestAdaptiveNativeAndSizeValidation(t *testing.T) {
	native := model.Source{Categories: []string{"adaptive"}, URL: "http://example.test"}
	if requestOrientation(native, []string{"adaptive"}, "vertical", false) != "" {
		t.Fatal("native should handle UA")
	}
	if requestOrientation(native, []string{"adaptive"}, "vertical", true) != "vertical" {
		t.Fatal("explicit direction ignored")
	}
	req, _ := newSourceRequest(context.Background(), native, "mobile")
	if req.Header.Get("User-Agent") != "mobile" {
		t.Fatal("UA missing")
	}
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 2)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(buf.Bytes()) }))
	defer srv.Close()
	if err := checkRemoteOrientation(native, srv.URL, "vertical", "", time.Second, srv.Client()); err == nil {
		t.Fatal("wrong dimensions accepted")
	}
}

// Explicit opt-in: exercises a real upstream through the normal and debug engine paths.
func TestAdaptivePixivLive(t *testing.T) {
	target := os.Getenv("PICHUB_LIVE_PIXIV")
	if target == "" {
		t.Skip("set PICHUB_LIVE_PIXIV to test real upstream")
	}
	for _, debug := range []bool{false, true} {
		for _, native := range []bool{false, true} {
			for _, ua := range []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile"} {
				st := healthTestStore(t)
				src := model.Source{Name: "pix live", URL: target, Enabled: true, Weight: 1, RespType: "json", JsonPath: "url"}
				if native {
					src.Categories = []string{"adaptive"}
				} else {
					src.Params = []model.QueryParam{{Key: "orientation", Value: "landscape", Categories: []string{"horizontal"}}, {Key: "orientation", Value: "portrait", Categories: []string{"vertical"}}}
				}
				src = addHealthSource(t, st, src)
				cfg, _ := st.GetSettings()
				cfg.ProxyMode = true
				cfg.PoolSize = 0
				cfg.Timeout = 15000
				cfg.MinResolution = "0"
				st.UpdateSettings(cfg)
				images := NewImageStore(st, t.TempDir(), nil)
				e := &Engine{store: st, httpClient: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{}}, imageStore: images, distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker()}
				if debug {
					e.SetDebugSource(src.ID)
				}
				result, code, err := e.RandomImage("adaptive", "json", "", ua)
				if err != nil || code != 200 || result == nil {
					t.Fatalf("debug=%t native=%t ua=%s result=%+v err=%v", debug, native, ua, result, err)
				}
				if !native && result.Orientation != detectOrientationFromUA(ua) {
					t.Fatalf("wrong size: %+v", result)
				}
				if !strings.HasPrefix(result.URL, "http") {
					t.Fatalf("invalid URL: %+v", result)
				}
				t.Logf("debug=%t native=%t UA=%s size=%dx%d", debug, native, detectOrientationFromUA(ua), result.Width, result.Height)
			}
		}
	}
}

func TestAdaptivePoolUsesRealImageDirection(t *testing.T) {
	st := healthTestStore(t)
	cfg, _ := st.GetSettings()
	cfg.MinResolution = "0"
	cfg.ProxyMode = true
	cfg.PoolSize = 2
	st.UpdateSettings(cfg)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		width, height := 4, 2
		if r.URL.Path == "/vertical" {
			width, height = 2, 4
		}
		png.Encode(w, image.NewRGBA(image.Rect(0, 0, width, height)))
	}))
	defer srv.Close()
	images := NewImageStore(st, t.TempDir(), nil)
	e := &Engine{store: st, httpClient: srv.Client(), imageStore: images, distPool: NewDistributionPool(), demandTracker: NewDemandTracker(), sourceDemand: NewSourceDemandTracker()}
	for _, direction := range []string{"vertical", "horizontal"} {
		img, err := images.DownloadAndStore(srv.URL+"/"+direction, srv.URL, 1, "native", []string{"adaptive"}, nil, true)
		if err != nil {
			t.Fatal(err)
		}
		e.distPool.Push(&PoolEntry{FileID: img.FileID, ImageID: img.ID, SourceID: 1, Categories: []string{"adaptive"}, Orientation: direction, Width: img.Width, Height: img.Height})
	}
	for _, ua := range []string{"Windows", "iPhone Mobile"} {
		result, _, err := e.RandomImage("adaptive", "json", "", ua)
		if err != nil || result.Orientation != detectOrientationFromUA(ua) {
			t.Fatalf("wrong cache result=%+v err=%v", result, err)
		}
	}
}
