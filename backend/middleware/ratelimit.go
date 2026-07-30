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
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-5 * time.Minute)
			for ip, times := range rl.visits {
				var active []time.Time
				for _, t := range times {
					if t.After(cutoff) {
						active = append(active, t)
					}
				}
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
		settings, _ := rl.store.GetSettings()
		limit := 60
		windowSecs := 60
		if settings != nil {
			if settings.RateLimit > 0 {
				limit = settings.RateLimit
			}
			if settings.RateLimitWindow > 0 {
				windowSecs = settings.RateLimitWindow
			}
		}

		ip := c.ClientIP()
		rl.mu.Lock()
		now := time.Now()
		window := now.Add(-time.Duration(windowSecs) * time.Second)
		var recent []time.Time
		for _, t := range rl.visits[ip] {
			if t.After(window) {
				recent = append(recent, t)
			}
		}
		rl.visits[ip] = append(recent, now)
		rl.mu.Unlock()
		if len(recent) >= limit {
			logger.Error("rate limit exceeded: %s (%d/%d)", ip, len(recent)+1, limit)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
