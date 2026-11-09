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

func (a *DockerDaemon) Check() (bool, string) {
	if _, err := a.d.Ping(); err != nil {
		return true, "unavailable — container states unknown"
	}
	return false, ""
}
