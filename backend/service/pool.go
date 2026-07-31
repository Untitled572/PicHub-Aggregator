package service

import (
	"strings"
	"sync"
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
	RoundCount  int
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

func (p *DistributionPool) RemoveBySourceID(sourceID int64) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	kept := p.items[:0]
	removed := 0
	for _, entry := range p.items {
		if entry.SourceID == sourceID {
			removed++
			continue
		}
		kept = append(kept, entry)
	}
	p.items = kept
	return removed
}

func (p *DistributionPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.items)
}

func (p *DistributionPool) Trim(target int, maxRounds int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	keep := make([]*PoolEntry, 0, target)
	for _, entry := range p.items {
		entry.RoundCount++
		if entry.RoundCount >= maxRounds {
			continue
		}
		keep = append(keep, entry)
	}

	for len(keep) > target {
		keep = keep[1:]
	}

	p.items = keep
}

func (p *DistributionPool) CategorySnapshot() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snap := make(map[string]int)
	for _, entry := range p.items {
		for _, cat := range entry.Categories {
			snap[cat]++
		}
	}
	return snap
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
