package monitor

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrDaemonUnavailable means the docker CLI could not talk to the daemon,
// so the container's real state is unknown.
var ErrDaemonUnavailable = errors.New("docker daemon unavailable")

// StatusRemoved is reported for a container that no longer exists.
const StatusRemoved = "removed"

const DockerAutoDiscover = "bpi-auto-discover"

// DiscoverDockerContainers returns names of all currently running containers.
func DiscoverDockerContainers() []string {
	out, err := output("docker", "ps", "--format", "{{.Names}}")
	if err != nil {
		return nil
	}
	var names []string
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

type DockerHealth struct {
	Container string
	Status    string
	Running   bool
}

type Docker struct {
	Container string
}

func (d Docker) Name() string          { return "Docker " + d.Container }
func (d Docker) ContainerName() string { return d.Container }
func (d Docker) Available() bool       { _, err := exec.LookPath("docker"); return err == nil }

func (d Docker) Health() (DockerHealth, error) {
	out, err := output("docker", "inspect", "--format", "{{.State.Status}}", d.Container)
	if err != nil {
		var stderr string
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if isNoSuchContainer(stderr) {
			return DockerHealth{Container: d.Container, Status: StatusRemoved}, nil
		}
		return DockerHealth{}, fmt.Errorf("%w: docker inspect %s: %v %s", ErrDaemonUnavailable, d.Container, err, stderr)
	}
	status := strings.TrimSpace(string(out))
	return DockerHealth{
		Container: d.Container,
		Status:    status,
		Running:   status == "running",
	}, nil
}

// isNoSuchContainer recognises docker's "container does not exist" errors:
// "Error: no such object: x" (CLI) and "No such container: x" (daemon).
func isNoSuchContainer(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "no such object") || strings.Contains(s, "no such container")
}

func (d Docker) Status() (string, error) {
	h, err := d.Health()
	if err != nil {
		return "", err
	}
	icon := "✅"
	if !h.Running {
		icon = "❌"
	}
	return fmt.Sprintf("🐳 %s — %s %s", h.Container, icon, h.Status), nil
}
