package service

import (
	"strings"
	"sync"
	"time"

	"github.com/pichub/backend/logger"
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

type PrecacheEntry struct {
	Result    *Result
	FetchedAt time.Time
}

type PrecachePool struct {
	mu    sync.Mutex
	items []*PrecacheEntry
}

func NewPrecachePool() *PrecachePool {
	return &PrecachePool{
		items: make([]*PrecacheEntry, 0),
	}
}

func (p *PrecachePool) Pop(category string) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return nil
	}

	queryCats := splitCategory(category)

	if len(queryCats) > 0 {
		for i, entry := range p.items {
			if hasAnyCategory(entry.Result.Categories, queryCats) {
				res := entry.Result
				p.items = append(p.items[:i], p.items[i+1:]...)
				return res
			}
		}
		return nil
	}

	res := p.items[0].Result
	p.items = p.items[1:]
	return res
}

func (p *PrecachePool) Discard(res *Result) {
	if res == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, entry := range p.items {
		if entry.Result == res {
			p.items = append(p.items[:i], p.items[i+1:]...)
			return
		}
	}
}

func (p *PrecachePool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.items)
}

func (p *PrecachePool) Push(res *Result) {
	if res == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = append(p.items, &PrecacheEntry{
		Result:    res,
		FetchedAt: time.Now(),
	})
	logger.System("precache pool updated, current count: %d", len(p.items))
}

func (p *PrecachePool) Trim(max int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.items) > max {
		p.items = p.items[1:]
	}
}
