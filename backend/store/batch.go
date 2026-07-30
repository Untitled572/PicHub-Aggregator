package store

import (
	"database/sql"
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
	db          *sql.DB
	mu          sync.Mutex
	reqs        map[string]int64
	tags        map[string]map[string]int64
	srcs        map[string]map[int64]srcStat
	history     []historyRow
	flushEvery  time.Duration
}

func NewStatsBatcher(db *sql.DB) *StatsBatcher {
	sb := &StatsBatcher{
		db:         db,
		reqs:       make(map[string]int64),
		tags:       make(map[string]map[string]int64),
		srcs:       make(map[string]map[int64]srcStat),
		flushEvery: 5 * time.Second,
	}
	go sb.start()
	return sb
}

func (sb *StatsBatcher) Record(ev StatsEvent) {
	now := time.Now()
	date := now.Format("2006-01-02")
	nowStr := now.Format("2006-01-02 15:04:05")

	sb.mu.Lock()
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
}

func (sb *StatsBatcher) start() {
	for {
		time.Sleep(sb.flushEvery)
		sb.flush()
	}
}

func (sb *StatsBatcher) flush() {
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
		return
	}

	tx, err := sb.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	for date, count := range reqs {
		tx.Exec("INSERT INTO stats_requests (date, count) VALUES (?, ?) ON CONFLICT(date) DO UPDATE SET count = count + ?", date, count, count)
	}

	for date, tagMap := range tags {
		for tagID, count := range tagMap {
			tx.Exec("INSERT INTO stats_tag (date, tag_id, count) VALUES (?, ?, ?) ON CONFLICT(date, tag_id) DO UPDATE SET count = count + ?", date, tagID, count, count)
		}
	}

	for date, srcMap := range srcs {
		for srcID, stat := range srcMap {
			tx.Exec("INSERT INTO stats_source (date, source_id, source_name, hit_count) VALUES (?, ?, ?, ?) ON CONFLICT(date, source_id) DO UPDATE SET hit_count = hit_count + ?, source_name = ?",
				date, srcID, stat.Name, stat.Hits, stat.Hits, stat.Name)
		}
	}

	for _, h := range history {
		tx.Exec("INSERT INTO image_history (image_url, source_id, source_name, categories, created_at, image_id, file_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
			h.ImageURL, h.SourceID, h.SourceName, h.Categories, h.CreatedAt, h.ImageID, h.FileID)
	}

	tx.Commit()
}

func (sb *StatsBatcher) Close() {
	sb.flush()
}
