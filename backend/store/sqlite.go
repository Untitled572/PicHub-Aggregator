package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pichub/backend/model"
)

type Store struct {
	db *sql.DB
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
	return s, nil
}

func (s *Store) Close() error {
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
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	_, _ = s.db.Exec("ALTER TABLE sources ADD COLUMN params TEXT DEFAULT '[]'")
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
		"proxy_mode":     "false",
		"cache_max_mb":   "200",
		"cache_ttl":      "60",
		"min_resolution": "640x480",
		"rate_limit":     "60",
		"timeout":        "3000",
	}
	for k, v := range defaults {
		if _, err := s.db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", k, v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListSources() ([]model.Source, error) {
	rows, err := s.db.Query("SELECT id, name, url, resp_type, json_path, weight, categories, headers, params, enabled, fail_count, success_rate, avg_latency, status, created_at, updated_at FROM sources ORDER BY id")
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
			&categoriesJSON, &headersJSON, &paramsJSON, &src.Enabled, &src.FailCount, &src.SuccessRate,
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
	err := s.db.QueryRow("SELECT id, name, url, resp_type, json_path, weight, categories, headers, params, enabled, fail_count, success_rate, avg_latency, status, created_at, updated_at FROM sources WHERE id=?", id).
		Scan(&src.ID, &src.Name, &src.URL, &src.RespType, &src.JsonPath, &src.Weight,
			&categoriesJSON, &headersJSON, &paramsJSON, &src.Enabled, &src.FailCount, &src.SuccessRate,
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
		"INSERT INTO sources (name, url, resp_type, json_path, weight, categories, headers, params, enabled, fail_count, success_rate, avg_latency, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		src.Name, src.URL, src.RespType, src.JsonPath, src.Weight, string(categoriesJSON), string(headersJSON), string(paramsJSON), src.Enabled, src.FailCount, src.SuccessRate, src.AvgLatency, src.Status,
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
		"UPDATE sources SET name=?, url=?, resp_type=?, json_path=?, weight=?, categories=?, headers=?, params=?, enabled=?, fail_count=?, success_rate=?, avg_latency=?, status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		src.Name, src.URL, src.RespType, src.JsonPath, src.Weight, string(categoriesJSON), string(headersJSON), string(paramsJSON), src.Enabled, src.FailCount, src.SuccessRate, src.AvgLatency, src.Status, src.ID,
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

func (s *Store) IncrementFailCount(id int64) error {
	_, err := s.db.Exec("UPDATE sources SET fail_count = fail_count + 1, updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
	return err
}

func (s *Store) ResetFailCount(id int64) error {
	_, err := s.db.Exec("UPDATE sources SET fail_count = 0, status='normal', updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
	return err
}

func (s *Store) GetSettings() (*model.Settings, error) {
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
		case "cache_max_mb":
			if n, err := fmt.Sscanf(v, "%d", &settings.CacheMaxMB); err != nil || n != 1 {
				settings.CacheMaxMB = 200
			}
		case "cache_ttl":
			if n, err := fmt.Sscanf(v, "%d", &settings.CacheTTL); err != nil || n != 1 {
				settings.CacheTTL = 60
			}
		case "min_resolution":
			settings.MinResolution = v
		case "rate_limit":
			if n, err := fmt.Sscanf(v, "%d", &settings.RateLimit); err != nil || n != 1 {
				settings.RateLimit = 60
			}
		case "timeout":
			if n, err := fmt.Sscanf(v, "%d", &settings.Timeout); err != nil || n != 1 {
				settings.Timeout = 3000
			}
		case "custom_domain":
			settings.CustomDomain = v
		case "health_check_interval":
			if n, err := fmt.Sscanf(v, "%d", &settings.HealthCheckInterval); err != nil || n != 1 {
				settings.HealthCheckInterval = 360
			}
		}
	}
	if settings.HealthCheckInterval <= 0 {
		settings.HealthCheckInterval = 360
	}
	return settings, nil
}

func (s *Store) UpdateSettings(settings *model.Settings) error {
	pairs := map[string]string{
		"proxy_mode":            fmt.Sprintf("%v", settings.ProxyMode),
		"cache_max_mb":          fmt.Sprintf("%d", settings.CacheMaxMB),
		"cache_ttl":             fmt.Sprintf("%d", settings.CacheTTL),
		"min_resolution":        settings.MinResolution,
		"rate_limit":            fmt.Sprintf("%d", settings.RateLimit),
		"timeout":               fmt.Sprintf("%d", settings.Timeout),
		"custom_domain":         settings.CustomDomain,
		"health_check_interval": fmt.Sprintf("%d", settings.HealthCheckInterval),
	}


	for k, v := range pairs {
		_, err := s.db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", k, v)
		if err != nil {
			return err
		}
	}
	return nil
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
