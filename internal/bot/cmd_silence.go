package bot

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
)

// now is replaceable in tests.
var now = time.Now

const maxSilenceMinutes = 10080 // 7 days

func (h *Handler) cmdSilence(args []string, userID int64) format.HTML {
	if !h.IsAdmin(userID) {
		return format.Text("⚠️ Only admins can manage alert silence")
	}
	if h.silencer == nil {
		return format.Text("⚠️ Alert manager not available")
	}
	if len(args) == 0 {
		until := h.silencer.SilencedUntil()
		if until.IsZero() || now().After(until) {
			return format.Text("🔔 Alerts active")
		}
		return format.Fmt("🔕 Alerts silenced until <b>%s</b> (%s left)",
			until.UTC().Format("15:04 UTC"), format.Duration(until.Sub(now()).Round(time.Minute)))
	}
	if strings.ToLower(args[0]) == "off" {
		h.silencer.Unsilence()
		return format.Text("🔔 Alerts resumed")
	}
	mins, err := strconv.Atoi(args[0])
	if err != nil || mins <= 0 {
		return format.Fmt("⚠️ Specify minutes: %s", format.Code("/silence 30"))
	}
	if mins > maxSilenceMinutes {
		return format.Text("⚠️ Maximum silence duration is 10080 minutes (7 days)")
	}
	d := time.Duration(mins) * time.Minute
	h.silencer.Silence(d)
	return format.Fmt("🔕 Alerts silenced for <b>%s</b>, until %s",
		format.Duration(d), now().Add(d).UTC().Format("15:04 UTC"))
}
