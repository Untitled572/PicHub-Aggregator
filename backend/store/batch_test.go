package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/pichub/backend/model"
)

func newTestStatsBatcher(t *testing.T) (*sql.DB, *StatsBatcher) {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE stats_requests (date TEXT PRIMARY KEY, count INTEGER DEFAULT 0)`,
		`CREATE TABLE stats_tag (date TEXT NOT NULL, tag_id TEXT NOT NULL, count INTEGER DEFAULT 0, PRIMARY KEY (date, tag_id))`,
		`CREATE TABLE stats_source (date TEXT NOT NULL, source_id INTEGER NOT NULL, source_name TEXT NOT NULL, hit_count INTEGER DEFAULT 0, PRIMARY KEY (date, source_id))`,
		`CREATE TABLE image_history (id INTEGER PRIMARY KEY AUTOINCREMENT, image_url TEXT NOT NULL, source_id INTEGER, source_name TEXT, categories TEXT, created_at DATETIME, image_id INTEGER, file_id TEXT)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db, &StatsBatcher{
		db:   db,
		reqs: make(map[string]int64),
		tags: make(map[string]map[string]int64),
		srcs: make(map[string]map[int64]srcStat),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func TestStatsBatcherRestoresFailedFlush(t *testing.T) {
	db, batcher := newTestStatsBatcher(t)
	if _, err := db.Exec(`CREATE TRIGGER reject_stats_source BEFORE INSERT ON stats_source BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := batcher.Record(StatsEvent{QueryCats: []string{"cats"}, Source: model.Source{ID: 7, Name: "source"}, ImageURL: "https://example.test/1"}); err != nil {
		t.Fatal(err)
	}
	if err := batcher.flush(); err == nil {
		t.Fatal("expected flush to report the injected SQL error")
	}

	var requests, histories int
	if err := db.QueryRow(`SELECT count FROM stats_requests`).Scan(&requests); err != sql.ErrNoRows {
		t.Fatalf("failed transaction should not persist request stats: count=%d err=%v", requests, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM image_history`).Scan(&histories); err != nil || histories != 0 {
		t.Fatalf("failed transaction should not persist history: count=%d err=%v", histories, err)
	}
	batcher.mu.Lock()
	pendingRequests := batcher.reqs[time.Now().Format("2006-01-02")]
	pendingHistory := len(batcher.history)
	batcher.mu.Unlock()
	if pendingRequests != 1 || pendingHistory != 1 {
		t.Fatalf("failed flush should retain in-memory batch: requests=%d history=%d", pendingRequests, pendingHistory)
	}

	if _, err := db.Exec(`DROP TRIGGER reject_stats_source`); err != nil {
		t.Fatal(err)
	}
	if err := batcher.flush(); err != nil {
		t.Fatalf("flush retry: %v", err)
	}
	var requestCount, tagCount, sourceCount, historyCount int
	for query, dest := range map[string]*int{
		`SELECT count FROM stats_requests`:   &requestCount,
		`SELECT count FROM stats_tag`:        &tagCount,
		`SELECT hit_count FROM stats_source`: &sourceCount,
		`SELECT COUNT(*) FROM image_history`: &historyCount,
	} {
		if err := db.QueryRow(query).Scan(dest); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
	}
	if requestCount != 1 || tagCount != 1 || sourceCount != 1 || historyCount != 1 {
		t.Fatalf("retry should persist batch exactly once: requests=%d tags=%d sources=%d history=%d", requestCount, tagCount, sourceCount, historyCount)
	}
}

func TestStatsBatcherCloseStopsWorkerAndFlushes(t *testing.T) {
	db, batcher := newTestStatsBatcher(t)
	batcher.flushEvery = time.Hour
	go batcher.start()
	if err := batcher.Record(StatsEvent{Source: model.Source{ID: 9, Name: "source"}, ImageURL: "https://example.test/close"}); err != nil {
		t.Fatal(err)
	}
	if err := batcher.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-batcher.done:
	default:
		t.Fatal("Close returned before the periodic worker stopped")
	}
	var count int
	if err := db.QueryRow(`SELECT count FROM stats_requests`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("Close should flush pending stats: count=%d err=%v", count, err)
	}
}

func TestStatsBatcherCloseReportsFailureAndCanRetry(t *testing.T) {
	db, batcher := newTestStatsBatcher(t)
	if _, err := db.Exec(`CREATE TRIGGER reject_stats_source BEFORE INSERT ON stats_source BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	batcher.flushEvery = time.Hour
	go batcher.start()
	if err := batcher.Record(StatsEvent{Source: model.Source{ID: 11, Name: "source"}, ImageURL: "https://example.test/retry-close"}); err != nil {
		t.Fatal(err)
	}
	if err := batcher.Close(); err == nil {
		t.Fatal("Close should report its final flush failure")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("failed flush must leave database open for retry: %v", err)
	}
	if err := batcher.Record(StatsEvent{ImageURL: "https://example.test/late"}); err != ErrStatsBatcherClosed {
		t.Fatalf("Record after shutdown should be rejected, got %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_stats_source`); err != nil {
		t.Fatal(err)
	}
	if err := batcher.Close(); err != nil {
		t.Fatalf("Close retry: %v", err)
	}
	var requestCount, sourceCount, historyCount int
	if err := db.QueryRow(`SELECT count FROM stats_requests`).Scan(&requestCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT hit_count FROM stats_source`).Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM image_history`).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if requestCount != 1 || sourceCount != 1 || historyCount != 1 {
		t.Fatalf("Close retry should persist batch once: requests=%d sources=%d history=%d", requestCount, sourceCount, historyCount)
	}
}

func TestStatsBatcherBeginFailureRetainsBatch(t *testing.T) {
	goodDB, batcher := newTestStatsBatcher(t)
	badDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := badDB.Close(); err != nil {
		t.Fatal(err)
	}
	batcher.db = badDB
	if err := batcher.Record(StatsEvent{QueryCats: []string{"dogs"}, Source: model.Source{ID: 12, Name: "source"}, ImageURL: "https://example.test/begin"}); err != nil {
		t.Fatal(err)
	}
	if err := batcher.flush(); err == nil {
		t.Fatal("expected transaction begin failure")
	}
	batcher.db = goodDB
	if err := batcher.flush(); err != nil {
		t.Fatalf("flush after repairing database: %v", err)
	}
	var requests, tags, sources, history int
	for query, dest := range map[string]*int{
		`SELECT count FROM stats_requests`:   &requests,
		`SELECT count FROM stats_tag`:        &tags,
		`SELECT hit_count FROM stats_source`: &sources,
		`SELECT COUNT(*) FROM image_history`: &history,
	} {
		if err := goodDB.QueryRow(query).Scan(dest); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
	}
	if requests != 1 || tags != 1 || sources != 1 || history != 1 {
		t.Fatalf("batch should survive Begin failure: requests=%d tags=%d sources=%d history=%d", requests, tags, sources, history)
	}
}

func TestStoreCloseLeavesDatabaseOpenAfterStatsFlushFailure(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`CREATE TRIGGER reject_stats_source BEFORE INSERT ON stats_source BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordStats([]string{"cats"}, model.Source{ID: 13, Name: "source"}, "https://example.test/store-close", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err == nil {
		t.Fatal("Store.Close should propagate the batch flush failure")
	}
	if err := st.db.Ping(); err != nil {
		t.Fatalf("Store.Close must leave the database open for retry: %v", err)
	}
	if _, err := st.db.Exec(`DROP TRIGGER reject_stats_source`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Store.Close retry: %v", err)
	}
	var count int
	if err := st.db.QueryRow(`SELECT count FROM stats_requests`).Scan(&count); err == nil {
		t.Fatalf("database should close after successful retry, query unexpectedly returned count %d", count)
	}
}
