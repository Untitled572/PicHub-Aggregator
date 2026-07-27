package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
		ip := c.ClientIP()
		rl.mu.Lock()
		now := time.Now()
		window := now.Add(-1 * time.Minute)
		var recent []time.Time
		for _, t := range rl.visits[ip] {
			if t.After(window) {
				recent = append(recent, t)
			}
		}
		rl.visits[ip] = append(recent, now)
		limit := 60
		if st := rl.store; st != nil {
			if s, err := st.GetSettings(); err == nil && s.RateLimit > 0 {
				limit = s.RateLimit
			}
		}
		rl.mu.Unlock()
		if len(recent) >= limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
