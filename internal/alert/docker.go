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

func (a *Docker) Check() (bool, string) {
	h, err := a.d.Health()
	if err != nil {
		return false, ""
	}
	if !h.Running {
		return true, fmt.Sprintf("%q is %s", h.Container, h.Status)
	}
	return false, ""
}
