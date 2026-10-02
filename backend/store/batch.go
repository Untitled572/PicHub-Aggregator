package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pichub/backend/model"
)

type StatsEvent struct {
	QueryCats []string
	Source    model.Source
	ImageURL  string
	ImageID   *int64
	FileID    string
}

type srcStat struct {
	Name string
	Hits int64
}

type historyRow struct {
	ImageURL   string
	SourceID   int64
	SourceName string
	Categories string
	CreatedAt  string
	ImageID    int64
	FileID     string
}

type StatsBatcher struct {
	db         *sql.DB
	mu         sync.Mutex
	flushMu    sync.Mutex
	reqs       map[string]int64
	tags       map[string]map[string]int64
	srcs       map[string]map[int64]srcStat
	history    []historyRow
	flushEvery time.Duration
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
	closeMu    sync.Mutex
	closing    bool // guarded by mu; Record rejects events after shutdown begins
}

var ErrStatsBatcherClosed = errors.New("stats batcher is closing")

func NewStatsBatcher(db *sql.DB) *StatsBatcher {
	sb := &StatsBatcher{
		db:         db,
		reqs:       make(map[string]int64),
		tags:       make(map[string]map[string]int64),
		srcs:       make(map[string]map[int64]srcStat),
		flushEvery: 5 * time.Second,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go sb.start()
	return sb
}

// Record accepts events until Close begins. Calls after shutdown starts return
// ErrStatsBatcherClosed so the final flush cannot race with new events.
func (sb *StatsBatcher) Record(ev StatsEvent) error {
	now := time.Now()
	date := now.Format("2006-01-02")
	nowStr := now.Format("2006-01-02 15:04:05")

	sb.mu.Lock()
	if sb.closing {
		sb.mu.Unlock()
		return ErrStatsBatcherClosed
	}
	sb.reqs[date]++

	if len(ev.QueryCats) == 1 {
		cat := ev.QueryCats[0]
		if cat != "" && cat != "__uncategorized__" {
			if sb.tags[date] == nil {
				sb.tags[date] = make(map[string]int64)
			}
			sb.tags[date][cat]++
		}
	}

	if sb.srcs[date] == nil {
		sb.srcs[date] = make(map[int64]srcStat)
	}
	agg := sb.srcs[date][ev.Source.ID]
	agg.Name = ev.Source.Name
	agg.Hits++
	sb.srcs[date][ev.Source.ID] = agg

	imgID := int64(0)
	fileID := ""
	if ev.ImageID != nil {
		imgID = *ev.ImageID
		fileID = ev.FileID
	}
	sb.history = append(sb.history, historyRow{
		ImageURL:   ev.ImageURL,
		SourceID:   ev.Source.ID,
		SourceName: ev.Source.Name,
		Categories: encodeStringSlice(ev.QueryCats),
		CreatedAt:  nowStr,
		ImageID:    imgID,
		FileID:     fileID,
	})
	sb.mu.Unlock()
	return nil
}

func (sb *StatsBatcher) start() {
	defer close(sb.done)
	ticker := time.NewTicker(sb.flushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := sb.flush(); err != nil {
				log.Printf("stats batch flush failed: %v", err)
			}
		case <-sb.stop:
			return
		}
	}
}

func (sb *StatsBatcher) flush() error {
	sb.flushMu.Lock()
	defer sb.flushMu.Unlock()

	sb.mu.Lock()
	reqs := sb.reqs
	tags := sb.tags
	srcs := sb.srcs
	history := sb.history
	sb.reqs = make(map[string]int64)
	sb.tags = make(map[string]map[string]int64)
	sb.srcs = make(map[string]map[int64]srcStat)
	sb.history = nil
	sb.mu.Unlock()

	if len(reqs) == 0 && len(history) == 0 {
		return nil
	}

	tx, err := sb.db.Begin()
	if err != nil {
		sb.restore(reqs, tags, srcs, history)
		return fmt.Errorf("begin stats transaction: %w", err)
	}
	rollback := func(op string, cause error) error {
		_ = tx.Rollback()
		sb.restore(reqs, tags, srcs, history)
		return fmt.Errorf("%s: %w", op, cause)
	}

	for date, count := range reqs {
		if _, err = tx.Exec("INSERT INTO stats_requests (date, count) VALUES (?, ?) ON CONFLICT(date) DO UPDATE SET count = count + ?", date, count, count); err != nil {
			return rollback("write request stats", err)
		}
	}

	for date, tagMap := range tags {
		for tagID, count := range tagMap {
			if _, err = tx.Exec("INSERT INTO stats_tag (date, tag_id, count) VALUES (?, ?, ?) ON CONFLICT(date, tag_id) DO UPDATE SET count = count + ?", date, tagID, count, count); err != nil {
				return rollback("write tag stats", err)
			}
		}
	}

	for date, srcMap := range srcs {
		for srcID, stat := range srcMap {
			if _, err = tx.Exec("INSERT INTO stats_source (date, source_id, source_name, hit_count) VALUES (?, ?, ?, ?) ON CONFLICT(date, source_id) DO UPDATE SET hit_count = hit_count + ?, source_name = ?",
				date, srcID, stat.Name, stat.Hits, stat.Hits, stat.Name); err != nil {
				return rollback("write source stats", err)
			}
		}
	}

	for _, h := range history {
		if _, err = tx.Exec("INSERT INTO image_history (image_url, source_id, source_name, categories, created_at, image_id, file_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
			h.ImageURL, h.SourceID, h.SourceName, h.Categories, h.CreatedAt, h.ImageID, h.FileID); err != nil {
			return rollback("write image history", err)
		}
	}

	if err = tx.Commit(); err != nil {
		sb.restore(reqs, tags, srcs, history)
		return fmt.Errorf("commit stats transaction: %w", err)
	}
	return nil
}

func (sb *StatsBatcher) Close() error {
	sb.closeMu.Lock()
	defer sb.closeMu.Unlock()
	sb.mu.Lock()
	sb.closing = true
	sb.mu.Unlock()
	sb.stopOnce.Do(func() {
		close(sb.stop)
		<-sb.done
	})
	return sb.flush()
}

// restore puts a failed flush back in front of events recorded while the
// transaction was running, so a later successful flush persists every event.
func (sb *StatsBatcher) restore(reqs map[string]int64, tags map[string]map[string]int64, srcs map[string]map[int64]srcStat, history []historyRow) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	for date, count := range reqs {
		sb.reqs[date] += count
	}
	for date, oldTags := range tags {
		if sb.tags[date] == nil {
			sb.tags[date] = make(map[string]int64)
		}
		for tagID, count := range oldTags {
			sb.tags[date][tagID] += count
		}
	}
	for date, oldSources := range srcs {
		if sb.srcs[date] == nil {
			sb.srcs[date] = make(map[int64]srcStat)
		}
		for sourceID, oldStat := range oldSources {
			if current, ok := sb.srcs[date][sourceID]; ok {
				current.Hits += oldStat.Hits
				sb.srcs[date][sourceID] = current
			} else {
				sb.srcs[date][sourceID] = oldStat
			}
		}
	}
	if len(history) > 0 {
		pending := make([]historyRow, 0, len(history)+len(sb.history))
		pending = append(pending, history...)
		pending = append(pending, sb.history...)
		sb.history = pending
	}
}
