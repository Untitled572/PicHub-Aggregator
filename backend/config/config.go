package config

import (
	"encoding/json"
	"os"
)

type AppConfig struct {
	Settings SettingsConfig `json:"settings"`
	Sources  []SourceConfig `json:"sources"`
}

type SettingsConfig struct {
	ProxyMode     bool   `json:"proxy_mode"`
	CacheMaxMB    int    `json:"cache_max_mb"`
	CacheTTL      int    `json:"cache_ttl"`
	MinResolution string `json:"min_resolution"`
	RateLimit     int    `json:"rate_limit"`
	Timeout       int    `json:"timeout"`
	CustomDomain  string `json:"custom_domain"`
}


type SourceConfig struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	RespType   string   `json:"resp_type"`
	JsonPath   string   `json:"json_path"`
	Weight     int      `json:"weight"`
	Categories []string `json:"categories"`
}

func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
