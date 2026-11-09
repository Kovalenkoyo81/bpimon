package bot

import (
	"bpimon/internal/buildinfo"
	"log/slog"
	"bpimon/internal/update"
	"context"
	"time"
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
func (v *VersionReporter) Startup(rebootUptime time.Duration) (msg, announced string) {
	msg = "🤖 bpimon " + v.Build.Version + " started"
	if rebootUptime > 0 {
		msg += "\n🔁 After system reboot, uptime " + buildinfo.FormatDuration(rebootUptime)
	}
	if r, ok := v.newerRelease(); ok {
		msg += "\n🆕 " + r.Tag + " available: " + r.URL
		announced = r.Tag
	}
	return msg, announced
}

// Report is the /version response.
func (v *VersionReporter) Report() string {
	msg := "🤖 bpimon " + v.Build.Version +
		"\n🏗 Built " + v.Build.BuildDate + ", " + v.Build.Platform() +
		"\n⏱ Up " + v.Build.Uptime()
	if v.Updates == nil {
		return msg
	}
	r, ok := v.newerRelease()
	switch {
	case ok:
		msg += "\n🆕 " + r.Tag + " available: " + r.URL
	case r.Tag != "":
		msg += "\n✅ Latest release: " + r.Tag
	default:
		msg += "\n⚠️ Could not check for updates"
	}
	return msg
}
