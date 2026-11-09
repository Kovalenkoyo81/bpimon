// Package system performs actions on the host: power management and
// container restarts. Every command is bounded by a timeout.
package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	powerTimeout   = 5 * time.Second
	restartTimeout = 90 * time.Second
	inspectTimeout = 10 * time.Second
)

// ContainerState is a container's state right after an action.
type ContainerState struct {
	Status    string
	StartedAt time.Time // zero if unknown
}

// Actions executes real host commands.
type Actions struct{}

// CanPower reports whether reboot/poweroff are possible (systemctl present).
func (Actions) CanPower() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func (Actions) Reboot() error   { return run(powerTimeout, "systemctl", "reboot") }
func (Actions) Poweroff() error { return run(powerTimeout, "systemctl", "poweroff") }

// RestartContainer runs docker restart; it returns ErrTimeout if it takes
// longer than 90s.
func (Actions) RestartContainer(name string) error {
	return run(restartTimeout, "docker", "restart", name)
}

func (Actions) ContainerState(name string) (ContainerState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), inspectTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Status}}|{{.State.StartedAt}}", name).Output()
	if err != nil {
		return ContainerState{}, fmt.Errorf("docker inspect %s: %w", name, err)
	}
	return parseContainerState(string(out)), nil
}

func parseContainerState(out string) ContainerState {
	status, started, _ := strings.Cut(strings.TrimSpace(out), "|")
	st := ContainerState{Status: status}
	if t, err := time.Parse(time.RFC3339Nano, started); err == nil {
		st.StartedAt = t
	}
	return st
}

// ErrTimeout is returned when an action exceeds its timeout.
var ErrTimeout = fmt.Errorf("timed out")

func run(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := exec.CommandContext(ctx, name, args...).Run()
	if ctx.Err() != nil {
		return fmt.Errorf("%s: %w (%s exceeded)", name, ErrTimeout, timeout)
	}
	return err
}
