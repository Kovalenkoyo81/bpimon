package bot

import (
	"fmt"
	"time"

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
	stamp := fmt.Sprintf("🕐 %s UTC\n\n", time.Now().UTC().Format("02 Jan 2006, 15:04"))
	return b.Send(stamp + text)
}

// UpdatesChan returns a channel of incoming updates.
// The channel closes if the Telegram connection is lost.
func (b *Bot) UpdatesChan() <-chan tgbotapi.Update {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	upstream := b.api.GetUpdatesChan(u)
	out := make(chan tgbotapi.Update)
	go func() {
		defer close(out)
		for u := range upstream {
			out <- u
		}
	}()
	return out
}
