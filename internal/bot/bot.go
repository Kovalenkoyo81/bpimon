package bot

import (
	"fmt"
	"time"

	"bpimon/internal/notifier"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	chatID int64
}

func New(token string, chatID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{api: api, chatID: chatID}, nil
}

func (b *Bot) Send(text string) error {
	_, err := b.api.Send(tgbotapi.NewMessage(b.chatID, text))
	return err
}

func (b *Bot) SendAlert(text string) error {
	return b.Send(alertStamp() + text)
}

// SendAlertWithButtons sends a timestamped alert with one row of inline buttons.
func (b *Bot) SendAlertWithButtons(text string, buttons []notifier.Button) error {
	row := make([]tgbotapi.InlineKeyboardButton, 0, len(buttons))
	for _, btn := range buttons {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.Data))
	}
	msg := tgbotapi.NewMessage(b.chatID, alertStamp()+text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row)
	_, err := b.api.Send(msg)
	return err
}

// AnswerCallback acknowledges a button press; non-empty text is shown as a toast.
func (b *Bot) AnswerCallback(callbackID, text string) error {
	_, err := b.api.Request(tgbotapi.NewCallback(callbackID, text))
	return err
}

// EditText replaces a message's text and removes its inline buttons.
func (b *Bot) EditText(messageID int, text string) error {
	_, err := b.api.Send(tgbotapi.NewEditMessageText(b.chatID, messageID, text))
	return err
}

func alertStamp() string {
	return fmt.Sprintf("🕐 %s UTC\n\n", time.Now().UTC().Format("02 Jan 2006, 15:04"))
}

// allowedUpdates is sent explicitly on every getUpdates call. Without it
// Telegram reuses the last list stored for the token — which may exclude
// callback_query, so inline-button presses would never arrive.
var allowedUpdates = []string{tgbotapi.UpdateTypeMessage, tgbotapi.UpdateTypeCallbackQuery}

func newUpdateConfig() tgbotapi.UpdateConfig {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = allowedUpdates
	return u
}

// UpdatesChan returns a channel of incoming updates.
// The channel closes if the Telegram connection is lost.
func (b *Bot) UpdatesChan() <-chan tgbotapi.Update {
	upstream := b.api.GetUpdatesChan(newUpdateConfig())
	out := make(chan tgbotapi.Update)
	go func() {
		defer close(out)
		for u := range upstream {
			out <- u
		}
	}()
	return out
}
