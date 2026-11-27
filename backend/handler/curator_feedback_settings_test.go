package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/middleware"
	"github.com/pichub/backend/store"
)

func TestCuratorFeedbackSettingsKeepTokenWriteOnlyAndBoundToOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newExportTestStore(t)
	h := &Handler{store: st}
	router := gin.New()
	api := router.Group("/api", middleware.AdminAuth(st))
	api.GET("/settings/curator-feedback", h.GetCuratorFeedbackSettings)
	api.PUT("/settings/curator-feedback", h.UpdateCuratorFeedbackSettings)
	api.GET("/settings", h.GetSettings)
	api.GET("/export", h.ExportData)

	call := func(method, path, body, token string, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != wantStatus {
			t.Fatalf("%s %s: status=%d, want %d; body=%s", method, path, w.Code, wantStatus, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "upstream-secret") {
			t.Fatal("Curator token leaked in API response")
		}
		return w
	}

	const path = "/api/settings/curator-feedback"
	call(http.MethodGet, path, "", "", http.StatusForbidden)
	call(http.MethodPut, path, `{"enabled":true,"address":"https://example.test","token":"upstream-secret"}`, "", http.StatusForbidden)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.AdminToken = "hub-admin"
	if err = st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	call(http.MethodGet, path, "", "", http.StatusUnauthorized)
	call(http.MethodPut, path, `{"enabled":true,"address":"https://example.test/path","token":"upstream-secret"}`, "hub-admin", http.StatusBadRequest)
	w := call(http.MethodPut, path, `{"enabled":true,"address":"https://EXAMPLE.test:443/","token":"upstream-secret"}`, "hub-admin", http.StatusOK)
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response) != 3 || response["enabled"] != true || response["token_configured"] != true || response["address"] != "https://example.test" {
		t.Fatalf("unexpected redacted response: %v", response)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("sensitive settings response should not be cached")
	}
	call(http.MethodGet, path, "", "hub-admin", http.StatusOK)
	call(http.MethodGet, "/api/settings", "", "hub-admin", http.StatusOK)
	call(http.MethodGet, "/api/export?scope=config", "", "hub-admin", http.StatusOK)
	call(http.MethodPut, path, `{"enabled":true,"address":"https://other.test"}`, "hub-admin", http.StatusBadRequest)
	call(http.MethodPut, path, `{"enabled":false,"address":"https://example.test"}`, "hub-admin", http.StatusOK)
	cfg, err := st.GetCuratorFeedbackConfig()
	if err != nil || cfg.Enabled || cfg.Token != "upstream-secret" || cfg.Address != "https://example.test" {
		t.Fatal("omitting the token at the same origin should preserve it")
	}
	call(http.MethodPut, path, `{"enabled":false,"address":"https://example.test","clear_token":true}`, "hub-admin", http.StatusOK)
	cfg, err = st.GetCuratorFeedbackConfig()
	if err != nil || cfg.Enabled || cfg.Token != "" {
		t.Fatal("clearing the token must disable integration")
	}
	call(http.MethodPut, path, `{"enabled":true,"address":"https://example.test","token":"upstream-secret"}`, "hub-admin", http.StatusOK)
	call(http.MethodPut, path, `{"enabled":true,"address":"  "}`, "hub-admin", http.StatusOK)
	cfg, err = st.GetCuratorFeedbackConfig()
	if err != nil || cfg.Enabled || cfg.Address != "" || cfg.Token != "" {
		t.Fatal("clearing the address must clear the token and disable integration")
	}
}

func TestCuratorFeedbackOriginNormalizesHTTPS(t *testing.T) {
	for input, want := range map[string]string{
		"https://EXAMPLE.test:443/random?format=json": "https://example.test",
		"https://example.test:8443/random":            "https://example.test:8443",
		"https://[2001:db8::1]:443/random":            "https://[2001:db8::1]",
		"https://[2001:db8::1]:8443/random":           "https://[2001:db8::1]:8443",
		"http://example.test":                         "",
		"https://user@example.test":                   "",
		"https://example.test:99999":                  "",
		"https://example.test#fragment":               "",
		"https:///random":                             "",
	} {
		if got := store.CuratorFeedbackOrigin(input); got != want {
			t.Errorf("origin(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestCuratorConnectionUsesReadOnlyAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newExportTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.AdminToken = "hub-admin"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	curator := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/admin/api/settings/preferences" {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer curator-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"rules":{},"version":1}`)
	}))
	defer curator.Close()
	original := curatorFeedbackHTTP
	client := *curator.Client()
	client.CheckRedirect = original.CheckRedirect
	curatorFeedbackHTTP = &client
	t.Cleanup(func() { curatorFeedbackHTTP = original })

	h := &Handler{store: st}
	router := gin.New()
	router.POST("/test", middleware.AdminAuth(st), h.TestCuratorFeedbackSettings)
	call := func(address, token, auth string, want int) {
		t.Helper()
		body := fmt.Sprintf(`{"address":%q%s}`, address, token)
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+auth)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != want || strings.Contains(w.Body.String(), "curator-key") {
			t.Fatalf("connection test status=%d want=%d body=%s", w.Code, want, w.Body)
		}
	}
	call(curator.URL, `,"token":"curator-key"`, "bad-admin", 401)
	call(curator.URL, `,"token":"wrong"`, "hub-admin", 502)
	call(curator.URL, `,"token":"curator-key"`, "hub-admin", 200)
	cfg, err := st.GetCuratorFeedbackConfig()
	if err != nil || cfg.Address != "" || cfg.Token != "" {
		t.Fatal("testing persisted the draft")
	}
	token := "curator-key"
	if _, err := st.UpdateCuratorFeedbackConfig(false, curator.URL, &token); err != nil {
		t.Fatal(err)
	}
	call("https://other.example", "", "hub-admin", 400)
	call(curator.URL, "", "hub-admin", 200)
	if calls.Load() != 3 {
		t.Fatalf("unexpected upstream calls: %d", calls.Load())
	}
}
