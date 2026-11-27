package handler

import (
	"encoding/json"
	"fmt"
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

func TestWebAndAppShareImageFeedback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type curatorVote struct {
		IllustID  string `json:"illust_id"`
		PageIndex int    `json:"page_index"`
		Value     string `json:"value"`
	}
	calls := make(chan curatorVote, 8)
	curator := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/admin/api/image-feedback" ||
			r.Header.Get("Authorization") != "Bearer curator-test" {
			t.Errorf("unexpected Curator request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var vote curatorVote
		if err := json.NewDecoder(r.Body).Decode(&vote); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		calls <- vote
		if vote.PageIndex == 4 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"value": vote.Value})
	}))
	defer curator.Close()
	originalClient := curatorFeedbackHTTP
	testClient := *curator.Client()
	testClient.CheckRedirect = originalClient.CheckRedirect
	curatorFeedbackHTTP = &testClient
	t.Cleanup(func() { curatorFeedbackHTTP = originalClient })

	st, err := store.New(filepath.Join(t.TempDir(), "feedback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.AdminToken = "hub-admin-test"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	appKey := "ph_app_" + strings.Repeat("x", 43)
	if err := st.SetAppKey(appKey); err != nil {
		t.Fatal(err)
	}
	token := "curator-test"
	if _, err := st.UpdateCuratorFeedbackConfig(true, curator.URL, &token); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(st, nil)
	router := gin.New()
	router.POST("/app", h.AppFeedback)
	router.PUT("/history/:id", middleware.AdminAuth(st), h.HistoryFeedback)

	linked := model.Source{URL: curator.URL + "/random", Name: "Curator", Weight: 50}
	regular := model.Source{URL: "https://ordinary.example/random", Name: "ordinary", Weight: 50}
	for _, src := range []*model.Source{&linked, &regular} {
		src.ID, err = st.CreateSource(src)
		if err != nil {
			t.Fatal(err)
		}
	}
	add := func(src model.Source, file, ref string, page *int) (int64, int64) {
		t.Helper()
		var identity *store.CuratorImageIdentity
		if page != nil {
			identity = &store.CuratorImageIdentity{Origin: curator.URL, IllustID: "9007199254740993", PageIndex: page}
		}
		id, err := st.InsertImageWithCuratorIdentity(file, src.URL, src.ID, src.Name, 1, 2, "png", 2, "[]", "vertical", false, identity)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.RecordDistribution(nil, src, src.URL, &id, file, ref); err != nil {
			t.Fatal(err)
		}
		historyID, err := st.LatestImageHistoryID(id)
		if err != nil || historyID == 0 {
			t.Fatalf("history=%d err=%v", historyID, err)
		}
		return id, historyID
	}
	page, otherPage, missingPage := 0, 1, 4
	ref := strings.Repeat("A", 26)
	one, _ := add(linked, "first", ref, &page)
	_, duplicateHistory := add(linked, "duplicate", strings.Repeat("B", 26), &page)
	_, otherHistory := add(linked, "other", strings.Repeat("C", 26), &otherPage)
	_, unknownHistory := add(linked, "legacy", strings.Repeat("D", 26), nil)
	_, missingHistory := add(linked, "missing", strings.Repeat("E", 26), &missingPage)
	normal, normalHistory := add(regular, "ordinary", strings.Repeat("F", 26), nil)
	request := func(method, path, credential, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+credential)
		req.Header.Set("Content-Type", "application/json")
		if want == http.StatusOK {
			req.Header.Set("Accept", "application/json")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: HTTP %d, want %d: %s", method, path, rec.Code, want, rec.Body)
		}
		return rec
	}
	web := func(id int64, operation int, vote string, expected int) *httptest.ResponseRecorder {
		t.Helper()
		return request(http.MethodPut, fmt.Sprintf("/history/%d", id), settings.AdminToken,
			fmt.Sprintf(`{"request_id":"00000000-0000-0000-0000-%012d","vote":%q}`, operation, vote), expected)
	}
	expectVote := func(w *httptest.ResponseRecorder, value string) {
		t.Helper()
		if strings.TrimSpace(w.Body.String()) != fmt.Sprintf(`{"vote":%q}`, value) {
			t.Fatalf("wanted %s, got %s", value, w.Body)
		}
	}
	expectCurator := func(index int, value string) {
		t.Helper()
		select {
		case got := <-calls:
			if got.IllustID != "9007199254740993" || got.PageIndex != index || got.Value != value {
				t.Fatalf("unexpected curator vote %+v", got)
			}
		default:
			t.Fatal("request did not reach Curator")
		}
	}
	app := func() *httptest.ResponseRecorder {
		return request(http.MethodPost, "/app", appKey,
			fmt.Sprintf(`{"request_id":"00000000-0000-0000-0000-000000000001","image_id":%d,"distribution_ref":%q,"vote":"like"}`, one, ref), 200)
	}
	expectVote(app(), "like")
	expectCurator(0, "like")
	expectVote(web(duplicateHistory, 2, "dislike", 200), "dislike")
	expectCurator(0, "dislike")
	expectVote(app(), "dislike") // A retry of the first operation must not overwrite the later vote.
	history, _, err := st.GetImageHistory(20, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range history {
		if row.ID == duplicateHistory || row.ImageID == one {
			if row.Feedback != "dislike" || !row.FeedbackSupported {
				t.Fatalf("divergent display state: %+v", row)
			}
		}
		if row.ID == otherHistory && row.Feedback != "none" {
			t.Fatal("another page inherited the vote")
		}
	}
	expectVote(web(duplicateHistory, 3, "none", 200), "none")
	expectCurator(0, "neutral")
	web(missingHistory, 4, "like", 410)
	expectCurator(4, "like")
	web(unknownHistory, 5, "like", 409)
	legacy := request(http.MethodPost, "/app", appKey,
		fmt.Sprintf(`{"request_id":"00000000-0000-0000-0000-000000000008","image_id":%d,"distribution_ref":%q,"vote":"like"}`, one, ref), http.StatusNoContent)
	if legacy.Body.Len() != 0 {
		t.Fatal("legacy feedback should have no response body")
	}
	expectCurator(0, "like")
	if len(calls) != 0 {
		t.Fatal("unmapped or replayed vote was forwarded")
	}
	curatorSource, err := st.GetSource(linked.ID)
	if err != nil || curatorSource.Weight != 50 {
		t.Fatalf("Curator source weight changed: %+v err=%v", curatorSource, err)
	}

	expectVote(web(normalHistory, 6, "like", 200), "like")
	expectVote(web(normalHistory, 6, "like", 200), "like")
	expectVote(request(http.MethodPost, "/app", appKey,
		fmt.Sprintf(`{"request_id":"00000000-0000-0000-0000-000000000007","image_id":%d,"distribution_ref":%q,"vote":"none"}`, normal, strings.Repeat("F", 26)), 200), "none")
	normalSource, err := st.GetSource(regular.ID)
	if err != nil || normalSource.Weight != 50 {
		t.Fatalf("ordinary source did not undo web vote: %+v err=%v", normalSource, err)
	}
}
