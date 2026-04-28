package bot

import (
	"sync"
	"testing"
	"time"
)

func TestConfirmatorValidCode(t *testing.T) {
	c := NewConfirmator(time.Minute)
	code := c.Request(1, "reboot", 6)
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %q", code)
	}
	action, ok := c.Validate(1, code)
	if !ok || action != "reboot" {
		t.Fatalf("expected valid confirmation, got ok=%v action=%q", ok, action)
	}
}

func TestConfirmatorWrongCode(t *testing.T) {
	c := NewConfirmator(time.Minute)
	c.Request(1, "reboot", 6)
	_, ok := c.Validate(1, "000000")
	if ok {
		t.Fatal("wrong code should not validate")
	}
}

func TestConfirmatorWrongUser(t *testing.T) {
	c := NewConfirmator(time.Minute)
	code := c.Request(1, "reboot", 6)
	_, ok := c.Validate(2, code)
	if ok {
		t.Fatal("different user should not validate")
	}
}

func TestConfirmatorExpired(t *testing.T) {
	c := NewConfirmator(5 * time.Millisecond)
	code := c.Request(1, "reboot", 6)
	time.Sleep(10 * time.Millisecond)
	_, ok := c.Validate(1, code)
	if ok {
		t.Fatal("expired code should not validate")
	}
}

func TestConfirmatorConsumedOnce(t *testing.T) {
	c := NewConfirmator(time.Minute)
	code := c.Request(1, "reboot", 6)
	if _, ok := c.Validate(1, code); !ok {
		t.Fatal("first validation should succeed")
	}
	if _, ok := c.Validate(1, code); ok {
		t.Fatal("code should be consumed after first use")
	}
}

func TestConfirmatorDigitLength(t *testing.T) {
	c := NewConfirmator(time.Minute)
	for _, digits := range []int{4, 6, 8} {
		code := c.Request(1, "action", digits)
		if len(code) != digits {
			t.Errorf("expected %d digits, got %q (len=%d)", digits, code, len(code))
		}
	}
}

func TestConfirmatorConcurrent(t *testing.T) {
	c := NewConfirmator(time.Minute)
	var wg sync.WaitGroup
	for i := int64(0); i < 50; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			code := c.Request(id, "action", 4)
			c.Validate(id, code)
		}(i)
	}
	wg.Wait()
}

func TestConfirmatorSweepsExpiredEntries(t *testing.T) {
	c := NewConfirmator(10 * time.Millisecond)
	c.Request(1, "a", 4)
	c.Request(2, "b", 4)
	c.Request(3, "c", 4)
	time.Sleep(20 * time.Millisecond)
	c.Validate(1, "wrong") // triggers sweep
	c.mu.Lock()
	n := len(c.pending)
	c.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected 0 pending entries after sweep, got %d", n)
	}
}
