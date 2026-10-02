package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/handler"
	"github.com/pichub/backend/store"
)

func TestManagementRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.New(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := handler.NewHandler(st, nil)
	r := gin.New()
	registerManagementRoutes(r, h, st)
	for _, mode := range []string{"token", "login", "incomplete login with token"} {
		t.Run(mode, func(t *testing.T) {
			current, err := st.GetSettings()
			if err != nil {
				t.Fatal(err)
			}
			settings := *current
			settings.AdminToken = "test-admin-token"
			settings.LoginEnabled = mode != "token"
			settings.AdminUsername = ""
			settings.AdminPasswordHash = ""
			token := settings.AdminToken
			if mode == "login" {
				settings.AdminToken = ""
				settings.AdminUsername = "test-admin"
				settings.AdminPasswordHash = "test-hash"
				token = st.Sessions().Create(time.Hour)
			}
			if err := st.UpdateSettings(&settings); err != nil {
				t.Fatal(err)
			}
			for _, route := range r.Routes() {
				if route.Path == "/api/login" || route.Path == "/api/auth/check" {
					continue
				}
				path := strings.ReplaceAll(route.Path, ":id", "1")
				for _, credential := range []string{"", "invalid-token"} {
					req := httptest.NewRequest(route.Method, path, nil)
					if credential != "" {
						req.Header.Set("Authorization", "Bearer "+credential)
					}
					res := httptest.NewRecorder()
					r.ServeHTTP(res, req)
					if res.Code != http.StatusUnauthorized {
						t.Errorf("%s %s status=%d", route.Method, path, res.Code)
					}
				}
			}
			for _, path := range []string{"/api/sources", "/api/settings", "/api/export?scope=config", "/api/tags"} {
				req := httptest.NewRequest("GET", path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				res := httptest.NewRecorder()
				r.ServeHTTP(res, req)
				if res.Code != http.StatusOK {
					t.Errorf("authorized %s status=%d body=%s", path, res.Code, res.Body)
				}
			}
			res := httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest("GET", "/api/auth/check", nil))
			var state map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &state); err != nil {
				t.Fatal(err)
			}
			if state["auth_required"] != true || state["valid"] != false {
				t.Fatalf("auth state: %v", state)
			}
			if strings.Contains(res.Body.String(), "test-admin") || strings.Contains(res.Body.String(), "test-hash") {
				t.Fatal("auth check exposed credentials")
			}
		})
	}
	current, _ := st.GetSettings()
	settings := *current
	settings.LoginEnabled = false
	settings.AdminToken = ""
	if err := st.UpdateSettings(&settings); err != nil {
		t.Fatal(err)
	}
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/api/sources", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("disabled authentication status=%d", res.Code)
	}
}
