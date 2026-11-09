package bot

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/notifier"
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

// Send sends an HTML message. Failures are logged here, so callers that
// cannot do anything about them may ignore the error.
func (b *Bot) Send(msg format.HTML) error {
	_, err := b.api.Send(b.newMessage(msg))
	return logSendErr("send message", err)
}

// SendWithButtons sends an HTML message with one row of inline buttons and
// returns the message ID.
func (b *Bot) SendWithButtons(msg format.HTML, buttons []notifier.Button) (int, error) {
	m := b.newMessage(msg)
	m.ReplyMarkup = keyboard(buttons)
	sent, err := b.api.Send(m)
	if err != nil {
		return 0, logSendErr("send message with buttons", err)
	}
	return sent.MessageID, nil
}

// ClearButtons removes the inline keyboard from a message, keeping its text.
func (b *Bot) ClearButtons(messageID int) error {
	return b.setKeyboard(messageID, nil)
}

// SetStatusButton replaces a message's buttons with a single button that
// shows the outcome (e.g. who answered). The message text and its
// formatting stay untouched.
func (b *Bot) SetStatusButton(messageID int, text, data string) error {
	return b.setKeyboard(messageID, []notifier.Button{{Text: text, Data: data}})
}

// AnswerCallback acknowledges a button press; non-empty text is shown as a toast.
func (b *Bot) AnswerCallback(callbackID, text string) error {
	_, err := b.api.Request(tgbotapi.NewCallback(callbackID, text))
	return err
}

func (b *Bot) newMessage(msg format.HTML) tgbotapi.MessageConfig {
	m := tgbotapi.NewMessage(b.chatID, msg.String())
	m.ParseMode = tgbotapi.ModeHTML
	return m
}

func (b *Bot) setKeyboard(messageID int, buttons []notifier.Button) error {
	_, err := b.api.Request(tgbotapi.NewEditMessageReplyMarkup(b.chatID, messageID, keyboard(buttons)))
	return err
}

// keyboard builds a one-row inline keyboard; nil buttons give an empty one.
func keyboard(buttons []notifier.Button) tgbotapi.InlineKeyboardMarkup {
	if len(buttons) == 0 {
		return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	}
	row := make([]tgbotapi.InlineKeyboardButton, 0, len(buttons))
	for _, btn := range buttons {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.Data))
	}
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func logSendErr(what string, err error) error {
	if err != nil {
		slog.Error("telegram request failed", "op", what, "err", err)
	}
	return err
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
