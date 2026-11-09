package bot

import (
	"fmt"
	"os"
	"time"

	"bpimon/internal/buildinfo"
	"bpimon/internal/notifier"
)

// rebootFlagFile lives in /tmp, which is cleared on every real reboot.
// If it already exists, this is a manual bot restart — not a reboot.
const rebootFlagFile = "/tmp/bpimon.booted"

// SendStartupMessage announces the bot start with its version, notes a
// recent system reboot, and mentions a newer release if one exists.
// It returns the release tag it announced, if any.
func (h *Handler) SendStartupMessage(s notifier.Notifier) string {
	reboot := systemRebootUptime()
	if h.version == nil {
		if reboot > 0 {
			_ = s.Send("🔁 Bot started after system reboot\n⏱ System uptime: " + buildinfo.FormatDuration(reboot))
		}
		return ""
	}
	msg, announced := h.version.Startup(reboot)
	_ = s.Send(msg)
	return announced
}

func (h *Handler) SendInitialStatus(s notifier.Notifier) {
	if msg := h.collect(); msg != "" {
		_ = s.Send("📊 Initial system status:\n" + msg)
	}
}

// systemRebootUptime returns the system uptime if the bot is starting for
// the first time since a reboot less than 5 minutes ago, or 0 otherwise.
func systemRebootUptime() time.Duration {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	var uptimeSec float64
	if _, err := fmt.Sscanf(string(b), "%f", &uptimeSec); err != nil || uptimeSec >= 300 {
		return 0
	}
	if _, err := os.Stat(rebootFlagFile); err == nil {
		return 0
	}
	_ = os.WriteFile(rebootFlagFile, []byte{}, 0600)
	return time.Duration(uptimeSec * float64(time.Second))
}
