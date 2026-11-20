package store

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/pichub/backend/model"
)

func TestDistributionFeedbackIsReversibleAndScopedToHistory(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "feedback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	src := model.Source{Name: "source", URL: "https://example.test/image", Weight: 69}
	src.ID, err = st.CreateSource(&src)
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.InsertImage("cached.jpg", src.URL, src.ID, src.Name, 600, 1000, "jpeg", 120, "[]", "vertical", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"first", "second"} {
		if err := st.RecordDistribution(nil, src, src.URL, &id, "cached.jpg", ref); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetAppKey("secret"); err != nil {
		t.Fatal(err)
	}

	assertState := func(weight, first, second int) {
		t.Helper()
		source, err := st.GetSource(src.ID)
		if err != nil || source.Weight != weight {
			t.Fatalf("source=%+v err=%v; wanted weight %d", source, err, weight)
		}
		history, _, err := st.GetImageHistory(10, 0)
		if err != nil || len(history) != 2 || history[1].AppFeedback != first || history[0].AppFeedback != second {
			t.Fatalf("history=%+v err=%v; wanted first=%d second=%d", history, err, first, second)
		}
	}
	react := func(requestID, ref string, vote int) {
		t.Helper()
		if err := st.RecordAppFeedback("secret", requestID, id, vote, ref); err != nil {
			t.Fatal(err)
		}
	}

	react("like", "first", 1)
	assertState(70, 1, 0)
	react("at-limit", "second", 1)
	assertState(70, 1, 1)
	react("undo-limit", "second", 0)
	assertState(70, 1, 0)
	react("switch", "first", -1)
	assertState(68, -1, 0)
	react("undo", "first", 0)
	react("undo", "first", 0)
	react("like", "first", 1) // Retrying the old operation cannot reapply a vote.
	assertState(69, 0, 0)
	if err := st.RecordAppFeedback("secret", "like", id, 1, "second"); !errors.Is(err, ErrFeedbackConflict) {
		t.Fatalf("wanted a conflict for a reused operation, got %v", err)
	}
	if err := st.RecordAppFeedback("secret", "unknown", id, 1, "missing"); !errors.Is(err, ErrFeedbackImageGone) {
		t.Fatalf("wanted an error for an unknown distribution, got %v", err)
	}
	assertState(69, 0, 0)
}
