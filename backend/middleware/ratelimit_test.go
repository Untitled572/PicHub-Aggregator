package middleware

import (
	"testing"
	"time"
)

func TestRateLimitRejectedRequestsDoNotGrowVisitQueue(t *testing.T) {
	rl := &RateLimitMiddleware{visits: make(map[string][]time.Time)}
	start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	window := time.Minute

	if _, allowed := rl.allow("client", start, 2, window); !allowed {
		t.Fatal("first request should be allowed")
	}
	if _, allowed := rl.allow("client", start.Add(time.Second), 2, window); !allowed {
		t.Fatal("second request should be allowed")
	}
	for i := 0; i < 100; i++ {
		if _, allowed := rl.allow("client", start.Add(2*time.Second), 2, window); allowed {
			t.Fatalf("request %d should be rejected", i+1)
		}
	}

	if got := len(rl.visits["client"]); got != 2 {
		t.Fatalf("rejected requests grew the queue: got %d visits, want 2", got)
	}
	if _, allowed := rl.allow("client", start.Add(window+time.Second), 2, window); !allowed {
		t.Fatal("request should be allowed after the configured window expires")
	}
}

func TestRateLimitUsesCurrentWindowForPruning(t *testing.T) {
	rl := &RateLimitMiddleware{visits: make(map[string][]time.Time)}
	start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

	if _, allowed := rl.allow("client", start, 1, 10*time.Second); !allowed {
		t.Fatal("initial request should be allowed")
	}
	if count, allowed := rl.allow("client", start.Add(2*time.Second), 1, 10*time.Second); allowed || count != 1 {
		t.Fatalf("request within 10-second window = (%d, %v), want (1, false)", count, allowed)
	}
	if count, allowed := rl.allow("client", start.Add(2*time.Second), 1, time.Second); !allowed || count != 0 {
		t.Fatalf("request after window changed to 1 second = (%d, %v), want (0, true)", count, allowed)
	}
	if got := len(rl.visits["client"]); got != 1 {
		t.Fatalf("window change left %d visits, want only the current request", got)
	}
}
