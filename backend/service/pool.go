package service

import (
	"strings"
	"sync"

	"github.com/pichub/backend/model"
)

func splitCategory(category string) []string {
	if category == "" {
		return nil
	}
	parts := strings.Split(category, ",")
	var result []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

type PoolEntry struct {
	FileID      string
	SourceName  string
	SourceID    int64
	Categories  []string
	Orientation string
	Width       int
	Height      int
	Format      string
	ImageID     int64
}

type DistributionPool struct {
	mu    sync.RWMutex
	items []*PoolEntry
}

func NewDistributionPool() *DistributionPool {
	return &DistributionPool{
		items: make([]*PoolEntry, 0),
	}
}

func (p *DistributionPool) PopByOrientation(orientation string, exclusive map[string]bool) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return nil
	}

	for i, entry := range p.items {
		if entry.Orientation != orientation || entryHasExclusive(entry, exclusive) {
			continue
		}
		res := entryToResult(entry)
		p.items = append(p.items[:i], p.items[i+1:]...)
		return res
	}
	return nil
}

func (p *DistributionPool) PopMatching(matchFn func(entry *PoolEntry) bool) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, entry := range p.items {
		if matchFn(entry) {
			res := entryToResult(entry)
			p.items = append(p.items[:i], p.items[i+1:]...)
			return res
		}
	}
	return nil
}

func (p *DistributionPool) PopAny(exclusive map[string]bool) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, entry := range p.items {
		if entryHasExclusive(entry, exclusive) {
			continue
		}
		res := entryToResult(entry)
		p.items = append(p.items[:i], p.items[i+1:]...)
		return res
	}
	return nil
}

// entryHasExclusive 判断池条目是否携带 exclusive 标签 (未显式点名的请求不得消费)
func entryHasExclusive(entry *PoolEntry, exclusive map[string]bool) bool {
	if len(exclusive) == 0 {
		return false
	}
	for _, cat := range entry.Categories {
		if exclusive[cat] {
			return true
		}
	}
	return false
}

func (p *DistributionPool) Push(entry *PoolEntry) {
	if entry == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = append(p.items, entry)
}

func (p *DistributionPool) Remove(fileID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, entry := range p.items {
		if entry.FileID == fileID {
			p.items = append(p.items[:i], p.items[i+1:]...)
			return
		}
	}
}

func (p *DistributionPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.items)
}

func (p *DistributionPool) snapshot() []PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	entries := make([]PoolEntry, 0, len(p.items))
	for _, entry := range p.items {
		entries = append(entries, *entry)
	}
	return entries
}

func (p *DistributionPool) SourceCounts() map[int64]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	counts := make(map[int64]int)
	for _, entry := range p.items {
		counts[entry.SourceID]++
	}
	return counts
}

func matchesReserve(entry *PoolEntry, tag string, tags []model.Tag) bool {
	switch tag {
	case "horizontal", "vertical", "square":
		if entry.Orientation != tag {
			return false
		}
	}
	return hasCategory(entry.Categories, tag) && matchCategories(entry.Categories, []string{tag}, tags, true, isExclusiveTag(tag, tags))
}

func (p *DistributionPool) categoryStock(tags []model.Tag) map[string]int {
	stock := make(map[string]int)
	for _, entry := range p.items {
		for _, cat := range entry.Categories {
			if matchesReserve(entry, cat, tags) {
				stock[cat]++
			}
		}
	}
	return stock
}

func (p *DistributionPool) CategorySnapshot(tags []model.Tag) map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.categoryStock(tags)
}

func entryToResult(entry *PoolEntry) *Result {
	return &Result{
		URL:         "",
		LocalURL:    "/images/" + entry.FileID,
		SourceName:  entry.SourceName,
		SourceID:    entry.SourceID,
		Categories:  entry.Categories,
		Orientation: entry.Orientation,
		FileID:      entry.FileID,
		Width:       entry.Width,
		Height:      entry.Height,
		Format:      entry.Format,
		ImageID:     entry.ImageID,
	}
}
