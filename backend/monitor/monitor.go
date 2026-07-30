package monitor

import (
	"sync"
	"time"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/store"
)

type SourceMonitor struct {
	store     *store.Store
	mu        sync.Mutex
	windows   map[int64][]bool
	maxWindow int
	threshold float64
}

func NewSourceMonitor(st *store.Store) *SourceMonitor {
	sm := &SourceMonitor{
		store:     st,
		windows:   make(map[int64][]bool),
		maxWindow: 20,
		threshold: 0.3,
	}
	go sm.recoveryWorker()
	return sm
}

func (sm *SourceMonitor) Emit(sourceID int64, success bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.windows[sourceID] = append(sm.windows[sourceID], success)

	if len(sm.windows[sourceID]) > sm.maxWindow {
		n := len(sm.windows[sourceID])
		sm.windows[sourceID] = sm.windows[sourceID][n-sm.maxWindow:]
	}

	if len(sm.windows[sourceID]) < sm.maxWindow {
		return
	}

	successes := 0
	for _, ok := range sm.windows[sourceID] {
		if ok {
			successes++
		}
	}
	rate := float64(successes) / float64(len(sm.windows[sourceID]))

	if rate < sm.threshold {
		logger.System("source %d disabled by monitor (success rate %.0f%%)", sourceID, rate*100)
		sm.store.UpdateSourceStatus(sourceID, "error")
		delete(sm.windows, sourceID)
	}
}

func (sm *SourceMonitor) recoveryWorker() {
	for {
		time.Sleep(30 * time.Minute)
		sm.store.ResetAllSourceErrors()
	}
}
