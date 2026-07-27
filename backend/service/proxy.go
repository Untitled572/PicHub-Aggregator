package service

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pichub/backend/store"
)

type ProxyCache struct {
	store    *store.Store
	cacheDir string
	mu       sync.Mutex
}

func NewProxyCache(st *store.Store, cacheDir string) *ProxyCache {
	os.MkdirAll(cacheDir, 0755)
	return &ProxyCache{
		store:    st,
		cacheDir: cacheDir,
	}
}

type CacheEntry struct {
	Path      string    `json:"path"`
	URL       string    `json:"url"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func (pc *ProxyCache) GetOrFetch(imageURL string) ([]byte, string, error) {
	settings, err := pc.store.GetSettings()
	if err != nil {
		return nil, "", fmt.Errorf("get settings: %w", err)
	}
	if !settings.ProxyMode {
		return nil, "", fmt.Errorf("proxy mode not enabled")
	}

	cacheKey := fmt.Sprintf("%x", imageURL)
	cachePath := filepath.Join(pc.cacheDir, cacheKey)

	if data, err := os.ReadFile(cachePath); err == nil {
		pc.cleanupExpired(settings.CacheTTL)
		return data, http.DetectContentType(data), nil
	}

	client := &http.Client{Timeout: time.Duration(settings.Timeout) * time.Millisecond}
	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}

	if settings.MinResolution != "" {
		ok, err := checkResolution(data, settings.MinResolution)
		if err != nil || !ok {
			return nil, "", fmt.Errorf("resolution below minimum: %s", settings.MinResolution)
		}
	}

	pc.mu.Lock()
	totalSize := pc.getCacheSize()
	if totalSize+int64(len(data)) > int64(settings.CacheMaxMB)*1024*1024 {
		pc.evictLRU()
	}
	os.WriteFile(cachePath, data, 0644)
	pc.mu.Unlock()

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(data)
	}

	return data, ct, nil
}

func checkResolution(data []byte, minRes string) (bool, error) {
	cfg, _, err := image.DecodeConfig(strings.NewReader(string(data)))
	if err != nil {
		return true, nil
	}

	parts := strings.Split(minRes, "x")
	if len(parts) != 2 {
		return true, nil
	}
	minW, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	minH, _ := strconv.Atoi(strings.TrimSpace(parts[1]))

	if minW > 0 && cfg.Width < minW {
		return false, nil
	}
	if minH > 0 && cfg.Height < minH {
		return false, nil
	}
	return true, nil
}

func (pc *ProxyCache) getCacheSize() int64 {
	var total int64
	filepath.Walk(pc.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func (pc *ProxyCache) evictLRU() {
	entries, _ := filepath.Glob(filepath.Join(pc.cacheDir, "*"))
	if len(entries) == 0 {
		return
	}

	oldest := entries[0]
	oldestTime := time.Now()
	for _, e := range entries {
		info, err := os.Stat(e)
		if err != nil {
			continue
		}
		if info.ModTime().Before(oldestTime) {
			oldest = e
			oldestTime = info.ModTime()
		}
	}
	os.Remove(oldest)
}

func (pc *ProxyCache) cleanupExpired(ttlMinutes int) {
	entries, _ := filepath.Glob(filepath.Join(pc.cacheDir, "*"))
	now := time.Now()
	for _, e := range entries {
		info, err := os.Stat(e)
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()).Minutes() > float64(ttlMinutes) {
			os.Remove(e)
		}
	}
}
