package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/middleware"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

func newExportTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create temporary store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close temporary store: %v", err)
		}
	})
	return st
}

func TestExportDataDoesNotClearCachedAdminToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newExportTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.AdminToken = "local-secret"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	h := NewHandler(st, nil)
	router.GET("/api/export", h.ExportData)
	router.GET("/protected", middleware.AdminAuth(st), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	export := httptest.NewRecorder()
	router.ServeHTTP(export, httptest.NewRequest(http.MethodGet, "/api/export?scope=config", nil))
	if export.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d", export.Code, http.StatusOK)
	}
	if strings.Contains(export.Body.String(), "local-secret") {
		t.Fatal("public export leaked the admin token")
	}

	protected := httptest.NewRecorder()
	router.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("protected status after export = %d, want %d", protected.Code, http.StatusUnauthorized)
	}
	settings, err = st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.AdminToken != "local-secret" {
		t.Fatalf("cached admin token = %q, want %q", settings.AdminToken, "local-secret")
	}
}

func TestImportDataPreservesLocalAuthenticationSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create temporary store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close temporary store: %v", err)
		}
	})
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.AdminToken = "local-token"
	settings.AdminPasswordHash = "local-password-hash"
	settings.AdminUsername = "local-admin"
	settings.LoginEnabled = true
	settings.SessionHours = 7
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	payloadBytes, err := json.Marshal(model.ExportManifest{Settings: &model.Settings{
		ProxyURL:       "http://imported.example",
		AdminUsername:  "imported-admin",
		LoginEnabled:   false,
		SessionHours:   1,
		AdminPassword:  "imported-password",
		SavedImagesDir: filepath.Join(t.TempDir(), "saved"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	h := NewHandler(st, nil)
	router.POST("/api/import", h.ImportData)
	router.GET("/protected", middleware.AdminAuth(st), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	importResponse := httptest.NewRecorder()
	router.ServeHTTP(importResponse, httptest.NewRequest(http.MethodPost, "/api/import", bytes.NewReader(payloadBytes)))
	if importResponse.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", importResponse.Code, importResponse.Body.String())
	}

	settings, err = st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	assertLocalAuthSettings(t, settings)
	if settings.ProxyURL != "http://imported.example" {
		t.Fatalf("imported application setting was not restored: proxy_url = %q", settings.ProxyURL)
	}

	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	settings, err = st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	assertLocalAuthSettings(t, settings)
	if settings.ProxyURL != "http://imported.example" {
		t.Fatalf("persisted imported proxy_url = %q", settings.ProxyURL)
	}

	router = gin.New()
	router.GET("/protected", middleware.AdminAuth(st), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, test := range []struct {
		name   string
		token  string
		status int
	}{{"local token", "local-token", http.StatusNoContent}, {"imported token", "imported-token", http.StatusUnauthorized}} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != test.status {
				t.Fatalf("status with %s = %d, want %d", test.name, response.Code, test.status)
			}
		})
	}
}

func assertLocalAuthSettings(t *testing.T, settings *model.Settings) {
	t.Helper()
	if settings.AdminToken != "local-token" || settings.AdminPasswordHash != "local-password-hash" || settings.AdminUsername != "local-admin" || !settings.LoginEnabled || settings.SessionHours != 7 {
		t.Fatalf("local authentication settings changed: token=%q username=%q login_enabled=%v session_hours=%d", settings.AdminToken, settings.AdminUsername, settings.LoginEnabled, settings.SessionHours)
	}
}
