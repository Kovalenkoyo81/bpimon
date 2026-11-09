package monitor

import (
	"fmt"
	"os"
	"time"
)

// SystemUptime returns how long the machine has been up.
func SystemUptime() (time.Duration, error) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	var sec float64
	if _, err := fmt.Sscanf(string(b), "%f", &sec); err != nil {
		return 0, fmt.Errorf("parse /proc/uptime: %w", err)
	}
	return time.Duration(sec * float64(time.Second)), nil
}
