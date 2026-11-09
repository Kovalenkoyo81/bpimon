package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/buildinfo"
	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/update"
)

const updateCheckTimeout = 5 * time.Second

// VersionReporter formats version information for the bot.
type VersionReporter struct {
	Build   buildinfo.Info
	Updates update.Source // nil disables update checks
}

// newerRelease returns the latest release if it is newer than the running build.
func (v *VersionReporter) newerRelease() (update.Release, bool) {
	if v.Updates == nil {
		return update.Release{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()
	r, err := v.Updates.Latest(ctx)
	if err != nil {
		slog.Warn("update check failed", "err", err)
		return update.Release{}, false
	}
	return r, update.IsNewer(r.Tag, v.Build.Version)
}

// Startup builds the first message after the bot starts. rebootUptime > 0
// means the bot started right after a system reboot. It returns the release
// tag it announced, if any.
//
//	🤖 bpimon v1.8.0 started · bananapipro
//	🔁 after system reboot, uptime 1m 12s
//	🆕 v1.9.0 available · release notes
func (v *VersionReporter) Startup(host string, rebootUptime time.Duration) (msg format.HTML, announced string) {
	head := format.Concat(format.Text("🤖 "), format.Bold("bpimon "+v.Build.Version+" started"))
	if host != "" {
		head = format.Concat(head, format.Text(" · "+host))
	}
	var reboot, avail format.HTML
	if rebootUptime > 0 {
		reboot = format.Text("🔁 after system reboot, uptime " + format.Duration(rebootUptime))
	}
	if r, ok := v.newerRelease(); ok {
		avail = newerLine(r)
		announced = r.Tag
	}
	return format.Lines(head, reboot, avail), announced
}

// Report is the /version response.
//
//	🤖 bpimon v1.8.0
//	built 2026-11-09 · linux/arm · up 3h 12m
//	✅ latest release
func (v *VersionReporter) Report() format.HTML {
	head := format.Concat(format.Text("🤖 "), format.Bold("bpimon "+v.Build.Version))
	details := format.Text("built " + v.Build.BuildDate + " · " + v.Build.Platform() + " · up " + v.Build.Uptime())
	if v.Updates == nil {
		return format.Lines(head, details)
	}
	r, ok := v.newerRelease()
	var status format.HTML
	switch {
	case ok:
		status = newerLine(r)
	case r.Tag != "":
		status = format.Fmt("✅ latest release is %s", format.Code(r.Tag))
	default:
		status = format.Text("⚠️ could not check for updates")
	}
	return format.Lines(head, details, status)
}

func newerLine(r update.Release) format.HTML {
	return format.Fmt(`🆕 <b>%s</b> available · <a href="%s">release notes</a>`, r.Tag, r.URL)
}
