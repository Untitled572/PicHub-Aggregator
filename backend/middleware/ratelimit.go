package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/store"
)

type RateLimitMiddleware struct {
	store  *store.Store
	visits map[string][]time.Time
	mu     sync.Mutex
}

func RateLimit(st *store.Store) gin.HandlerFunc {
	rl := &RateLimitMiddleware{
		store:  st,
		visits: make(map[string][]time.Time),
	}

	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			_, window := rl.config()
			cutoff := time.Now().Add(-window)
			rl.mu.Lock()
			for ip, times := range rl.visits {
				active := recentVisits(times, cutoff)
				if len(active) == 0 {
					delete(rl.visits, ip)
				} else {
					rl.visits[ip] = active
				}
			}
			rl.mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		limit, window := rl.config()

		ip := c.ClientIP()
		count, allowed := rl.allow(ip, time.Now(), limit, window)
		if !allowed {
			logger.Error("rate limit exceeded: %s (%d/%d)", ip, count+1, limit)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func (rl *RateLimitMiddleware) config() (int, time.Duration) {
	limit := 60
	window := time.Minute
	settings, _ := rl.store.GetSettings()
	if settings != nil {
		if settings.RateLimit > 0 {
			limit = settings.RateLimit
		}
		if settings.RateLimitWindow > 0 {
			window = time.Duration(settings.RateLimitWindow) * time.Second
		}
	}
	return limit, window
}

func (rl *RateLimitMiddleware) allow(ip string, now time.Time, limit int, window time.Duration) (int, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	recent := recentVisits(rl.visits[ip], now.Add(-window))
	rl.visits[ip] = recent
	if len(recent) >= limit {
		return len(recent), false
	}
	rl.visits[ip] = append(recent, now)
	return len(recent), true
}

func recentVisits(visits []time.Time, cutoff time.Time) []time.Time {
	active := make([]time.Time, 0, len(visits))
	for _, visit := range visits {
		if visit.After(cutoff) {
			active = append(active, visit)
		}
	}
	return active
}
