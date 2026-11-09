package bot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"log/slog"

	"github.com/Kovalenkoyo81/bpimon/internal/notifier"

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

// Send sends a plain message. Failures are logged here, so callers that
// cannot do anything about them may ignore the error.
func (b *Bot) Send(text string) error {
	_, err := b.api.Send(tgbotapi.NewMessage(b.chatID, text))
	return logSendErr("send message", err)
}

func logSendErr(what string, err error) error {
	if err != nil {
		slog.Error("telegram request failed", "op", what, "err", err)
	}
	return err
}

func (b *Bot) SendAlert(text string) error {
	return b.Send(alertStamp() + text)
}

// SendAlertWithButtons sends a timestamped alert with one row of inline
// buttons and returns the message ID.
func (b *Bot) SendAlertWithButtons(text string, buttons []notifier.Button) (int, error) {
	row := make([]tgbotapi.InlineKeyboardButton, 0, len(buttons))
	for _, btn := range buttons {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.Data))
	}
	msg := tgbotapi.NewMessage(b.chatID, alertStamp()+text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row)
	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, logSendErr("send alert with buttons", err)
	}
	return sent.MessageID, nil
}

// ClearButtons removes the inline keyboard from a message, keeping its text.
func (b *Bot) ClearButtons(messageID int) error {
	empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	_, err := b.api.Request(tgbotapi.NewEditMessageReplyMarkup(b.chatID, messageID, empty))
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

// UpdatesChan returns the channel of incoming updates. The library retries
// failed polls on its own; the channel is closed only by StopUpdates.
func (b *Bot) UpdatesChan() <-chan tgbotapi.Update {
	return b.api.GetUpdatesChan(newUpdateConfig())
}

// StopUpdates stops long polling and closes the updates channel.
func (b *Bot) StopUpdates() { b.api.StopReceivingUpdates() }

// Connect retries New with linear backoff (5s steps, max 60s) until it
// succeeds or ctx is cancelled. This covers DNS or network not being ready
// at boot.
func Connect(ctx context.Context, token string, chatID int64) (*Bot, error) {
	for attempt := 1; ; attempt++ {
		b, err := New(token, chatID)
		if err == nil {
			return b, nil
		}
		delay := min(time.Duration(attempt)*5*time.Second, 60*time.Second)
		msg := "connection failed"
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			msg = urlErr.Err.Error()
		}
		slog.Warn("telegram unavailable", "attempt", attempt, "reason", msg, "retry_in", delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
