package store

import (
	"github.com/pichub/backend/model"
	"path/filepath"
	"testing"
	"time"
)

func TestSourceTimestampsSurviveSQLiteRoundTrip(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.CreateSource(&model.Source{Name: "source", URL: "https://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := st.GetSource(id)
	if err != nil {
		t.Fatal(err)
	}
	if src.CreatedAt.IsZero() || src.UpdatedAt.IsZero() {
		t.Fatalf("timestamps lost: %+v", src)
	}
	src.Name = "edited"
	if err := st.UpdateSource(src); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec("UPDATE sources SET updated_at=? WHERE id=?", "2026-10-01 12:34:56.789", id); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 1, 12, 34, 56, 789000000, time.UTC)
	src, err = st.GetSource(id)
	if err != nil {
		t.Fatal(err)
	}
	list, err := st.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if !src.UpdatedAt.Equal(want) || !list[0].UpdatedAt.Equal(want) {
		t.Fatalf("want %v got %v / %v", want, src.UpdatedAt, list[0].UpdatedAt)
	}
}
