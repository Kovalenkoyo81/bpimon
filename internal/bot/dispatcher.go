package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"bpimon/internal/log"
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
				log.Warn.Println("Telegram updates channel closed")
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
	log.Info.Printf("response len=%d val=%q", len(response), response)
	_ = d.api.Send(response)
}

// handleCallback processes an inline-button press. Every press is answered,
// otherwise the button keeps spinning in the client.
func (d *Dispatcher) handleCallback(cq *tgbotapi.CallbackQuery) {
	var from int64
	if cq.From != nil {
		from = cq.From.ID
	}
	log.Info.Printf("callback received: data=%q from=%d", cq.Data, from)
	if cq.Message == nil || cq.Message.Chat == nil || cq.Message.Chat.ID != d.chatID || cq.From == nil {
		log.Warn.Printf("callback ignored: not from the configured chat")
		d.answer(cq.ID, "")
		return
	}
	res := d.callbacks.Handle(cq.Data, cq.From.ID, displayName(cq.From))
	log.Info.Printf("callback %q from %d: %s", cq.Data, cq.From.ID, res.Toast)
	d.answer(cq.ID, res.Toast)
	if res.Note != "" {
		if err := d.api.EditText(cq.Message.MessageID, cq.Message.Text+"\n\n"+res.Note); err != nil {
			log.Warn.Printf("edit message: %v", err)
		}
	}
}

func (d *Dispatcher) answer(callbackID, text string) {
	if err := d.api.AnswerCallback(callbackID, text); err != nil {
		log.Warn.Printf("answer callback: %v", err)
	}
}

func displayName(u *tgbotapi.User) string {
	if u.UserName != "" {
		return "@" + u.UserName
	}
	return u.FirstName
}
