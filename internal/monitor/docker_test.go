package monitor

import "testing"

func TestIsNoSuchContainer(t *testing.T) {
	yes := []string{
		"Error: no such object: app",
		"Error: No such object: app",
		"Error response from daemon: No such container: app",
	}
	no := []string{
		"",
		"Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
		"permission denied while trying to connect to the Docker daemon socket",
	}
	for _, s := range yes {
		if !isNoSuchContainer(s) {
			t.Errorf("expected not-found for %q", s)
		}
	}
	for _, s := range no {
		if isNoSuchContainer(s) {
			t.Errorf("unexpected not-found for %q", s)
		}
	}
}
