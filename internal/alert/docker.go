package alert

import (
	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
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

// Headline names the container: "picoclaw-gateway is down" / "is back".
func (a *Docker) Headline(firing bool) string {
	if firing {
		return a.d.ContainerName() + " is down"
	}
	return a.d.ContainerName() + " is back"
}

// Evaluate returns Unknown when the daemon cannot be reached: the container's
// state is not known, and the DockerDaemon alert reports the outage instead.
func (a *Docker) Evaluate() (Verdict, string) {
	h, err := a.d.Health()
	if err != nil {
		return Unknown, err.Error()
	}
	if !h.Running {
		return Firing, "status: " + h.Status
	}
	return Clear, ""
}
