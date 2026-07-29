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
}

type Settings struct {
	ProxyMode           bool     `json:"proxy_mode"`
	CacheMaxMB          int      `json:"cache_max_mb"`
	CacheTTL            int      `json:"cache_ttl"`
	MinResolution       string   `json:"min_resolution"`
	RateLimit           int      `json:"rate_limit"`
	Timeout             int      `json:"timeout"`
	HealthCheckInterval int      `json:"health_check_interval"`
	MaxHistoryRecords   int      `json:"max_history_records"`
	BoundTags           []string `json:"bound_tags"`
	AdminToken          string   `json:"admin_token,omitempty"`
}



