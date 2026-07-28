package service

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"

	"time"

	"github.com/tidwall/gjson"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

type Engine struct {
	store      *store.Store
	proxyCache *ProxyCache
}

func NewEngine(st *store.Store, pc *ProxyCache) *Engine {
	return &Engine{
		store:      st,
		proxyCache: pc,
	}
}

type Result struct {
	URL        string   `json:"url"`
	SourceName string   `json:"source"`
	Categories []string `json:"categories"`
}

func (e *Engine) RandomImage(category string, format string, clientUA string) (*Result, int, error) {
	settings, err := e.store.GetSettings()
	if err != nil {
		return nil, 0, fmt.Errorf("get settings: %w", err)
	}

	sources, err := e.store.ListSources()
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}

	candidates := filterSources(sources, category)
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no available sources")
	}

	timeout := time.Duration(settings.Timeout) * time.Millisecond

	for attempt := 0; attempt < 3 && len(candidates) > 0; attempt++ {
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

		client := &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("source %q fetch failed: %v", selected.Name, err)
			e.store.IncrementFailCount(selected.ID)
			candidates = removeSource(candidates, selected.ID)
			checkAndSuspend(selected.ID, e.store)
			continue
		}

		hasFailed := false

		if resp.StatusCode >= 500 {
			logger.Error("source %q returned %d", selected.Name, resp.StatusCode)
			e.store.IncrementFailCount(selected.ID)
			candidates = removeSource(candidates, selected.ID)
			checkAndSuspend(selected.ID, e.store)
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

		if e.proxyCache != nil {
			settings, _ := e.store.GetSettings()
			if settings != nil && settings.ProxyMode {
				_, _, proxyErr := e.proxyCache.GetOrFetch(imageURL)
				if proxyErr != nil {
					hasFailed = true
				}
			}
		}

		if hasFailed {
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		if format == "json" {
			return &Result{
				URL:        imageURL,
				SourceName: selected.Name,
				Categories: selected.Categories,
			}, http.StatusOK, nil
		}
		return &Result{URL: imageURL}, http.StatusFound, nil
	}

	return nil, 0, fmt.Errorf("all sources failed")
}

func filterSources(sources []model.Source, category string) []model.Source {
	var result []model.Source
	for _, src := range sources {
		if !src.Enabled || src.Status == "error" {
			continue
		}

		if len(src.Params) > 0 {
			for _, param := range src.Params {
				key := strings.TrimSpace(param.Key)
				val := strings.TrimSpace(param.Value)
				if key == "" {
					continue
				}
				paramURL := buildURL(src.URL, key, val)
				paramWeight := param.Weight
				if paramWeight <= 0 {
					paramWeight = src.Weight
				}
				paramCats := param.Categories
				if len(paramCats) == 0 {
					paramCats = src.Categories
				}

				if category != "" {
					cats := strings.Split(category, ",")
					if !hasAnyCategory(paramCats, cats) {
						continue
					}
				}

				variant := src
				variant.URL = paramURL
				variant.Weight = paramWeight
				variant.Categories = paramCats
				result = append(result, variant)
			}
		} else {
			if category != "" {
				cats := strings.Split(category, ",")
				if !hasAnyCategory(src.Categories, cats) {
					continue
				}
			}
			result = append(result, src)
		}
	}
	return result
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

func checkAndSuspend(id int64, st *store.Store) {
	src, err := st.GetSource(id)
	if err != nil {
		return
	}
	if src.FailCount >= 5 {
		st.UpdateSourceStatus(id, "error")
	}
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
			if strings.Contains(loc, "image.baidu.com/search/down") || strings.Contains(loc, "search/down") {
				if u, err := url.Parse(loc); err == nil {
					if realURL := u.Query().Get("url"); realURL != "" && (strings.HasPrefix(realURL, "http://") || strings.HasPrefix(realURL, "https://")) {
						return realURL, nil
					}
				}
			}
			return loc, nil
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

	return reqURL, nil
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

