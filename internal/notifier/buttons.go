package notifier

import "github.com/Kovalenkoyo81/bpimon/internal/format"

// Button is an inline button attached to a message. Data is returned
// verbatim in the callback when the button is pressed (Telegram: ≤64 bytes).
type Button struct {
	Text string
	Data string
}

// ButtonSender is implemented by notifiers that support inline buttons.
// Callers fall back to Notifier.Send when it is not implemented.
type ButtonSender interface {
	// SendWithButtons returns the sent message's ID.
	SendWithButtons(msg format.HTML, buttons []Button) (messageID int, err error)
	// ClearButtons removes the inline buttons from a sent message.
	ClearButtons(messageID int) error
}
