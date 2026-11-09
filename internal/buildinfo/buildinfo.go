// Package buildinfo describes the running bpimon binary.
package buildinfo

import (
	"fmt"
	"runtime"
	"time"
)

type Info struct {
	Version   string
	BuildDate string
	Started   time.Time
}

func New(version, buildDate string) Info {
	return Info{Version: version, BuildDate: buildDate, Started: time.Now()}
}

// Platform is GOOS/GOARCH of the running binary.
func (Info) Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

// Uptime is how long this process has been running, e.g. "3h 12m".
func (i Info) Uptime() string { return FormatDuration(time.Since(i.Started)) }

// FormatDuration renders d as "2d 3h", "3h 12m", "4m 5s" or "7s".
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
