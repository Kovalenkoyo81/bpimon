package system

import (
	"errors"
	"testing"
	"time"
)

func TestParseContainerState(t *testing.T) {
	st := parseContainerState("running|2026-11-09T21:33:43.123456789Z\n")
	if st.Status != "running" || st.StartedAt.IsZero() {
		t.Fatalf("unexpected %+v", st)
	}
	st = parseContainerState("exited|garbage")
	if st.Status != "exited" || !st.StartedAt.IsZero() {
		t.Fatalf("unexpected %+v", st)
	}
}

func TestRunTimeout(t *testing.T) {
	start := time.Now()
	err := run(100*time.Millisecond, "sleep", "5")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("command was not killed")
	}
}

func TestRunError(t *testing.T) {
	if err := run(time.Second, "false"); err == nil || errors.Is(err, ErrTimeout) {
		t.Fatalf("expected plain failure, got %v", err)
	}
}
