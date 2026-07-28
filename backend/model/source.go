package model

import "time"

type QueryParam struct {
	Key        string   `json:"key"`
	Value      string   `json:"value"`
	Weight     int      `json:"weight"`
	Categories []string `json:"categories"`
}

type Source struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	RespType    string            `json:"resp_type"`
	JsonPath    string            `json:"json_path"`
	Weight      int               `json:"weight"`
	Categories  []string          `json:"categories"`
	Headers     map[string]string `json:"headers"`
	Params      []QueryParam      `json:"params"`
	Enabled     bool              `json:"enabled"`
	FailCount   int               `json:"fail_count"`
	SuccessRate float64           `json:"success_rate"`
	AvgLatency  int64             `json:"avg_latency"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}


type Settings struct {
	ProxyMode           bool   `json:"proxy_mode"`
	CacheMaxMB          int    `json:"cache_max_mb"`
	CacheTTL            int    `json:"cache_ttl"`
	MinResolution       string `json:"min_resolution"`
	RateLimit           int    `json:"rate_limit"`
	Timeout             int    `json:"timeout"`
	CustomDomain        string `json:"custom_domain"`
	HealthCheckInterval int    `json:"health_check_interval"`
}


