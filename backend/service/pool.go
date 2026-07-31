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

func (p *DistributionPool) Pop(category string) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return nil
	}

	if category != "" {
		for i, entry := range p.items {
			for _, cat := range entry.Categories {
				if cat == category {
					res := entryToResult(entry)
					p.items = append(p.items[:i], p.items[i+1:]...)
					return res
				}
			}
		}
		return nil
	}

	res := entryToResult(p.items[0])
	p.items = p.items[1:]
	return res
}

func (p *DistributionPool) PopByOrientation(orientation string) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return nil
	}

	for i, entry := range p.items {
		if entry.Orientation == orientation {
			res := entryToResult(entry)
			p.items = append(p.items[:i], p.items[i+1:]...)
			return res
		}
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

func (p *DistributionPool) PopAny() *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return nil
	}

	res := entryToResult(p.items[0])
	p.items = p.items[1:]
	return res
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

func (p *DistributionPool) SourceSnapshot() map[int64]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snap := make(map[int64]int)
	for _, entry := range p.items {
		snap[entry.SourceID]++
	}
	return snap
}

func (p *DistributionPool) OrientationSnapshot() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snap := make(map[string]int)
	for _, entry := range p.items {
		ori := entry.Orientation
		if ori == "" {
			ori = "unknown"
		}
		snap[ori]++
	}
	return snap
}

func (p *DistributionPool) All() []*PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cp := make([]*PoolEntry, len(p.items))
	copy(cp, p.items)
	return cp
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
	}
}
