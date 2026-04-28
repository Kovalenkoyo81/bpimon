package bot

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllows(t *testing.T) {
	r := NewRateLimiter(time.Second, 3)
	for i := 0; i < 3; i++ {
		if !r.Allow(1) {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}
}

func TestRateLimiterBlocks(t *testing.T) {
	r := NewRateLimiter(time.Second, 3)
	for i := 0; i < 3; i++ {
		r.Allow(1)
	}
	if r.Allow(1) {
		t.Fatal("4th call should be blocked")
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	r := NewRateLimiter(50*time.Millisecond, 2)
	r.Allow(1)
	r.Allow(1)
	if r.Allow(1) {
		t.Fatal("3rd call should be blocked within window")
	}
	time.Sleep(60 * time.Millisecond)
	if !r.Allow(1) {
		t.Fatal("should be allowed after window expires")
	}
}

func TestRateLimiterIsolatesUsers(t *testing.T) {
	r := NewRateLimiter(time.Second, 2)
	r.Allow(1)
	r.Allow(1)
	if r.Allow(1) {
		t.Fatal("user 1 should be blocked")
	}
	if !r.Allow(2) {
		t.Fatal("user 2 should not be affected by user 1 limit")
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	r := NewRateLimiter(time.Second, 100)
	var wg sync.WaitGroup
	for i := int64(0); i < 50; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			r.Allow(id)
			r.Allow(id)
		}(i)
	}
	wg.Wait()
}

func TestRateLimiterCleansUpExpiredEntries(t *testing.T) {
	r := NewRateLimiter(20*time.Millisecond, 5)
	r.Allow(42)
	time.Sleep(30 * time.Millisecond)
	r.Allow(42) // old entry expired; new slice allocated fresh
	r.mu.Lock()
	n := len(r.calls[42])
	r.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 entry after window reset, got %d", n)
	}
}
