package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// now is replaceable in tests.
var now = time.Now

const maxSilenceMinutes = 10080 // 7 days

func (h *Handler) cmdSilence(args []string, userID int64) string {
	if !h.IsAdmin(userID) {
		return "⚠️ Only admins can manage alert silence"
	}
	if h.silencer == nil {
		return "⚠️ Alert manager not available"
	}
	if len(args) == 0 {
		until := h.silencer.SilencedUntil()
		if until.IsZero() || now().After(until) {
			return "🔔 Alerts active"
		}
		remaining := until.Sub(now()).Round(time.Minute)
		return fmt.Sprintf("🔕 Alerts silenced until %s UTC (%v remaining)", until.UTC().Format("15:04"), remaining)
	}
	if strings.ToLower(args[0]) == "off" {
		h.silencer.Unsilence()
		return "🔔 Alerts resumed"
	}
	mins, err := strconv.Atoi(args[0])
	if err != nil || mins <= 0 {
		return "⚠️ Specify minutes: /silence 30"
	}
	if mins > maxSilenceMinutes {
		return "⚠️ Maximum silence duration is 10080 minutes (7 days)"
	}
	d := time.Duration(mins) * time.Minute
	h.silencer.Silence(d)
	until := now().Add(d).UTC().Format("15:04 UTC")
	return fmt.Sprintf("🔕 Alerts silenced for %dm (until %s)", mins, until)
}
