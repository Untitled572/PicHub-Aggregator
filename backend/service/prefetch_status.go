package service

import (
	"sort"
	"time"
)

type PrefetchStock struct {
	Count      int `json:"count"`
	Horizontal int `json:"horizontal"`
	Vertical   int `json:"vertical"`
	Square     int `json:"square"`
}

func (s *PrefetchStock) add(orientation string) {
	s.Count++
	switch orientation {
	case "horizontal":
		s.Horizontal++
	case "vertical":
		s.Vertical++
	case "square":
		s.Square++
	}
}

type PrefetchTagStock struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Target int    `json:"target"`
	PrefetchStock
}

type PrefetchSourceStock struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	PrefetchStock
}

type PrefetchRefill struct {
	ImageID    int64     `json:"image_id"`
	SourceName string    `json:"source_name"`
	AddedAt    time.Time `json:"added_at"`
}

type PrefetchStatus struct {
	Enabled    bool            `json:"enabled"`
	Paused     bool            `json:"paused"`
	Target     int             `json:"target"`
	CheckedAt  time.Time       `json:"checked_at"`
	LastRefill *PrefetchRefill `json:"last_refill,omitempty"`
	PrefetchStock
	ByTag    []PrefetchTagStock    `json:"by_tag"`
	BySource []PrefetchSourceStock `json:"by_source"`
}

func (e *Engine) PrefetchStatus() (PrefetchStatus, error) {
	settings, err := e.store.GetSettings()
	if err != nil {
		return PrefetchStatus{}, err
	}
	debug := e.DebugSource()
	lastRefill := e.lastPrefetch.Load()
	status := PrefetchStatus{
		Enabled: settings.ProxyMode && settings.PoolSize > 0 && e.distPool != nil,
		Paused:  debug.Enabled || debug.Switching,
		Target:  settings.PoolSize, CheckedAt: time.Now(), LastRefill: lastRefill,
		ByTag: []PrefetchTagStock{}, BySource: []PrefetchSourceStock{},
	}
	if e.distPool == nil {
		return status, nil
	}
	entries := e.distPool.snapshot()
	live, err := e.store.GetPooledFileIDSet()
	if err != nil {
		return status, err
	}
	tags, err := e.store.GetTags()
	if err != nil {
		return status, err
	}
	sources, err := e.store.ListSources()
	if err != nil {
		return status, err
	}
	names := make(map[int64]string, len(sources))
	for _, src := range sources {
		names[src.ID] = src.Name
	}
	eligible := make(map[string]bool, len(tags))
	for _, tag := range tags {
		eligible[tag.ID] = len(filterSources(sources, []string{tag.ID}, tags)) > 0
	}
	eligibleCount := 0
	for _, ok := range eligible {
		if ok {
			eligibleCount++
		}
	}
	reserve := 0
	if eligibleCount > 0 {
		reserve = min(prefetchTagReserve, status.Target/eligibleCount)
	}
	for _, tag := range tags {
		row := PrefetchTagStock{ID: tag.ID, Name: tag.Name}
		if eligible[tag.ID] {
			row.Target = reserve
		}
		status.ByTag = append(status.ByTag, row)
	}
	bySource := make(map[int64]*PrefetchSourceStock)
	for _, entry := range entries {
		if !live[entry.FileID] {
			continue
		}
		status.add(entry.Orientation)
		row := bySource[entry.SourceID]
		if row == nil {
			name := names[entry.SourceID]
			if name == "" {
				name = entry.SourceName
			}
			row = &PrefetchSourceStock{ID: entry.SourceID, Name: name}
			bySource[entry.SourceID] = row
		}
		row.add(entry.Orientation)
		for i := range status.ByTag {
			if matchesReserve(&entry, status.ByTag[i].ID, tags) {
				status.ByTag[i].add(entry.Orientation)
			}
		}
	}
	for _, row := range bySource {
		status.BySource = append(status.BySource, *row)
	}
	sort.Slice(status.BySource, func(i, j int) bool {
		a, b := status.BySource[i], status.BySource[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	return status, nil
}
