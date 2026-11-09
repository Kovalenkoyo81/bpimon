package monitor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// commandTimeout bounds every external command. A hung docker CLI,
// smartctl on a dying disk or df on a stale mount must not block the
// alert loop forever.
var commandTimeout = 10 * time.Second

// ErrCommandTimeout is returned when a command exceeds commandTimeout.
var ErrCommandTimeout = errors.New("command timed out")

// output runs a command and returns its stdout, killing it after
// commandTimeout. Errors other than a timeout are returned unchanged,
// so callers can still inspect *exec.ExitError.
func output(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return out, fmt.Errorf("%s: %w after %s", name, ErrCommandTimeout, commandTimeout)
	}
	return out, err
}
