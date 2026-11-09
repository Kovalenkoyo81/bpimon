package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

// DockerDaemon reports whether the docker daemon is reachable.
type DockerDaemon struct{}

func (DockerDaemon) Name() string    { return "Docker daemon" }
func (DockerDaemon) Available() bool { _, err := exec.LookPath("docker"); return err == nil }

// Ping returns the server version, or an error if the daemon is unreachable.
func (DockerDaemon) Ping() (string, error) {
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return "", fmt.Errorf("docker version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (d DockerDaemon) Status() (string, error) {
	v, err := d.Ping()
	if err != nil {
		return "🐳 Docker daemon — ❌ unavailable", nil
	}
	return "🐳 Docker daemon — ✅ " + v, nil
}
