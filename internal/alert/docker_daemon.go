package alert

import "bpimon/internal/monitor"

// DockerDaemon fires while the docker daemon is unreachable.
type DockerDaemon struct {
	d monitor.DockerDaemonReader
}

func NewDockerDaemon(d monitor.DockerDaemonReader) *DockerDaemon {
	return &DockerDaemon{d: d}
}

func (a *DockerDaemon) Name() string { return "Docker daemon" }

// Evaluate fires when the daemon is unreachable: here the error is the
// condition itself, not a reason to report Unknown.
func (a *DockerDaemon) Evaluate() (Verdict, string) {
	if _, err := a.d.Ping(); err != nil {
		return Firing, "unavailable — container states unknown"
	}
	return Clear, ""
}
