package monitor

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestOutputTimesOut(t *testing.T) {
	old := commandTimeout
	commandTimeout = 100 * time.Millisecond
	defer func() { commandTimeout = old }()

	start := time.Now()
	_, err := output("sleep", "5")
	if !errors.Is(err, ErrCommandTimeout) {
		t.Fatalf("expected ErrCommandTimeout, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("command was not killed on timeout (took %s)", time.Since(start))
	}
}

func TestOutputKeepsExitError(t *testing.T) {
	_, err := output("sh", "-c", "echo oops >&2; exit 3")
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 3 || string(ee.Stderr) != "oops\n" {
		t.Fatalf("expected ExitError with code 3 and stderr, got %v", err)
	}
}

func TestOutputSuccess(t *testing.T) {
	out, err := output("echo", "hi")
	if err != nil || string(out) != "hi\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}
