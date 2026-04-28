package bot

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	maxCalls int
	calls    map[int64][]time.Time
}

func NewRateLimiter(window time.Duration, maxCalls int) *RateLimiter {
	return &RateLimiter{
		window:   window,
		maxCalls: maxCalls,
		calls:    make(map[int64][]time.Time),
	}
}

// Allow returns true if the user is within the rate limit, false if exceeded.
func (r *RateLimiter) Allow(userID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	var recent []time.Time
	for _, t := range r.calls[userID] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= r.maxCalls {
		r.calls[userID] = recent
		return false
	}

	recent = append(recent, now)
	r.calls[userID] = recent
	return true
}
