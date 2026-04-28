package monitor

import (
	"bpimon/internal/log"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewProviders creates the full list of monitoring providers.
// SD cards are auto-discovered and merged with devices.mmc from config.
func NewProviders(smartDevices, mmcDevices, dockerContainers []string) []Provider {
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
			log.Warn.Printf("docker not found — skipping %d container(s)", len(dockerContainers))
		} else {
			seen := make(map[string]string)
			for _, name := range dockerContainers {
				norm := strings.ReplaceAll(name, "-", "_")
				if existing, ok := seen[norm]; ok {
					fmt.Fprintf(os.Stderr, "config error: docker container name collision: %q and %q both normalize to /restart_%s\n", existing, name, norm)
				os.Exit(1)
				}
				seen[norm] = name
				providers = append(providers, Docker{Container: name})
			}
		}
	}

	return providers
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
