package bot

import (
	"slices"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Telegram remembers allowed_updates per token; if callback_query is not
// requested explicitly, inline-button presses may never be delivered.
func TestUpdateConfigRequestsCallbacks(t *testing.T) {
	u := newUpdateConfig()
	for _, want := range []string{tgbotapi.UpdateTypeMessage, tgbotapi.UpdateTypeCallbackQuery} {
		if !slices.Contains(u.AllowedUpdates, want) {
			t.Errorf("AllowedUpdates %v must contain %q", u.AllowedUpdates, want)
		}
	}
}
