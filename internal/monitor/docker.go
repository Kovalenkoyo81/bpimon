package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

const DockerAutoDiscover = "bpi-auto-discover"

// DiscoverDockerContainers returns names of all currently running containers.
func DiscoverDockerContainers() []string {
	out, err := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
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
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", d.Container).Output()
	if err != nil {
		return DockerHealth{}, fmt.Errorf("docker inspect %s: %w", d.Container, err)
	}
	status := strings.TrimSpace(string(out))
	return DockerHealth{
		Container: d.Container,
		Status:    status,
		Running:   status == "running",
	}, nil
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
