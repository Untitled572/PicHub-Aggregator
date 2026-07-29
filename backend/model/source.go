package model

import "time"

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type QueryParam struct {
	Key        string   `json:"key"`
	Value      string   `json:"value"`
	Weight     int      `json:"weight"`
	Categories []string `json:"categories"`
}

type Source struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	URL          string            `json:"url"`
	RespType     string            `json:"resp_type"`
	JsonPath     string            `json:"json_path"`
	Weight       int               `json:"weight"`
	Categories   []string          `json:"categories"`
	Headers      map[string]string `json:"headers"`
	Params       []QueryParam      `json:"params"`
	DefaultQuery string            `json:"default_query"`
	Enabled      bool              `json:"enabled"`
	FailCount    int               `json:"fail_count"`
	SuccessRate  float64           `json:"success_rate"`
	AvgLatency   int64             `json:"avg_latency"`
	Status       string            `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}


type DailyTrend struct {
	Date  string `json:"date"`
	Total int    `json:"total"`
}

type SourceDailyTrend struct {
	Date       string `json:"date"`
	SourceID   int64  `json:"source_id"`
	SourceName string `json:"source_name"`
	HitCount   int    `json:"hit_count"`
}

type StatsOverview struct {
	Total        int                `json:"total"`
	Tags         []TagStat          `json:"tags"`
	Sources      []SourceStat       `json:"sources"`
	DailyTrends  []DailyTrend       `json:"daily_trends"`
	SourceTrends []SourceDailyTrend `json:"source_trends"`
}


type TagStat struct {
	TagID string `json:"tag_id"`
	Count int    `json:"count"`
}

type SourceStat struct {
	SourceID   int64  `json:"source_id"`
	SourceName string `json:"source_name"`
	HitCount   int    `json:"hit_count"`
}

type ImageHistoryRecord struct {
	ID         int64     `json:"id"`
	ImageURL   string    `json:"image_url"`
	SourceID   int64     `json:"source_id"`
	SourceName string    `json:"source_name"`
	Categories string    `json:"categories"`
	CreatedAt  time.Time `json:"created_at"`
	ImageID    int64     `json:"image_id,omitempty"`
	FileID     string    `json:"file_id,omitempty"`
}

type CachedImage struct {
	ID          int64  `json:"id"`
	FileID      string `json:"file_id"`
	OriginalURL string `json:"original_url"`
	SourceID    int64  `json:"source_id"`
	SourceName  string `json:"source_name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	FileSize    int64  `json:"file_size"`
	Categories  string `json:"categories"`
	IsSaved     bool   `json:"is_saved"`
	CreatedAt   string `json:"created_at"`
}

type SavedImage struct {
	ID          int64     `json:"id"`
	FileID      string    `json:"file_id"`
	SourceName  string    `json:"source_name"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Format      string    `json:"format"`
	FileSize    int64     `json:"file_size"`
	OriginalURL string    `json:"original_url"`
	SavedAt     time.Time `json:"saved_at"`
}

type Settings struct {
	ProxyMode           bool     `json:"proxy_mode"`
	CacheMaxMB          int      `json:"cache_max_mb"`
	CacheMaxImages      int      `json:"cache_max_images"`
	CacheTTL            int      `json:"cache_ttl"`
	PrecacheCount       int      `json:"precache_count"`
	MinResolution       string   `json:"min_resolution"`
	RateLimit           int      `json:"rate_limit"`
	Timeout             int      `json:"timeout"`
	HealthCheckInterval int      `json:"health_check_interval"`
	MaxHistoryRecords   int      `json:"max_history_records"`
	BoundTags           []string `json:"bound_tags"`
	AdminToken          string   `json:"admin_token,omitempty"`
	SavedImagesDir      string   `json:"saved_images_dir,omitempty"`
}


type StatsRequestRow struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type StatsTagRow struct {
	Date  string `json:"date"`
	TagID string `json:"tag_id"`
	Count int    `json:"count"`
}

type StatsSourceRow struct {
	Date       string `json:"date"`
	SourceID   int64  `json:"source_id"`
	SourceName string `json:"source_name"`
	HitCount   int    `json:"hit_count"`
}

type StatsExportData struct {
	StatsRequests []StatsRequestRow    `json:"stats_requests,omitempty"`
	StatsTag      []StatsTagRow        `json:"stats_tag,omitempty"`
	StatsSource   []StatsSourceRow     `json:"stats_source,omitempty"`
	ImageHistory  []ImageHistoryRecord `json:"image_history,omitempty"`
}

type ExportManifest struct {
	Version     string           `json:"version"`
	ExportedAt  string           `json:"exported_at"`
	Scope       []string         `json:"scope"`
	Settings    *Settings        `json:"settings,omitempty"`
	Sources     []Source         `json:"sources,omitempty"`
	Tags        []Tag            `json:"tags,omitempty"`
	Stats       *StatsExportData `json:"stats,omitempty"`
	SavedImages []SavedImage     `json:"saved_images,omitempty"`
}




