package monitor

import (
	"sync"
	"testing"
	"time"
)

// fakeTimes returns successive snapshots; each call advances by 100 ticks
// with the given busy share.
type fakeTimes struct {
	mu   sync.Mutex
	cur  cpuTimes
	busy uint64 // busy ticks per 100
}

func (f *fakeTimes) read() (cpuTimes, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cur.total += 100
	f.cur.idle += 100 - f.busy
	return f.cur, nil
}

func newFakeCPU(busy uint64) (*CPU, *fakeTimes) {
	f := &fakeTimes{busy: busy}
	return &CPU{read: f.read, statusWindow: time.Millisecond}, f
}

func TestUsageBetween(t *testing.T) {
	cases := []struct {
		prev, cur cpuTimes
		want      float64
	}{
		{cpuTimes{idle: 0, total: 0}, cpuTimes{idle: 75, total: 100}, 25},
		{cpuTimes{idle: 50, total: 100}, cpuTimes{idle: 50, total: 200}, 100},
		{cpuTimes{idle: 10, total: 100}, cpuTimes{idle: 10, total: 100}, 0}, // no time passed
		{cpuTimes{idle: 10, total: 100}, cpuTimes{idle: 5, total: 50}, 0},   // counter reset
	}
	for _, c := range cases {
		if got := usageBetween(c.prev, c.cur); got != c.want {
			t.Errorf("usageBetween(%+v, %+v) = %v, want %v", c.prev, c.cur, got, c.want)
		}
	}
}

func TestStatusDoesNotShiftAlertBaseline(t *testing.T) {
	c, f := newFakeCPU(20)
	if _, err := c.Usage(); err != nil { // baseline
		t.Fatal(err)
	}
	baseline := c.prev

	f.mu.Lock()
	f.busy = 90
	f.mu.Unlock()
	if _, err := c.Status(); err != nil {
		t.Fatal(err)
	}
	if c.prev != baseline {
		t.Fatalf("Status changed the alert baseline: %+v → %+v", baseline, c.prev)
	}
}

func TestCPUConcurrentUse(t *testing.T) {
	c, _ := newFakeCPU(50)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = c.Usage() }()
		go func() { defer wg.Done(); _, _ = c.Status() }()
	}
	wg.Wait()
}

func TestReadCPUTimesFromProc(t *testing.T) {
	ct, err := readCPUTimes()
	if err != nil {
		t.Skipf("/proc/stat unavailable: %v", err)
	}
	if ct.total == 0 || ct.idle > ct.total {
		t.Fatalf("implausible snapshot %+v", ct)
	}
}
