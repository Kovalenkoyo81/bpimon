package bot

import (
	"fmt"
	"strings"

	"bpimon/internal/log"
	"bpimon/internal/notifier"
)

const (
	actionReboot   = "reboot"
	actionPoweroff = "poweroff"
)

func (h *Handler) cmdReboot(userID int64) string {
	if !h.IsAdmin(userID) {
		return "⚠️ Only admins can reboot the server"
	}
	if !h.system.CanPower() {
		return "⚠️ Reboot unavailable: systemctl not found"
	}
	code := h.confirm.Request(userID, actionReboot, 6)
	return fmt.Sprintf("🔐 Type `/confirm %s` within 60 seconds to confirm reboot.", code)
}

func (h *Handler) cmdPoweroff(userID int64) string {
	if !h.IsAdmin(userID) {
		return "⚠️ Only admins can power off the server"
	}
	if !h.system.CanPower() {
		return "⚠️ Poweroff unavailable: systemctl not found"
	}
	code := h.confirm.Request(userID, actionPoweroff, 6)
	return fmt.Sprintf("🔐 Type `/confirm %s` within 60 seconds to confirm poweroff.", code)
}

// cmdConfirm executes the action a valid confirmation code was issued for.
func (h *Handler) cmdConfirm(args []string, userID int64, s notifier.Notifier) string {
	if len(args) == 0 {
		return "Usage: /confirm <code>"
	}
	action, ok := h.confirm.Validate(userID, args[0])
	if !ok {
		return "⚠️ Invalid or expired code"
	}
	switch {
	case action == actionReboot:
		if err := h.system.Reboot(); err != nil {
			log.Error.Printf("reboot failed: %v", err)
			return "❌ Failed to initiate reboot"
		}
		return "🔄 Server reboot initiated"
	case action == actionPoweroff:
		if err := h.system.Poweroff(); err != nil {
			log.Error.Printf("poweroff failed: %v", err)
			return "❌ Failed to initiate poweroff"
		}
		return "🔌 Server poweroff initiated"
	case strings.HasPrefix(action, actionDockerPrefix):
		h.restartContainer(strings.TrimPrefix(action, actionDockerPrefix), s)
		return ""
	}
	return ""
}
