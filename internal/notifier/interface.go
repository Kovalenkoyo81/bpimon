package notifier

import "github.com/Kovalenkoyo81/bpimon/internal/format"

// Notifier delivers a message to the chat.
type Notifier interface {
	Send(msg format.HTML) error
}
