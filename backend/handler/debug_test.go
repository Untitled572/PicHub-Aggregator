package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/service"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDebugHTTPModeAndMemoryImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newExportTestStore(t)
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 2)))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data.Bytes())
	}))
	defer upstream.Close()
	src := &model.Source{Name: "disabled test source", URL: upstream.URL, Categories: []string{"test"}, Enabled: false, Status: "error"}
	id, err := st.CreateSource(src)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := st.GetSettings()
	settings.ProxyMode = true
	st.UpdateSettings(settings)
	e := service.NewEngine(st, nil, nil, nil, nil)
	h := NewHandler(st, e)
	r := gin.New()
	r.GET("/api/debug/source", h.GetDebugSource)
	r.PUT("/api/debug/source", h.SetDebugSource)
	r.DELETE("/api/debug/source", h.ClearDebugSource)
	r.GET("/random", h.RandomImage)
	r.GET("/e/:name", h.EndpointImage)
	endpoint := &model.Endpoint{Name: "test", Enabled: true, BoundTags: []string{"test"}}
	if _, err := st.CreateEndpoint(endpoint); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]int64{"source_id": id})
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("PUT", "/api/debug/source", bytes.NewReader(body)))
	if res.Code != 200 {
		t.Fatalf("enable: %d %s", res.Code, res.Body)
	}
	for _, path := range []string{"/random?category=test&orientation=horizontal", "/e/test?orientation=horizontal"} {
		res = httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
		if res.Code != 200 || res.Header().Get("Cache-Control") != "no-store" || !bytes.Equal(res.Body.Bytes(), data.Bytes()) {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body)
		}
	}
	path := "/random?category=test&orientation=horizontal&format=json"
	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
	if res.Code != 200 || !json.Valid(res.Body.Bytes()) {
		t.Fatalf("ordinary debug JSON response changed: %d", res.Code)
	}

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-PicHub-Debug-Image", "true")
	res = httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != 200 || res.Header().Get("X-PicHub-Debug") != "true" || !bytes.Equal(res.Body.Bytes(), data.Bytes()) {
		t.Fatalf("app debug response=%d", res.Code)
	}
	metadata, err := base64.RawURLEncoding.DecodeString(res.Header().Get("X-PicHub-Debug-Metadata"))
	var details struct {
		Source     string
		Categories []string
	}
	if err != nil || json.Unmarshal(metadata, &details) != nil || details.Source != src.Name || len(details.Categories) != 1 || details.Categories[0] != "test" {
		t.Fatalf("invalid app debug metadata: %q", metadata)
	}

	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/random?category=missing&format=json", nil))
	var failure struct{ Code string }
	if res.Code != 503 || json.Unmarshal(res.Body.Bytes(), &failure) != nil || failure.Code != "debug_distribution_failed" {
		t.Fatalf("debug failure was not identified: %d %s", res.Code, res.Body)
	}

	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("DELETE", "/api/debug/source", nil))
	if res.Code != 200 || e.DebugSource().Enabled {
		t.Fatal("debug mode did not exit")
	}
}
