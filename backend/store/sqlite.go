package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pichub/backend/model"
)

type Store struct {
	db             *sql.DB
	batcher        *StatsBatcher
	cachedSettings *model.Settings
	settingsMu     sync.RWMutex
	settingsLoaded bool
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	s.batcher = NewStatsBatcher(db)
	return s, nil
}

func (s *Store) Close() error {
	if s.batcher != nil {
		s.batcher.Close()
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			resp_type TEXT DEFAULT 'json',
			json_path TEXT DEFAULT '',
			weight INTEGER DEFAULT 10,
			categories TEXT DEFAULT '[]',
			headers TEXT DEFAULT '{}',
			enabled INTEGER DEFAULT 1,
			fail_count INTEGER DEFAULT 0,
			success_rate REAL DEFAULT 100.0,
			avg_latency INTEGER DEFAULT 0,
			status TEXT DEFAULT 'normal',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS stats_requests (
			date TEXT PRIMARY KEY,
			count INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS stats_tag (
			date TEXT NOT NULL,
			tag_id TEXT NOT NULL,
			count INTEGER DEFAULT 0,
			PRIMARY KEY (date, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS stats_source (
			date TEXT NOT NULL,
			source_id INTEGER NOT NULL,
			source_name TEXT NOT NULL,
			hit_count INTEGER DEFAULT 0,
			PRIMARY KEY (date, source_id)
		)`,
		`CREATE TABLE IF NOT EXISTS image_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			image_url TEXT NOT NULL,
			source_id INTEGER,
			source_name TEXT,
			categories TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_id TEXT NOT NULL UNIQUE,
			original_url TEXT NOT NULL,
			source_id INTEGER NOT NULL DEFAULT 0,
			source_name TEXT NOT NULL DEFAULT '',
			width INTEGER NOT NULL DEFAULT 0,
			height INTEGER NOT NULL DEFAULT 0,
			format TEXT NOT NULL DEFAULT '',
			file_size INTEGER NOT NULL DEFAULT 0,
			categories TEXT NOT NULL DEFAULT '[]',
			is_saved INTEGER NOT NULL DEFAULT 0,
			saved_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	_, _ = s.db.Exec("ALTER TABLE sources ADD COLUMN params TEXT DEFAULT '[]'")
	_, _ = s.db.Exec("ALTER TABLE sources ADD COLUMN default_query TEXT DEFAULT ''")
	_, _ = s.db.Exec("ALTER TABLE image_history ADD COLUMN image_id INTEGER DEFAULT 0")
	_, _ = s.db.Exec("ALTER TABLE image_history ADD COLUMN file_id TEXT DEFAULT ''")
	_, _ = s.db.Exec("ALTER TABLE images ADD COLUMN orientation TEXT DEFAULT ''")
	if err := s.seedDefaults(); err != nil {

		return err
	}
	return nil
}

func (s *Store) seedDefaults() error {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaults := map[string]string{
		"proxy_mode":            "false",
		"proxy_enabled":         "false",
		"proxy_url":             "http://127.0.0.1:7890",
		"cache_max_mb":          "200",
		"cache_max_images":      "60",
		"cache_ttl":             "0",
		"precache_count":        "5",
		"pool_size":             "10",
		"min_resolution":        "1920x1080",
		"rate_limit":            "60",
		"timeout":               "3000",
		"health_check_interval": "360",
		"admin_token":           "",
		"saved_images_dir":      "./data/saved",
		"seeded":                "true",
	}


	for k, v := range defaults {
		if _, err := s.db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", k, v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SeedSources(sources []model.Source) error {
	var seeded string
	s.db.QueryRow("SELECT value FROM settings WHERE key='seeded'").Scan(&seeded)
	if seeded == "true" {
		return nil
	}
	for _, src := range sources {
		var exists int
		s.db.QueryRow("SELECT COUNT(*) FROM sources WHERE name=? AND url=?", src.Name, src.URL).Scan(&exists)
		if exists > 0 {
			continue
		}
		s.CreateSource(&src)
	}
	s.db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES ('seeded', 'true')")
	return nil
}

func (s *Store) ListSources() ([]model.Source, error) {
	rows, err := s.db.Query("SELECT id, name, url, resp_type, json_path, weight, categories, headers, params, default_query, enabled, fail_count, success_rate, avg_latency, status, created_at, updated_at FROM sources ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := make([]model.Source, 0)
	for rows.Next() {
		var src model.Source
		var categoriesJSON, headersJSON, paramsJSON string
		var createdAt, updatedAt string
		err := rows.Scan(&src.ID, &src.Name, &src.URL, &src.RespType, &src.JsonPath, &src.Weight,
			&categoriesJSON, &headersJSON, &paramsJSON, &src.DefaultQuery, &src.Enabled, &src.FailCount, &src.SuccessRate,
			&src.AvgLatency, &src.Status, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(categoriesJSON), &src.Categories)
		json.Unmarshal([]byte(headersJSON), &src.Headers)
		json.Unmarshal([]byte(paramsJSON), &src.Params)
		src.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		src.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		sources = append(sources, src)
	}
	return sources, nil
}

func (s *Store) GetSource(id int64) (*model.Source, error) {
	var src model.Source
	var categoriesJSON, headersJSON, paramsJSON string
	var createdAt, updatedAt string
	err := s.db.QueryRow("SELECT id, name, url, resp_type, json_path, weight, categories, headers, params, default_query, enabled, fail_count, success_rate, avg_latency, status, created_at, updated_at FROM sources WHERE id=?", id).
		Scan(&src.ID, &src.Name, &src.URL, &src.RespType, &src.JsonPath, &src.Weight,
			&categoriesJSON, &headersJSON, &paramsJSON, &src.DefaultQuery, &src.Enabled, &src.FailCount, &src.SuccessRate,
			&src.AvgLatency, &src.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(categoriesJSON), &src.Categories)
	json.Unmarshal([]byte(headersJSON), &src.Headers)
	json.Unmarshal([]byte(paramsJSON), &src.Params)
	src.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	src.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &src, nil
}

func (s *Store) CreateSource(src *model.Source) (int64, error) {
	categoriesJSON, _ := json.Marshal(src.Categories)
	headersJSON, _ := json.Marshal(src.Headers)
	paramsJSON, _ := json.Marshal(src.Params)
	result, err := s.db.Exec(
		"INSERT INTO sources (name, url, resp_type, json_path, weight, categories, headers, params, default_query, enabled, fail_count, success_rate, avg_latency, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		src.Name, src.URL, src.RespType, src.JsonPath, src.Weight, string(categoriesJSON), string(headersJSON), string(paramsJSON), src.DefaultQuery, src.Enabled, src.FailCount, src.SuccessRate, src.AvgLatency, src.Status,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) UpdateSource(src *model.Source) error {
	categoriesJSON, _ := json.Marshal(src.Categories)
	headersJSON, _ := json.Marshal(src.Headers)
	paramsJSON, _ := json.Marshal(src.Params)
	_, err := s.db.Exec(
		"UPDATE sources SET name=?, url=?, resp_type=?, json_path=?, weight=?, categories=?, headers=?, params=?, default_query=?, enabled=?, fail_count=?, success_rate=?, avg_latency=?, status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		src.Name, src.URL, src.RespType, src.JsonPath, src.Weight, string(categoriesJSON), string(headersJSON), string(paramsJSON), src.DefaultQuery, src.Enabled, src.FailCount, src.SuccessRate, src.AvgLatency, src.Status, src.ID,
	)
	return err
}

func (s *Store) DeleteSource(id int64) error {
	_, err := s.db.Exec("DELETE FROM sources WHERE id=?", id)
	return err
}

func (s *Store) UpdateSourceStatus(id int64, status string) error {
	_, err := s.db.Exec("UPDATE sources SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?", status, id)
	return err
}

func (s *Store) ResetAllSourceErrors() error {
	_, err := s.db.Exec("UPDATE sources SET status='normal', fail_count=0 WHERE status='error'")
	return err
}

func (s *Store) IncrementSourceWeight(sourceID int64, delta int) error {
	_, err := s.db.Exec("UPDATE sources SET weight = MAX(30, MIN(70, weight + ?)), updated_at = CURRENT_TIMESTAMP WHERE id = ?", delta, sourceID)
	return err
}

func (s *Store) IncrementFailCount(id int64) (int, error) {
	_, err := s.db.Exec("UPDATE sources SET fail_count = fail_count + 1, updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
	if err != nil {
		return 0, err
	}
	var fc int
	err = s.db.QueryRow("SELECT fail_count FROM sources WHERE id=?", id).Scan(&fc)
	return fc, err
}

func (s *Store) ResetFailCount(id int64) error {
	_, err := s.db.Exec("UPDATE sources SET fail_count = 0, status='normal', updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
	return err
}

func (s *Store) GetSettings() (*model.Settings, error) {
	s.settingsMu.RLock()
	if s.settingsLoaded {
		defer s.settingsMu.RUnlock()
		return s.cachedSettings, nil
	}
	s.settingsMu.RUnlock()

	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	if s.settingsLoaded {
		return s.cachedSettings, nil
	}

	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settings := &model.Settings{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		switch k {
		case "proxy_mode":
			settings.ProxyMode = v == "true"
		case "proxy_enabled":
			settings.ProxyEnabled = v == "true"
		case "proxy_url":
			settings.ProxyURL = v
		case "cache_max_mb":
			if n, err := fmt.Sscanf(v, "%d", &settings.CacheMaxMB); err != nil || n != 1 {
				settings.CacheMaxMB = 200
			}
		case "cache_max_images":
			if n, err := fmt.Sscanf(v, "%d", &settings.CacheMaxImages); err != nil || n != 1 {
				settings.CacheMaxImages = 60
			}
		case "cache_ttl":
			if n, err := fmt.Sscanf(v, "%d", &settings.CacheTTL); err != nil || n != 1 {
				settings.CacheTTL = 0
			}
		case "precache_count":
			if n, err := fmt.Sscanf(v, "%d", &settings.PrecacheCount); err != nil || n != 1 {
				settings.PrecacheCount = 5
			}
		case "pool_size":
			if n, err := fmt.Sscanf(v, "%d", &settings.PoolSize); err != nil || n != 1 {
				settings.PoolSize = 10
			}
		case "min_resolution":
			settings.MinResolution = v
		case "rate_limit":
			if n, err := fmt.Sscanf(v, "%d", &settings.RateLimit); err != nil || n != 1 {
				settings.RateLimit = 60
			}
		case "rate_limit_window":
			if n, err := fmt.Sscanf(v, "%d", &settings.RateLimitWindow); err != nil || n != 1 {
				settings.RateLimitWindow = 60
			}
		case "timeout":
			if n, err := fmt.Sscanf(v, "%d", &settings.Timeout); err != nil || n != 1 {
				settings.Timeout = 3000
			}
		case "custom_domain":
			_ = v
		case "health_check_interval":
			if n, err := fmt.Sscanf(v, "%d", &settings.HealthCheckInterval); err != nil || n != 1 {
				settings.HealthCheckInterval = 360
			}
		case "max_history_records":
			if n, err := fmt.Sscanf(v, "%d", &settings.MaxHistoryRecords); err != nil || n != 1 {
				settings.MaxHistoryRecords = 60
			}
		case "bound_tags":
			if v != "" {
				json.Unmarshal([]byte(v), &settings.BoundTags)
			}
		case "admin_token":
			settings.AdminToken = v
		case "saved_images_dir":
			settings.SavedImagesDir = v
		case "seeded":
			_ = v
		}
	}
	if settings.HealthCheckInterval <= 0 {
		settings.HealthCheckInterval = 360
	}
	if settings.MaxHistoryRecords <= 0 {
		settings.MaxHistoryRecords = 60
	}
	if settings.CacheMaxImages <= 0 {
		settings.CacheMaxImages = 60
	}
	if settings.PrecacheCount < 0 {
		settings.PrecacheCount = 5
	}
	if settings.PoolSize <= 0 {
		settings.PoolSize = 10
	}
	if settings.RateLimitWindow <= 0 {
		settings.RateLimitWindow = 60
	}
	s.cachedSettings = settings
	s.settingsLoaded = true
	return settings, nil
}

func (s *Store) UpdateSettings(settings *model.Settings) error {
	pairs := map[string]string{
		"proxy_mode":            fmt.Sprintf("%v", settings.ProxyMode),
		"proxy_enabled":         fmt.Sprintf("%v", settings.ProxyEnabled),
		"proxy_url":             settings.ProxyURL,
		"cache_max_mb":          fmt.Sprintf("%d", settings.CacheMaxMB),
		"cache_max_images":      fmt.Sprintf("%d", settings.CacheMaxImages),
		"cache_ttl":             fmt.Sprintf("%d", settings.CacheTTL),
		"precache_count":        fmt.Sprintf("%d", settings.PrecacheCount),
		"pool_size":             fmt.Sprintf("%d", settings.PoolSize),
		"min_resolution":        settings.MinResolution,
		"rate_limit":            fmt.Sprintf("%d", settings.RateLimit),
		"rate_limit_window":     fmt.Sprintf("%d", settings.RateLimitWindow),
		"timeout":               fmt.Sprintf("%d", settings.Timeout),
		"custom_domain":         "",
		"health_check_interval": fmt.Sprintf("%d", settings.HealthCheckInterval),
		"max_history_records":   fmt.Sprintf("%d", settings.MaxHistoryRecords),
		"bound_tags":            encodeBoundTags(settings.BoundTags),
		"admin_token":           settings.AdminToken,
		"saved_images_dir":      settings.SavedImagesDir,
	}


	for k, v := range pairs {
		_, err := s.db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", k, v)
		if err != nil {
			return err
		}
	}
	s.settingsMu.Lock()
	s.cachedSettings = settings
	s.settingsLoaded = true
	s.settingsMu.Unlock()
	return nil
}

func encodeBoundTags(tags []string) string {
	if tags == nil {
		return ""
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

func systemTags() []model.Tag {
	return []model.Tag{
		{ID: "horizontal", Name: "横屏", System: true},
		{ID: "vertical", Name: "竖屏", System: true},
		{ID: "adaptive", Name: "自适应", System: true},
	}
}

func (s *Store) GetTags() ([]model.Tag, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key='tags'").Scan(&v)
	if err != nil {
		return systemTags(), nil
	}
	var tags []model.Tag
	if v != "" {
		if err := json.Unmarshal([]byte(v), &tags); err != nil {
			tags = nil
		}
	}
	merged := mergeWithSystemTags(tags)
	for i := range merged {
		merged[i].System = isSystemTag(merged[i].ID)
	}
	return merged, nil
}


func (s *Store) UpdateTags(tags []model.Tag) error {
	var kept []model.Tag
	for _, t := range tags {
		if !isSystemTag(t.ID) {
			kept = append(kept, t)
		}
	}
	finalTags := mergeWithSystemTags(kept)
	b, _ := json.Marshal(finalTags)
	_, err := s.db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES ('tags', ?)", string(b))
	return err
}

func isSystemTag(id string) bool {
	return id == "horizontal" || id == "vertical" || id == "adaptive"
}

func mergeWithSystemTags(tags []model.Tag) []model.Tag {
	seen := make(map[string]bool)
	for _, t := range tags {
		seen[t.ID] = true
	}
	var result []model.Tag
	for _, st := range systemTags() {
		result = append(result, st)
	}
	for _, t := range tags {
		if !seen[t.ID] {
			continue
		}
		if !isSystemTag(t.ID) {
			result = append(result, t)
		}
	}
	return result
}

func defaultTags() []model.Tag {
	return []model.Tag{
		{ID: "horizontal", Name: "横屏", System: true},
		{ID: "vertical", Name: "竖屏", System: true},
		{ID: "adaptive", Name: "自适应"},
		{ID: "r18", Name: "R18", Exclusive: true},
	}
}

func hasExclusiveTag(tags []model.Tag, ids []string) bool {
	for _, id := range ids {
		for _, t := range tags {
			if t.ID == id && t.Exclusive {
				return true
			}
		}
	}
	return false
}

func (s *Store) GetEnabledSources() ([]model.Source, error) {
	all, err := s.ListSources()
	if err != nil {
		return nil, err
	}
	var enabled []model.Source
	for _, src := range all {
		if src.Enabled {
			enabled = append(enabled, src)
		}
	}
	return enabled, nil
}

func (s *Store) RecordStats(queryCats []string, src model.Source, imageURL string, imageID *int64, fileID string) error {
	if s.batcher != nil {
		s.batcher.Record(StatsEvent{
			QueryCats: queryCats,
			Source:    src,
			ImageURL:  imageURL,
			ImageID:   imageID,
			FileID:    fileID,
		})
	}
	return nil
}

func (s *Store) InsertImage(fileID, originalURL string, sourceID int64, sourceName string, width, height int, format string, fileSize int64, categories string, orientation string) (int64, error) {
	result, err := s.db.Exec(
		"INSERT INTO images (file_id, original_url, source_id, source_name, width, height, format, file_size, categories, orientation) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		fileID, originalURL, sourceID, sourceName, width, height, format, fileSize, categories, orientation,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) GetImageByFileID(fileID string) (*model.CachedImage, error) {
	var img model.CachedImage
	var categories string
	var createdAt string
	var isSaved int
	var savedAt sql.NullString
	err := s.db.QueryRow(
		"SELECT id, file_id, original_url, source_id, source_name, width, height, format, file_size, categories, COALESCE(orientation,''), is_saved, created_at FROM images WHERE file_id=? OR id=?",
		fileID, fileID,
	).Scan(&img.ID, &img.FileID, &img.OriginalURL, &img.SourceID, &img.SourceName, &img.Width, &img.Height, &img.Format, &img.FileSize, &categories, &img.Orientation, &isSaved, &createdAt)

	if err != nil {
		return nil, err
	}
	img.Categories = categories
	img.IsSaved = isSaved > 0
	img.CreatedAt = createdAt
	_ = savedAt
	return &img, nil
}

func (s *Store) GetImageByID(id int64) (*model.CachedImage, error) {
	var img model.CachedImage
	var categories string
	var createdAt string
	var isSaved int
	var savedAt sql.NullString
	err := s.db.QueryRow(
		"SELECT id, file_id, original_url, source_id, source_name, width, height, format, file_size, categories, COALESCE(orientation,''), is_saved, created_at FROM images WHERE id=?",
		id,
	).Scan(&img.ID, &img.FileID, &img.OriginalURL, &img.SourceID, &img.SourceName, &img.Width, &img.Height, &img.Format, &img.FileSize, &categories, &img.Orientation, &isSaved, &createdAt)
	if err != nil {
		return nil, err
	}
	img.Categories = categories
	img.IsSaved = isSaved > 0
	img.CreatedAt = createdAt
	_ = savedAt
	return &img, nil
}

func (s *Store) DeleteImageByFileID(fileID string) error {
	_, err := s.db.Exec("DELETE FROM images WHERE file_id=?", fileID)
	return err
}

func (s *Store) UpdateImageSaved(id int64, saved bool) error {
	if saved {
		_, err := s.db.Exec("UPDATE images SET is_saved=1, saved_at=CURRENT_TIMESTAMP WHERE id=?", id)
		return err
	}
	_, err := s.db.Exec("UPDATE images SET is_saved=0, saved_at=NULL WHERE id=?", id)
	return err
}

func (s *Store) ListSavedImages(limit, offset int) ([]model.SavedImage, int, error) {
	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM images WHERE is_saved=1").Scan(&total)

	rows, err := s.db.Query(
		"SELECT id, file_id, source_name, width, height, format, file_size, original_url, COALESCE(orientation,''), COALESCE(saved_at, created_at) FROM images WHERE is_saved=1 ORDER BY saved_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []model.SavedImage
	for rows.Next() {
		var si model.SavedImage
		var savedAt string
		if err := rows.Scan(&si.ID, &si.FileID, &si.SourceName, &si.Width, &si.Height, &si.Format, &si.FileSize, &si.OriginalURL, &si.Orientation, &savedAt); err != nil {
			return nil, 0, err
		}
		si.SavedAt, _ = time.Parse("2006-01-02 15:04:05", savedAt)
		list = append(list, si)
	}
	if list == nil {
		list = []model.SavedImage{}
	}
	return list, total, nil
}

func encodeStringSlice(s []string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (s *Store) GetTodayStats() (*model.StatsOverview, error) {
	today := time.Now().Format("2006-01-02")
	return s.GetStatsRange(today, today)
}

func (s *Store) GetStatsRange(startDate, endDate string) (*model.StatsOverview, error) {
	overview := &model.StatsOverview{}

	s.db.QueryRow("SELECT COALESCE(SUM(count), 0) FROM stats_requests WHERE date BETWEEN ? AND ?", startDate, endDate).Scan(&overview.Total)

	tagRows, err := s.db.Query("SELECT tag_id, SUM(count) AS total_count FROM stats_tag WHERE date BETWEEN ? AND ? AND tag_id != '__uncategorized__' GROUP BY tag_id ORDER BY total_count DESC", startDate, endDate)
	if err == nil && tagRows != nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var ts model.TagStat
			if tagRows.Scan(&ts.TagID, &ts.Count) == nil {
				overview.Tags = append(overview.Tags, ts)
			}
		}
	}

	srcRows, err := s.db.Query("SELECT source_id, source_name, SUM(hit_count) AS total_hits FROM stats_source WHERE date BETWEEN ? AND ? GROUP BY source_id, source_name ORDER BY total_hits DESC", startDate, endDate)
	if err == nil && srcRows != nil {
		defer srcRows.Close()
		for srcRows.Next() {
			var ss model.SourceStat
			if srcRows.Scan(&ss.SourceID, &ss.SourceName, &ss.HitCount) == nil {
				overview.Sources = append(overview.Sources, ss)
			}
		}
	}

	if overview.Tags == nil {
		overview.Tags = []model.TagStat{}
	}
	if overview.Sources == nil {
		overview.Sources = []model.SourceStat{}
	}

	dailyRows, err := s.db.Query("SELECT date, count FROM stats_requests WHERE date BETWEEN ? AND ? ORDER BY date ASC", startDate, endDate)
	if err == nil && dailyRows != nil {
		defer dailyRows.Close()
		for dailyRows.Next() {
			var dt model.DailyTrend
			if dailyRows.Scan(&dt.Date, &dt.Total) == nil {
				overview.DailyTrends = append(overview.DailyTrends, dt)
			}
		}
	}

	srcTrendRows, err := s.db.Query("SELECT date, source_id, source_name, hit_count FROM stats_source WHERE date BETWEEN ? AND ? ORDER BY date ASC, hit_count DESC", startDate, endDate)
	if err == nil && srcTrendRows != nil {
		defer srcTrendRows.Close()
		for srcTrendRows.Next() {
			var sdt model.SourceDailyTrend
			if srcTrendRows.Scan(&sdt.Date, &sdt.SourceID, &sdt.SourceName, &sdt.HitCount) == nil {
				overview.SourceTrends = append(overview.SourceTrends, sdt)
			}
		}
	}

	if overview.DailyTrends == nil {
		overview.DailyTrends = []model.DailyTrend{}
	}
	if overview.SourceTrends == nil {
		overview.SourceTrends = []model.SourceDailyTrend{}
	}

	return overview, nil
}


func (s *Store) GetTotalRequests() (int, error) {
	var total int
	err := s.db.QueryRow("SELECT COALESCE(SUM(count), 0) FROM stats_requests").Scan(&total)
	return total, err
}

func (s *Store) GetImageHistory(limit, offset int) ([]model.ImageHistoryRecord, int, error) {
	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM image_history").Scan(&total)

	query := `
		SELECT h.id, h.image_url, h.source_id, h.source_name, h.categories, h.created_at, COALESCE(h.image_id,0), COALESCE(h.file_id,''),
		       COALESCE(img.is_saved, 0)
		FROM image_history h
		LEFT JOIN images img ON (h.file_id != '' AND h.file_id = img.file_id) OR (h.image_id > 0 AND h.image_id = img.id)
		ORDER BY h.id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []model.ImageHistoryRecord
	for rows.Next() {
		var r model.ImageHistoryRecord
		var createdAt string
		var isSaved int
		if err := rows.Scan(&r.ID, &r.ImageURL, &r.SourceID, &r.SourceName, &r.Categories, &createdAt, &r.ImageID, &r.FileID, &isSaved); err != nil {
			return nil, 0, err
		}
		r.IsSaved = isSaved > 0
		parsedTime, err := time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			parsedTime, err = time.Parse(time.RFC3339, createdAt)
		}
		if err != nil {
			parsedTime, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		}
		r.CreatedAt = parsedTime
		records = append(records, r)
	}
	if records == nil {
		records = []model.ImageHistoryRecord{}
	}
	return records, total, nil
}


func (s *Store) ExportStatsData() (*model.StatsExportData, error) {

	data := &model.StatsExportData{}

	reqRows, err := s.db.Query("SELECT date, count FROM stats_requests ORDER BY date ASC")
	if err == nil && reqRows != nil {
		defer reqRows.Close()
		for reqRows.Next() {
			var r model.StatsRequestRow
			if reqRows.Scan(&r.Date, &r.Count) == nil {
				data.StatsRequests = append(data.StatsRequests, r)
			}
		}
	}

	tagRows, err := s.db.Query("SELECT date, tag_id, count FROM stats_tag ORDER BY date ASC")
	if err == nil && tagRows != nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var t model.StatsTagRow
			if tagRows.Scan(&t.Date, &t.TagID, &t.Count) == nil {
				data.StatsTag = append(data.StatsTag, t)
			}
		}
	}

	srcRows, err := s.db.Query("SELECT date, source_id, source_name, hit_count FROM stats_source ORDER BY date ASC")
	if err == nil && srcRows != nil {
		defer srcRows.Close()
		for srcRows.Next() {
			var sr model.StatsSourceRow
			if srcRows.Scan(&sr.Date, &sr.SourceID, &sr.SourceName, &sr.HitCount) == nil {
				data.StatsSource = append(data.StatsSource, sr)
			}
		}
	}

	history, _, _ := s.GetImageHistory(1000, 0)
	data.ImageHistory = history

	return data, nil
}

func (s *Store) ImportStatsData(data *model.StatsExportData) error {
	if data == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, r := range data.StatsRequests {
		tx.Exec("INSERT INTO stats_requests (date, count) VALUES (?, ?) ON CONFLICT(date) DO UPDATE SET count = count + ?", r.Date, r.Count, r.Count)
	}

	for _, t := range data.StatsTag {
		tx.Exec("INSERT INTO stats_tag (date, tag_id, count) VALUES (?, ?, ?) ON CONFLICT(date, tag_id) DO UPDATE SET count = count + ?", t.Date, t.TagID, t.Count, t.Count)
	}

	for _, sr := range data.StatsSource {
		tx.Exec("INSERT INTO stats_source (date, source_id, source_name, hit_count) VALUES (?, ?, ?, ?) ON CONFLICT(date, source_id) DO UPDATE SET hit_count = hit_count + ?", sr.Date, sr.SourceID, sr.SourceName, sr.HitCount, sr.HitCount)
	}

	for _, h := range data.ImageHistory {
		nowStr := h.CreatedAt.Format("2006-01-02 15:04:05")
		tx.Exec("INSERT INTO image_history (image_url, source_id, source_name, categories, created_at, image_id, file_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
			h.ImageURL, h.SourceID, h.SourceName, h.Categories, nowStr, h.ImageID, h.FileID)
	}

	return tx.Commit()
}

func (s *Store) ExportSavedImages() ([]model.SavedImage, error) {
	images, _, err := s.ListSavedImages(1000, 0)
	return images, err
}

func (s *Store) ImportSavedImage(img *model.SavedImage) error {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM images WHERE file_id=?", img.FileID).Scan(&count)
	savedAtStr := img.SavedAt.Format("2006-01-02 15:04:05")
	if count > 0 {
		_, err := s.db.Exec("UPDATE images SET is_saved=1, saved_at=? WHERE file_id=?", savedAtStr, img.FileID)
		return err
	}
	_, err := s.db.Exec(
		"INSERT INTO images (file_id, original_url, source_id, source_name, width, height, format, file_size, categories, orientation, is_saved, created_at, saved_at) VALUES (?, ?, 0, ?, ?, ?, ?, ?, '[]', ?, 1, ?, ?)",
		img.FileID, img.OriginalURL, img.SourceName, img.Width, img.Height, img.Format, img.FileSize, img.Orientation, savedAtStr, savedAtStr,
	)
	return err
}



