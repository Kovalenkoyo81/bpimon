package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var mmcblkRoot = regexp.MustCompile(`^mmcblk\d+$`)

type SDHealth struct {
	Device     string
	Name       string // model name from sysfs
	ErrorCount int
	LastError  string
}

type SD struct {
	Device string // e.g., "mmcblk0" or "/dev/mmcblk0"
}

func (s SD) Name() string { return "SD " + filepath.Base(s.Device) }

func (s SD) Health() (SDHealth, error) {
	base := filepath.Base(s.Device)
	r, err := readKernelErrors(base)
	if err != nil {
		return SDHealth{}, err
	}
	return SDHealth{
		Device:     s.Device,
		Name:       sdCardName(base),
		ErrorCount: r.errorCount,
		LastError:  r.lastError,
	}, nil
}

func (s SD) Status() (string, error) {
	h, err := s.Health()
	if err != nil {
		return "", err
	}
	out := fmt.Sprintf("💳 SD %s [%s]\nErrors: %d", filepath.Base(h.Device), h.Name, h.ErrorCount)
	if h.LastError != "" {
		out += "\nLast: " + h.LastError
	}
	return out, nil
}

// DiscoverSDCards returns all SD cards found in sysfs (MMC_TYPE=SD, no partitions).
func DiscoverSDCards() []string {
	entries, err := filepath.Glob("/sys/class/block/mmcblk*")
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		base := filepath.Base(entry)
		// Accept only root devices (mmcblk0, mmcblk1, ...).
		// Excludes partitions (mmcblk0p1), eMMC boot partitions (mmcblk0boot0),
		// and replay-protected regions (mmcblk0rpmb).
		if !mmcblkRoot.MatchString(base) {
			continue
		}
		found = append(found, base)
	}
	return found
}

// sdCardName returns the card model name from sysfs MMC_NAME field.
func sdCardName(blockDev string) string {
	name := readUeventField(blockDev, "MMC_NAME")
	if name == "" {
		return "Unknown"
	}
	return name
}

// readUeventField reads a key=value field from the device uevent file.
func readUeventField(blockDev, key string) string {
	data, err := os.ReadFile("/sys/block/" + blockDev + "/device/uevent")
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}
