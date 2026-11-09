package bot

import (
	"bpimon/internal/alert"
	"errors"
)

// CallbackResult tells the caller how to react to a button press.
type CallbackResult struct {
	Toast string // shown to the presser as a popup; may be empty
	Note  string // appended to the original message (buttons removed); empty = leave message as is
}

// CallbackHandler turns inline-button presses into alert resolutions.
type CallbackHandler struct {
	Resolver alert.Resolver
	IsAdmin  func(userID int64) bool
}

// Handle processes callback data from a button pressed by userID (shown as who).
func (h *CallbackHandler) Handle(data string, userID int64, who string) CallbackResult {
	name, r, ok := alert.ParseCallbackData(data)
	if !ok {
		return CallbackResult{Toast: "Unknown action"}
	}
	if !h.IsAdmin(userID) {
		return CallbackResult{Toast: "⚠️ Only admins can answer alerts"}
	}
	if err := h.Resolver.Resolve(name, r); err != nil {
		if errors.Is(err, alert.ErrNotActive) {
			return CallbackResult{Toast: "Already back to normal", Note: "ℹ️ Resolved before an answer was given"}
		}
		if errors.Is(err, alert.ErrAmbiguous) {
			return CallbackResult{Toast: "⚠️ Ambiguous container name, answer not recorded"}
		}
		return CallbackResult{Toast: "Error: " + err.Error()}
	}
	if r == alert.Intentional {
		return CallbackResult{
			Toast: "OK, silent until it is running again",
			Note:  "✋ Stopped by admin (" + who + ") — silent until running again",
		}
	}
	return CallbackResult{
		Toast: "OK, reminders continue until it recovers",
		Note:  "💥 Marked as crash (" + who + ") — reminders continue until it recovers",
	}
}
