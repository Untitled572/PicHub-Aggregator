package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/handler"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

func TestAndroidFeedbackAuthorizationAndRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := store.New(filepath.Join(t.TempDir(), "feedback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	settings, err := db.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	adminSettings := *settings
	adminSettings.AdminToken = "admin-for-test"
	if err := db.UpdateSettings(&adminSettings); err != nil {
		t.Fatal(err)
	}

	sourceID, err := db.CreateSource(&model.Source{Name: "source", URL: "https://example.com", Weight: 50})
	if err != nil {
		t.Fatal(err)
	}
	imageID, err := db.InsertImage("image.jpg", "https://example.com/image.jpg", sourceID, "source", 720, 1080, "jpeg", 128, "[]", "vertical", false)
	if err != nil {
		t.Fatal(err)
	}

	routes := gin.New()
	h := handler.NewHandler(db, nil)
	registerAppRoutes(routes, h, db)
	registerManagementRoutes(routes, h, db)
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		routes.ServeHTTP(result, req)
		return result
	}
	requireStatus := func(result *httptest.ResponseRecorder, want int) {
		t.Helper()
		if result.Code != want {
			t.Fatalf("HTTP %d, want %d, body %s", result.Code, want, result.Body)
		}
	}
	generate := func() string {
		result := request(http.MethodPost, "/api/app-key", adminSettings.AdminToken, "")
		requireStatus(result, http.StatusCreated)
		var secret struct{ Key string }
		if err := json.Unmarshal(result.Body.Bytes(), &secret); err != nil || !strings.HasPrefix(secret.Key, "ph_app_") {
			t.Fatal("key is missing")
		}
		return secret.Key
	}
	weight := func(want int) {
		t.Helper()
		source, err := db.GetSource(sourceID)
		if err != nil || source.Weight != want {
			t.Fatalf("source %+v, want weight %d, error %v", source, want, err)
		}
	}
	payload := func(id int64, requestID, choice string) string {
		return fmt.Sprintf(`{"image_id":%d,"request_id":%q,"vote":%q}`, id, requestID, choice)
	}

	key := generate()
	state := request(http.MethodGet, "/api/app-key", adminSettings.AdminToken, "")
	requireStatus(state, http.StatusOK)
	if strings.Contains(state.Body.String(), key) {
		t.Fatal("key was returned after creation")
	}
	requireStatus(request(http.MethodGet, "/api/settings", key, ""), http.StatusUnauthorized)
	requireStatus(request(http.MethodGet, "/api/app/auth/check", adminSettings.AdminToken, ""), http.StatusUnauthorized)
	requireStatus(request(http.MethodGet, "/api/app/auth/check", key, ""), http.StatusNoContent)

	scopePath := "/api/app/tags"
	requireStatus(request(http.MethodGet, scopePath, "", ""), http.StatusUnauthorized)
	scopeResponse := request(http.MethodGet, scopePath, key, "")
	requireStatus(scopeResponse, http.StatusOK)
	var scope struct {
		Tags []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(scopeResponse.Body.Bytes(), &scope); err != nil {
		t.Fatal(err)
	}
	foundVertical := false
	for _, tag := range scope.Tags {
		if tag.ID == "vertical" && tag.Name == "竖屏" {
			foundVertical = true
		}
	}
	if !foundVertical {
		t.Fatalf("built-in vertical tag missing: %s", scopeResponse.Body)
	}

	receipt := "e8dc6582-04ca-4d60-833b-d2537ed23d9d"
	like := payload(imageID, receipt, "like")
	var calls sync.WaitGroup
	for range 8 {
		calls.Go(func() {
			if result := request(http.MethodPost, "/api/app/feedback", key, like); result.Code != http.StatusNoContent {
				t.Errorf("concurrent retry returned %d", result.Code)
			}
		})
	}
	calls.Wait()
	weight(51)
	requireStatus(request(http.MethodPost, "/api/app/feedback", key, payload(imageID, receipt, "dislike")), http.StatusConflict)

	dislike := payload(imageID, "1dcee6a0-2ab6-4cc5-bc41-31d1b6082f20", "dislike")
	requireStatus(request(http.MethodPost, "/api/app/feedback", key, dislike), http.StatusNoContent)
	weight(50)

	newKey := generate()
	requireStatus(request(http.MethodPost, "/api/app/feedback", key, like), http.StatusUnauthorized)
	requireStatus(request(http.MethodPost, "/api/app/feedback", newKey, payload(imageID+1, "da3227da-cd3a-4ce1-ad2d-c6e83234d900", "like")), http.StatusGone)
	weight(50)
	requireStatus(request(http.MethodDelete, "/api/app-key", adminSettings.AdminToken, ""), http.StatusNoContent)
	requireStatus(request(http.MethodGet, "/api/app/auth/check", newKey, ""), http.StatusUnauthorized)
}
