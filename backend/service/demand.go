package service

import (
	"math"
	"sync"
	"time"
)

type TagDemand struct {
	Requested    int
	Misses       int
	LastAccessAt time.Time
}

type DemandTracker struct {
	mu       sync.Mutex
	window   map[string]*TagDemand
	interval time.Duration
}

func NewDemandTracker() *DemandTracker {
	return &DemandTracker{
		window:   make(map[string]*TagDemand),
		interval: 5 * time.Minute,
	}
}

func (d *DemandTracker) RecordRequest(categories []string, hit bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := "_any"
	if len(categories) > 0 && categories[0] != "" {
		key = categories[0]
	}
	td, ok := d.window[key]
	if !ok {
		td = &TagDemand{}
		d.window[key] = td
	}

	td.Requested++
	if !hit {
		td.Misses++
	}
	td.LastAccessAt = time.Now()
}

func (d *DemandTracker) GetAllocationPlan(poolSize int, currentStock map[string]int) map[string]int {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.expire()

	if poolSize <= 0 {
		return nil
	}

	totalRequested := 0
	for _, td := range d.window {
		totalRequested += td.Requested
	}

	if totalRequested == 0 {
		return nil
	}

	plan := make(map[string]int)
	remaining := poolSize

	for tag, td := range d.window {
		if td.Misses > 0 {
			current := currentStock[tag]
			if current == 0 {
				guarantee := int(math.Ceil(float64(poolSize) * 0.1))
				if guarantee < 1 {
					guarantee = 1
				}
				plan[tag] = guarantee
				remaining -= guarantee
				if remaining <= 0 {
					return plan
				}
			}
		}
	}

	if remaining <= 0 {
		return plan
	}

	totalForProportion := 0
	for _, td := range d.window {
		totalForProportion += td.Requested
	}
	if totalForProportion == 0 {
		return plan
	}

	allocated := 0
	proportional := make(map[string]int)
	for tag, td := range d.window {
		if _, has := plan[tag]; has {
			continue
		}
		alloc := int(math.Round(float64(td.Requested) / float64(totalForProportion) * float64(remaining)))
		if alloc < 0 {
			alloc = 0
		}
		proportional[tag] = alloc
		allocated += alloc
	}

	diff := remaining - allocated
	if diff > 0 {
		for tag := range proportional {
			if diff <= 0 {
				break
			}
			proportional[tag]++
			diff--
		}
		if diff > 0 && len(d.window) > 0 {
			for tag := range d.window {
				if diff <= 0 {
					break
				}
				proportional[tag]++
				diff--
			}
		}
	}

	for tag, alloc := range proportional {
		plan[tag] += alloc
	}

	return plan
}

func (d *DemandTracker) MissedTags() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var tags []string
	for tag, td := range d.window {
		if td.Misses > 0 {
			tags = append(tags, tag)
		}
	}
	return tags
}

func (d *DemandTracker) ResetWindow() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.window = make(map[string]*TagDemand)
}

func (d *DemandTracker) expire() {
	cutoff := time.Now().Add(-d.interval)
	for tag, td := range d.window {
		if td.LastAccessAt.Before(cutoff) {
			delete(d.window, tag)
		}
	}
}
