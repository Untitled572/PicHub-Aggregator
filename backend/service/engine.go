package service

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

type Engine struct {
	store        *store.Store
	proxyCache   *ProxyCache
	imageStore   *ImageStore
	precachePool *PrecachePool
	httpClient   *http.Client
}

func NewEngine(st *store.Store, pc *ProxyCache, imgStore *ImageStore) *Engine {
	eng := &Engine{
		store:        st,
		proxyCache:   pc,
		imageStore:   imgStore,
		precachePool: NewPrecachePool(),
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	go eng.StartPrecacheWorker()
	return eng
}

type Result struct {
	URL        string   `json:"url"`
	LocalURL   string   `json:"local_url,omitempty"`
	SourceName string   `json:"source"`
	SourceID   int64    `json:"source_id,omitempty"`
	Categories []string `json:"categories"`
	FileID     string   `json:"file_id,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	Format     string   `json:"format,omitempty"`
	ImageID    int64    `json:"image_id,omitempty"`
}

func (e *Engine) RandomImage(category string, format string, orientation string, clientUA string) (*Result, int, error) {
	settings, err := e.store.GetSettings()
	if err != nil {
		return nil, 0, fmt.Errorf("get settings: %w", err)
	}

	queryCats := splitCategory(category)

	if settings.ProxyMode && settings.PrecacheCount > 0 && e.precachePool != nil {
		cachedResult := e.precachePool.Pop(category)
		if cachedResult != nil {
			if !precacheFileExists(cachedResult, e.imageStore) {
				e.precachePool.Discard(cachedResult)
				go e.ReplenishPrecache()
			} else if orientation != "" && cachedResult.Width > 0 {
				imgOrient := GetOrientation(cachedResult.Width, cachedResult.Height)
				if imgOrient != orientation {
					logger.System("precache orientation mismatch: got %s, wanted %s", imgOrient, orientation)
					e.precachePool.Discard(cachedResult)
					go e.ReplenishPrecache()
				} else {
					logger.System("precache hit for category %q, returning 0ms instant response", category)
					go e.recordPrecacheStats(queryCats, cachedResult)
					go e.ReplenishPrecache()
					return cachedResult, http.StatusFound, nil
				}
			} else {
				logger.System("precache hit for category %q, returning 0ms instant response", category)
				go e.recordPrecacheStats(queryCats, cachedResult)
				go e.ReplenishPrecache()
				return cachedResult, http.StatusFound, nil
			}
		}
	}

	sources, err := e.store.ListSources()
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}


	candidates := filterSources(sources, queryCats)
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no available sources")
	}

	timeout := time.Duration(settings.Timeout) * time.Millisecond

	maxAttempts := 3
	if e.imageStore != nil && settings.ProxyMode && orientation != "" {
		maxAttempts = 8
	}

	for attempt := 0; attempt < maxAttempts && len(candidates) > 0; attempt++ {
		selected := weightedPick(candidates)

		req, err := http.NewRequest("GET", selected.URL, nil)
		if err != nil {
			candidates = removeSource(candidates, selected.ID)
			continue
		}
		for k, v := range selected.Headers {
			req.Header.Set(k, v)
		}
		if clientUA != "" {
			hasUA := false
			for k := range selected.Headers {
				if strings.EqualFold(k, "User-Agent") {
					hasUA = true
					break
				}
			}
			if !hasUA {
				req.Header.Set("User-Agent", clientUA)
			}
		}

		e.httpClient.Timeout = timeout
		resp, err := e.httpClient.Do(req)
		if err != nil {
			logger.Error("source %q fetch failed: %v", selected.Name, err)
			failCount, _ := e.store.IncrementFailCount(selected.ID)
			candidates = removeSource(candidates, selected.ID)
			if failCount >= 5 {
				e.store.UpdateSourceStatus(selected.ID, "error")
			}
			continue
		}

		hasFailed := false
		var cachedInfo *CachedImageInfo

		if resp.StatusCode >= 500 {
			logger.Error("source %q returned %d", selected.Name, resp.StatusCode)
			failCount, _ := e.store.IncrementFailCount(selected.ID)
			candidates = removeSource(candidates, selected.ID)
			if failCount >= 5 {
				e.store.UpdateSourceStatus(selected.ID, "error")
			}
			resp.Body.Close()
			continue
		}

		e.store.ResetFailCount(selected.ID)

		imageURL, err := extractImageURL(resp, selected)
		resp.Body.Close()
		if err != nil {
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		e.store.UpdateSourceStatus(selected.ID, "normal")

		origURL := imageURL

		if e.imageStore != nil && settings.ProxyMode {
			cached, cacheErr := e.imageStore.DownloadAndStore(origURL, selected.ID, selected.Name, queryCats)
			if cacheErr != nil {
				logger.Error("cache download failed for %s: %v", selected.Name, cacheErr)
				candidates = removeSource(candidates, selected.ID)
				continue
			}
			cachedInfo = cached

			if orientation != "" {
				imgOrient := GetOrientation(cached.Width, cached.Height)
				if imgOrient != orientation {
					logger.System("orientation mismatch for %s: got %s, wanted %s", selected.Name, imgOrient, orientation)
					candidates = removeSource(candidates, selected.ID)
					continue
				}
			}

			imageURL = "/images/" + cached.FileID
		} else if e.proxyCache != nil && settings.ProxyMode {
			_, _, proxyErr := e.proxyCache.GetOrFetch(imageURL)
			if proxyErr != nil {
				hasFailed = true
			}
		}

		if hasFailed {
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		var imgID *int64
		var fileID string
		if cachedInfo != nil {
			imgID = &cachedInfo.ID
			fileID = cachedInfo.FileID
		}
		go e.store.RecordStats(queryCats, selected, origURL, imgID, fileID)

		res := &Result{
			URL:        origURL,
			SourceName: selected.Name,
			Categories: selected.Categories,
		}
		if cachedInfo != nil {
			res.LocalURL = imageURL
			res.SourceID = selected.ID
			res.FileID = cachedInfo.FileID
			res.Width = cachedInfo.Width
			res.Height = cachedInfo.Height
			res.Format = cachedInfo.Format
			res.ImageID = cachedInfo.ID
		}

		if format == "json" {
			return res, http.StatusOK, nil
		}
		return res, http.StatusFound, nil
	}

	return nil, 0, fmt.Errorf("all sources failed")
}

func filterSources(sources []model.Source, queryCats []string) []model.Source {
	var result []model.Source
	hasQueryCat := len(queryCats) > 0 && queryCats[0] != ""
	for _, src := range sources {
		if !src.Enabled || src.Status == "error" {
			continue
		}
		if len(src.Categories) == 0 {
			src.Categories = []string{"__uncategorized__"}
		}

		if len(src.Params) > 0 {
			for _, param := range src.Params {
				key := strings.TrimSpace(param.Key)
				val := strings.TrimSpace(param.Value)
				if key == "" {
					continue
				}
				paramURL := appendDefaultQuery(buildURL(src.URL, key, val), src.DefaultQuery)
				paramWeight := param.Weight
				if paramWeight <= 0 {
					paramWeight = src.Weight
				}
				paramCats := param.Categories
				if len(paramCats) == 0 {
					paramCats = src.Categories
				}

				if len(param.Categories) > 0 && hasQueryCat {
					if !hasAnyCategory(param.Categories, queryCats) {
						continue
					}
				}

				variant := src
				variant.URL = paramURL
				variant.Weight = paramWeight
				variant.Categories = paramCats
				result = append(result, variant)
			}

			baseURL := appendDefaultQuery(src.URL, src.DefaultQuery)
			if len(src.Categories) > 0 && hasQueryCat {
				if hasAnyCategory(src.Categories, queryCats) {
					variant := src
					variant.URL = baseURL
					result = append(result, variant)
				}
			} else {
				variant := src
				variant.URL = baseURL
				result = append(result, variant)
			}
		} else {
			if len(src.Categories) > 0 && hasQueryCat {
				if !hasAnyCategory(src.Categories, queryCats) {
					continue
				}
			}
			variant := src
			variant.URL = appendDefaultQuery(src.URL, src.DefaultQuery)
			result = append(result, variant)
		}
	}
	return result
}

func appendDefaultQuery(baseURL, defaultQuery string) string {
	defaultQuery = strings.TrimSpace(defaultQuery)
	if defaultQuery == "" {
		return baseURL
	}
	if strings.Contains(baseURL, "?") {
		return baseURL + "&" + defaultQuery
	}
	return baseURL + "?" + defaultQuery
}

func buildURL(baseURL, key, value string) string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" && value == "" {
		return baseURL
	}

	if key == "/" || strings.HasPrefix(value, "/") {
		path := value
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return strings.TrimRight(baseURL, "/") + path
	}

	if key == "" || strings.Contains(key, "=") {
		paramStr := key
		if paramStr == "" {
			paramStr = value
		}
		if strings.Contains(baseURL, "?") {
			return fmt.Sprintf("%s&%s", baseURL, paramStr)
		}
		return fmt.Sprintf("%s?%s", baseURL, paramStr)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		if strings.Contains(baseURL, "?") {
			return fmt.Sprintf("%s&%s=%s", baseURL, url.QueryEscape(key), url.QueryEscape(value))
		}
		return fmt.Sprintf("%s?%s=%s", baseURL, url.QueryEscape(key), url.QueryEscape(value))
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}



func hasAnyCategory(srcCats, queryCats []string) bool {
	for _, qc := range queryCats {
		qc = strings.TrimSpace(qc)
		for _, sc := range srcCats {
			if strings.EqualFold(sc, qc) {
				return true
			}
		}
	}
	return false
}

func weightedPick(sources []model.Source) model.Source {
	totalWeight := 0
	for _, src := range sources {
		totalWeight += src.Weight
	}
	if totalWeight == 0 {
		return sources[rand.Intn(len(sources))]
	}
	r := rand.Intn(totalWeight)
	for _, src := range sources {
		r -= src.Weight
		if r < 0 {
			return src
		}
	}
	return sources[len(sources)-1]
}

func removeSource(sources []model.Source, id int64) []model.Source {
	for i, src := range sources {
		if src.ID == id {
			return append(sources[:i], sources[i+1:]...)
		}
	}
	return sources
}

func extractImageURL(resp *http.Response, src model.Source) (string, error) {
	reqURL := src.URL
	if resp.Request != nil && resp.Request.URL != nil {
		reqURL = resp.Request.URL.String()
	}

	if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 || resp.StatusCode == 308 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			loc = resolveURL(reqURL, loc)
			return resolveBaiduURL(loc), nil
		}
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			text := strings.TrimSpace(string(body))
			if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
				return resolveBaiduURL(text), nil
			}
		}
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "image/") {
		return reqURL, nil
	}

	if strings.HasPrefix(ct, "application/json") || src.RespType == "json" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read body: %w", err)
		}
		path := src.JsonPath
		if path == "" {
			path = "url"
		}
		result := gjson.Get(string(body), path)
		if result.Exists() {
			return resolveURL(reqURL, result.String()), nil
		}
		return "", fmt.Errorf("json path %s not found", path)
	}

	body, err := io.ReadAll(resp.Body)
	if err == nil {
		text := strings.TrimSpace(string(body))
		if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
			return resolveBaiduURL(text), nil
		}
	}

	return reqURL, nil
}

func resolveBaiduURL(rawURL string) string {
	if !strings.Contains(rawURL, "image.baidu.com/search/down") && !strings.Contains(rawURL, "search/down") {
		return rawURL
	}
	if u, err := url.Parse(rawURL); err == nil {
		if realURL := u.Query().Get("url"); realURL != "" &&
			(strings.HasPrefix(realURL, "http://") || strings.HasPrefix(realURL, "https://")) {
			return realURL
		}
	}
	return rawURL
}

func resolveURL(baseURLStr, targetURLStr string) string {
	targetURLStr = strings.TrimSpace(targetURLStr)
	if targetURLStr == "" {
		return baseURLStr
	}
	if strings.HasPrefix(targetURLStr, "http://") || strings.HasPrefix(targetURLStr, "https://") {
		return targetURLStr
	}
	base, err := url.Parse(baseURLStr)
	if err != nil {
		return targetURLStr
	}
	rel, err := url.Parse(targetURLStr)
	if err != nil {
		return targetURLStr
	}
	resolved := base.ResolveReference(rel)
	return resolved.String()
}

func (e *Engine) StartPrecacheWorker() {
	time.Sleep(3 * time.Second)
	e.ReplenishPrecache()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		e.ReplenishPrecache()
	}
}

func (e *Engine) ReplenishPrecache() {
	settings, err := e.store.GetSettings()
	if err != nil || !settings.ProxyMode || settings.PrecacheCount <= 0 || e.precachePool == nil {
		return
	}

	target := settings.PrecacheCount
	current := e.precachePool.Len()

	if current > target {
		e.precachePool.Trim(target)
		return
	}

	if current >= target {
		return
	}

	needed := target - current
	for i := 0; i < needed; i++ {
		res := e.fetchSinglePrecache()
		if res != nil {
			e.precachePool.Push(res)
		} else {
			break
		}
	}
}

func (e *Engine) fetchSinglePrecache() *Result {
	sources, err := e.store.ListSources()
	if err != nil || len(sources) == 0 {
		return nil
	}
	candidates := filterSources(sources, nil)
	if len(candidates) == 0 {
		return nil
	}

	selected := weightedPick(candidates)
	req, err := http.NewRequest("GET", selected.URL, nil)
	if err != nil {
		return nil
	}
	for k, v := range selected.Headers {
		req.Header.Set(k, v)
	}

	settings, err := e.store.GetSettings()
	if err != nil {
		return nil
	}
	e.httpClient.Timeout = time.Duration(settings.Timeout) * time.Millisecond
	resp, err := e.httpClient.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}

	imageURL, err := extractImageURL(resp, selected)
	resp.Body.Close()
	if err != nil {
		return nil
	}

	if e.imageStore != nil {
		srcCats := selected.Categories
		if srcCats == nil {
			srcCats = []string{}
		}
		origURL := imageURL
		cached, cacheErr := e.imageStore.DownloadAndStore(origURL, selected.ID, selected.Name, srcCats)
		if cacheErr != nil {
			return nil
		}
		go e.store.RecordStats(srcCats, selected, origURL, &cached.ID, cached.FileID)

		return &Result{
			URL:        origURL,
			LocalURL:   "/images/" + cached.FileID,
			SourceName: selected.Name,
			SourceID:   selected.ID,
			Categories: selected.Categories,
			FileID:     cached.FileID,
			Width:      cached.Width,
			Height:     cached.Height,
			Format:     cached.Format,
			ImageID:    cached.ID,
		}
	}
	return nil
}

func precacheFileExists(res *Result, imgStore *ImageStore) bool {
	if imgStore == nil || res.FileID == "" {
		return false
	}
	pattern := filepath.Join(imgStore.cacheDir, res.FileID+".*")
	matches, err := filepath.Glob(pattern)
	return err == nil && len(matches) > 0
}

func (e *Engine) recordPrecacheStats(queryCats []string, res *Result) {
	if res == nil || e.store == nil {
		return
	}
	src := model.Source{
		ID:   res.SourceID,
		Name: res.SourceName,
	}
	imgID := res.ImageID
	e.store.RecordStats(queryCats, src, res.URL, &imgID, res.FileID)
}


