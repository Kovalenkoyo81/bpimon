// Package buildinfo describes the running bpimon binary.
package buildinfo

import (
	"runtime"
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
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
func (i Info) Uptime() string { return format.Duration(time.Since(i.Started)) }
