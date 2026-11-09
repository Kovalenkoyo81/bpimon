package bot

import (
	"log/slog"
	"strings"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/notifier"
)

const (
	actionReboot   = "reboot"
	actionPoweroff = "poweroff"
)

func (h *Handler) cmdReboot(userID int64) format.HTML {
	if !h.IsAdmin(userID) {
		return format.Text("⚠️ Only admins can reboot the server")
	}
	if !h.system.CanPower() {
		return format.Text("⚠️ Reboot unavailable: systemctl not found")
	}
	code := h.confirm.Request(userID, actionReboot, 6)
	return format.Fmt("🔐 Send %s within 60 seconds to <b>reboot</b>.", format.Code("/confirm "+code))
}

func (h *Handler) cmdPoweroff(userID int64) format.HTML {
	if !h.IsAdmin(userID) {
		return format.Text("⚠️ Only admins can power off the server")
	}
	if !h.system.CanPower() {
		return format.Text("⚠️ Poweroff unavailable: systemctl not found")
	}
	code := h.confirm.Request(userID, actionPoweroff, 6)
	return format.Fmt("🔐 Send %s within 60 seconds to <b>power off</b>.", format.Code("/confirm "+code))
}

// cmdConfirm executes the action a valid confirmation code was issued for.
func (h *Handler) cmdConfirm(args []string, userID int64, s notifier.Notifier) format.HTML {
	if len(args) == 0 {
		return format.Fmt("Usage: %s", format.Code("/confirm <code>"))
	}
	action, ok := h.confirm.Validate(userID, args[0])
	if !ok {
		return format.Text("⚠️ Invalid or expired code")
	}
	switch {
	case action == actionReboot:
		if err := h.system.Reboot(); err != nil {
			slog.Error("reboot failed", "err", err)
			return format.Text("❌ Failed to initiate reboot")
		}
		return format.Text("🔄 Server reboot initiated")
	case action == actionPoweroff:
		if err := h.system.Poweroff(); err != nil {
			slog.Error("poweroff failed", "err", err)
			return format.Text("❌ Failed to initiate poweroff")
		}
		return format.Text("🔌 Server poweroff initiated")
	case strings.HasPrefix(action, actionDockerPrefix):
		h.restartContainer(strings.TrimPrefix(action, actionDockerPrefix), s)
	}
	return format.HTML{}
}
