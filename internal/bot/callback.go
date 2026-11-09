package bot

import (
	"errors"

	"github.com/Kovalenkoyo81/bpimon/internal/alert"
)

// AnsweredData is the callback data of the status button that replaces the
// answer buttons once a question has been answered.
const AnsweredData = "answered"

// CallbackResult tells the caller how to react to a button press.
type CallbackResult struct {
	Toast  string // shown to the presser as a popup; may be empty
	Status string // replaces the message's buttons with one status button; empty = leave buttons as is
}

// CallbackHandler turns inline-button presses into alert resolutions.
type CallbackHandler struct {
	Resolver alert.Resolver
	IsAdmin  func(userID int64) bool
}

// Handle processes callback data from a button pressed by userID (shown as who).
func (h *CallbackHandler) Handle(data string, userID int64, who string) CallbackResult {
	if data == AnsweredData {
		return CallbackResult{Toast: "Already answered"}
	}
	name, r, ok := alert.ParseCallbackData(data)
	if !ok {
		return CallbackResult{Toast: "Unknown action"}
	}
	if !h.IsAdmin(userID) {
		return CallbackResult{Toast: "⚠️ Only admins can answer alerts"}
	}
	if err := h.Resolver.Resolve(name, r); err != nil {
		if errors.Is(err, alert.ErrNotActive) {
			return CallbackResult{Toast: "Already back to normal", Status: "ℹ️ Recovered before an answer"}
		}
		if errors.Is(err, alert.ErrUnknown) {
			return CallbackResult{Toast: "⚠️ State unknown right now (Docker daemon unavailable?), try again later"}
		}
		if errors.Is(err, alert.ErrAmbiguous) {
			return CallbackResult{Toast: "⚠️ Ambiguous container name, answer not recorded"}
		}
		return CallbackResult{Toast: "Error: " + err.Error()}
	}
	if r == alert.Intentional {
		return CallbackResult{
			Toast:  "OK, silent until it is running again",
			Status: "✋ Stopped by admin · " + who,
		}
	}
	return CallbackResult{
		Toast:  "OK, reminders continue until it recovers",
		Status: "💥 Crash · " + who,
	}
}
