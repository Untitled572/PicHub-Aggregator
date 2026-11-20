package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

func TestHistoryWaitReturnsAfterNewDistribution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newExportTestStore(t)
	h := NewHandler(st, nil)
	router := gin.New()
	router.GET("/api/stats/history", h.GetImageHistory)
	read := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	const path = "/api/stats/history"
	var initial struct{ Revision string }
	first := read(path)
	if err := json.Unmarshal(first.Body.Bytes(), &initial); err != nil || initial.Revision == "" {
		t.Fatalf("initial history: %s, %v", first.Body, err)
	}
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- read(path + "?revision=" + initial.Revision) }()
	select {
	case response := <-result:
		t.Fatalf("idle wait returned early: %d %s", response.Code, response.Body)
	case <-time.After(50 * time.Millisecond):
	}
	if err := st.RecordStats([]string{"test"}, model.Source{ID: 1, Name: "test"}, "https://example.test/image", nil, ""); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		var next struct {
			Revision string
			Total    int
			History  []model.ImageHistoryRecord
		}
		if err := json.Unmarshal(response.Body.Bytes(), &next); err != nil || response.Code != 200 || next.Revision == initial.Revision || next.Total != 1 || len(next.History) != 1 {
			t.Fatalf("new history: %d %s, %v", response.Code, response.Body, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("new distribution did not wake the history request")
	}
}
