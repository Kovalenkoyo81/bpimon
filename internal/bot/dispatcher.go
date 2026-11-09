package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"log/slog"
)

// chatAPI is the part of the Telegram client the dispatcher needs.
type chatAPI interface {
	Send(text string) error
	AnswerCallback(callbackID, text string) error
	EditText(messageID int, text string) error
}

// Dispatcher routes incoming Telegram updates from the configured chat:
// messages to the Handler, inline-button presses to the CallbackHandler.
type Dispatcher struct {
	api       chatAPI
	handler   *Handler
	callbacks *CallbackHandler
	chatID    int64
}

func NewDispatcher(api chatAPI, h *Handler, cb *CallbackHandler, chatID int64) *Dispatcher {
	return &Dispatcher{api: api, handler: h, callbacks: cb, chatID: chatID}
}

// Run processes updates until the channel is closed or ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context, updates <-chan tgbotapi.Update) {
	for {
		select {
		case u, ok := <-updates:
			if !ok {
				slog.Warn("telegram updates channel closed")
				return
			}
			d.dispatch(u)
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) dispatch(u tgbotapi.Update) {
	switch {
	case u.CallbackQuery != nil:
		d.handleCallback(u.CallbackQuery)
	case u.Message != nil:
		d.handleMessage(u.Message)
	}
}

func (d *Dispatcher) handleMessage(m *tgbotapi.Message) {
	if m.Chat == nil || m.Chat.ID != d.chatID || m.From == nil {
		return
	}
	response := d.handler.Handle(m.Text, m.From.ID, d.api)
	if response == "" {
		return
	}
	slog.Debug("response", "len", len(response), "text", response)
	_ = d.api.Send(response)
}

// handleCallback processes an inline-button press. Every press is answered,
// otherwise the button keeps spinning in the client.
func (d *Dispatcher) handleCallback(cq *tgbotapi.CallbackQuery) {
	var from int64
	if cq.From != nil {
		from = cq.From.ID
	}
	slog.Info("callback received", "data", cq.Data, "user", from)
	if cq.Message == nil || cq.Message.Chat == nil || cq.Message.Chat.ID != d.chatID || cq.From == nil {
		slog.Warn("callback ignored: not from the configured chat", "data", cq.Data, "user", from)
		d.answer(cq.ID, "")
		return
	}
	res := d.callbacks.Handle(cq.Data, cq.From.ID, displayName(cq.From))
	slog.Info("callback handled", "data", cq.Data, "user", cq.From.ID, "result", res.Toast)
	d.answer(cq.ID, res.Toast)
	if res.Note != "" {
		if err := d.api.EditText(cq.Message.MessageID, cq.Message.Text+"\n\n"+res.Note); err != nil {
			slog.Warn("edit message failed", "err", err)
		}
	}
}

func (d *Dispatcher) answer(callbackID, text string) {
	if err := d.api.AnswerCallback(callbackID, text); err != nil {
		slog.Warn("answer callback failed", "err", err)
	}
}

func displayName(u *tgbotapi.User) string {
	if u.UserName != "" {
		return "@" + u.UserName
	}
	return u.FirstName
}
