package alert

import (
	"bpimon/internal/monitor"
	"fmt"
)

type Docker struct {
	d monitor.DockerReader
}

func NewDocker(d monitor.DockerReader) *Docker {
	return &Docker{d: d}
}

func (a *Docker) Name() string { return "Docker " + a.d.ContainerName() }

// Choices: a stopped container may have been stopped on purpose, so ask.
func (a *Docker) Choices() (intentional, incident string) {
	return "✋ Stopped by admin", "💥 Crashed"
}

// Evaluate returns Unknown when the daemon cannot be reached: the container's
// state is not known, and the DockerDaemon alert reports the outage instead.
func (a *Docker) Evaluate() (Verdict, string) {
	h, err := a.d.Health()
	if err != nil {
		return Unknown, err.Error()
	}
	if !h.Running {
		return Firing, fmt.Sprintf("%q is %s", h.Container, h.Status)
	}
	return Clear, ""
}
