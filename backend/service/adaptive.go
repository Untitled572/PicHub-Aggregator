package service

import (
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"time"

	"github.com/pichub/backend/model"
)

func hasDirectionTag(cats []string) bool {
	return hasCategory(cats, "horizontal") || hasCategory(cats, "vertical")
}

// Adaptive is a request policy, rather than a required source category.
func filterRequestSources(sources []model.Source, cats []string, tags []model.Tag, orientation string) []model.Source {
	selectionCats := cats
	if hasCategory(cats, "adaptive") {
		selectionCats = nil
		for _, cat := range cats {
			if cat != "adaptive" {
				selectionCats = append(selectionCats, cat)
			}
		}
	}
	candidates := filterSources(sources, selectionCats, tags)
	if hasCategory(cats, "adaptive") {
		allowed := candidates[:0]
		for _, src := range candidates {
			if matchRequestCategories(src.Categories, cats, tags, len(cats) == 1, hasExclusiveTag(tags, cats)) {
				allowed = append(allowed, src)
			}
		}
		candidates = allowed
	}
	if !hasCategory(cats, "adaptive") || orientation == "" {
		return candidates
	}
	native := make(map[int64]bool)
	directional := make(map[int64]bool)
	for _, src := range candidates {
		if hasCategory(src.Categories, "adaptive") {
			native[src.ID] = true
		}
		if hasCategory(src.Categories, orientation) {
			directional[src.ID] = true
		}
	}
	kept := candidates[:0]
	for _, src := range candidates {
		if native[src.ID] && !hasCategory(src.Categories, "adaptive") {
			continue
		}
		if directional[src.ID] && !hasCategory(src.Categories, "adaptive") && !hasCategory(src.Categories, orientation) {
			continue
		}
		if hasCategory(src.Categories, "adaptive") || !hasDirectionTag(src.Categories) || hasCategory(src.Categories, orientation) {
			kept = append(kept, src)
		}
	}
	return kept
}

func requestOrientation(src model.Source, cats []string, orientation string, explicit bool) string {
	if !explicit && hasCategory(cats, "adaptive") && hasCategory(src.Categories, "adaptive") {
		return "" // The source handles the forwarded UA itself.
	}
	return orientation
}

// Non-proxy requests still need size validation for sources without direction tags.
func checkRemoteOrientation(src model.Source, target, orientation, ua string, timeout time.Duration, client *http.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header = buildImageRequestHeaders(src.URL, src.Headers)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	imageClient := *client
	imageClient.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many image redirects")
		}
		return nil
	}
	resp, err := imageClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("image HTTP %d", resp.StatusCode)
	}
	cfg, _, err := image.DecodeConfig(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if GetOrientation(cfg.Width, cfg.Height) != orientation {
		return fmt.Errorf("image orientation does not match %s", orientation)
	}
	return nil
}

func matchRequestCategories(srcCats, queryCats []string, tags []model.Tag, single, hasExcl bool) bool {
	if hasCategory(queryCats, "adaptive") {
		var rest []string
		for _, cat := range queryCats {
			if cat != "adaptive" {
				rest = append(rest, cat)
			}
		}
		if len(rest) > 0 {
			return matchCategories(srcCats, rest, tags, len(rest) == 1, hasExclusiveTag(tags, rest))
		}
		for _, cat := range srcCats {
			if isExclusiveTag(cat, tags) {
				return false
			}
		}
		return true
	}
	return matchCategories(srcCats, queryCats, tags, single, hasExcl)
}
