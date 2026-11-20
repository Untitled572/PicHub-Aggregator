package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/service"
)

func TestSourceHealthCheckOnlyProbesSelectedSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newExportTestStore(t)
	var selectedCalls, otherCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/other" {
			otherCalls.Add(1)
		} else {
			selectedCalls.Add(1)
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	src := &model.Source{Name: "selected", URL: upstream.URL + "/selected", Enabled: true, Params: []model.QueryParam{{Key: "type", Value: "phone"}}}
	id, err := st.CreateSource(src)
	if err != nil {
		t.Fatal(err)
	}
	other := &model.Source{Name: "other", URL: upstream.URL + "/other", Enabled: true}
	otherID, err := st.CreateSource(other)
	if err != nil {
		t.Fatal(err)
	}
	hc := service.NewHealthChecker(st)
	defer hc.Stop()
	saved := service.HealthResult{ID: otherID, Name: "other", Origin: "cache", Available: true, CheckedAt: time.Now()}
	hc.RecordCacheResult(saved)
	h := NewHandlerWithEngine(st, nil, hc)
	router := gin.New()
	router.POST("/api/sources/:id/health-check", h.SourceHealthCheck)
	for _, tc := range []struct {
		path   string
		status int
	}{{"bad", 400}, {"0", 400}, {"-1", 400}, {"9999", 404}, {fmt.Sprint(id), 200}} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/sources/"+tc.path+"/health-check", nil))
		if res.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.path, res.Code, res.Body)
		}
		if tc.status == 200 {
			var result service.HealthResult
			if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.ID != id || result.CheckedEndpoints != 2 || !result.Available {
				t.Fatalf("result=%+v", result)
			}
		}
	}
	if selectedCalls.Load() != 2 || otherCalls.Load() != 0 {
		t.Fatalf("selected=%d other=%d", selectedCalls.Load(), otherCalls.Load())
	}
	results := hc.GetLastResult()
	if len(results) != 2 || !reflect.DeepEqual(results[0], saved) || results[1].ID != id {
		t.Fatalf("results were not merged: %+v", results)
	}
	if hc.LastRunAt().Before(results[1].CheckedAt) {
		t.Fatal("last feedback time not updated")
	}
}
