package service

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/pichub/backend/model"
	"github.com/pichub/backend/store"
)

type Engine struct {
	store   *store.Store
	client  *http.Client
}

func NewEngine(st *store.Store) *Engine {
	return &Engine{
		store: st,
		client: &http.Client{
			Timeout:   10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type Result struct {
	URL        string   `json:"url"`
	SourceName string   `json:"source"`
	Categories []string `json:"categories"`
}

func (e *Engine) RandomImage(category string, format string) (*Result, int, error) {
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

		client := &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			e.store.IncrementFailCount(selected.ID)
			candidates = removeSource(candidates, selected.ID)
			checkAndSuspend(selected.ID, e.store)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			e.store.IncrementFailCount(selected.ID)
			candidates = removeSource(candidates, selected.ID)
			checkAndSuspend(selected.ID, e.store)
			continue
		}

		e.store.ResetFailCount(selected.ID)

		imageURL, err := extractImageURL(resp, selected)
		if err != nil {
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		e.store.UpdateSourceStatus(selected.ID, "normal")

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
		if !src.Enabled {
			continue
		}
		if src.Status == "error" {
			continue
		}
		if category != "" {
			cats := strings.Split(category, ",")
			if !hasAnyCategory(src.Categories, cats) {
				continue
			}
		}
		result = append(result, src)
	}
	return result
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
	if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 || resp.StatusCode == 308 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			return loc, nil
		}
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "image/") {
		return src.URL, nil
	}

	if strings.Contains(ct, "json") || src.RespType == "json" {
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
			return result.String(), nil
		}
		return "", fmt.Errorf("json path %s not found", path)
	}

	return src.URL, nil
}
