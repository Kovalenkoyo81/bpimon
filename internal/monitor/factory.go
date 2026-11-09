package monitor

import (
	"bpimon/internal/log"
	"fmt"
	"path/filepath"
	"strings"
)

// NewProviders creates the full list of monitoring providers.
// SD cards are auto-discovered and merged with devices.mmc from config.
// It fails if two container names map to the same /restart_ command.
func NewProviders(smartDevices, mmcDevices, dockerContainers []string) ([]Provider, error) {
	providers := []Provider{NewCPU(), Memory{}, Disk{}}

	if len(smartDevices) > 0 {
		if !(Smart{}).Available() {
			log.Warn.Printf("smartctl not found — skipping %d SMART device(s)", len(smartDevices))
		} else {
			for _, dev := range smartDevices {
				providers = append(providers, Smart{Device: dev})
			}
		}
	}

	for _, dev := range mergeSDDevices(mmcDevices) {
		providers = append(providers, SD{Device: dev})
	}

	if len(dockerContainers) > 0 {
		if !(Docker{}).Available() {
			log.Warn.Printf("docker not found — skipping Docker monitoring")
		} else {
			providers = append(providers, DockerDaemon{})
			containers := dockerContainers
			if len(containers) == 1 && containers[0] == DockerAutoDiscover {
				containers = DiscoverDockerContainers()
				if len(containers) == 0 {
					log.Warn.Printf("docker auto-discover: no running containers found")
				} else {
					log.Info.Printf("docker auto-discover: found %d container(s): %s", len(containers), strings.Join(containers, ", "))
				}
			}
			seen := make(map[string]string)
			for _, name := range containers {
				norm := strings.ReplaceAll(name, "-", "_")
				if existing, ok := seen[norm]; ok {
					return nil, fmt.Errorf("docker container name collision: %q and %q both normalize to /restart_%s", existing, name, norm)
				}
				seen[norm] = name
				providers = append(providers, Docker{Container: name})
			}
		}
	}

	return providers, nil
}

// mergeSDDevices merges auto-discovered SD cards with those in config,
// deduplicating by base device name.
func mergeSDDevices(fromConfig []string) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(dev string) {
		base := filepath.Base(dev)
		if !seen[base] {
			seen[base] = true
			result = append(result, dev)
		}
	}

	for _, dev := range DiscoverSDCards() {
		add(dev)
	}
	for _, dev := range fromConfig {
		add(dev)
	}

	return result
}
