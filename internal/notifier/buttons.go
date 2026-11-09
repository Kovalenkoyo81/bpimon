package notifier

// Button is an inline button attached to a message. Data is returned
// verbatim in the callback when the button is pressed (Telegram: ≤64 bytes).
type Button struct {
	Text string
	Data string
}

// ButtonSender is implemented by notifiers that can attach inline buttons
// to a timestamped alert. Callers fall back to AlertSender.SendAlert when
// the notifier does not implement it.
type ButtonSender interface {
	// SendAlertWithButtons returns the sent message's ID.
	SendAlertWithButtons(text string, buttons []Button) (messageID int, err error)
	// ClearButtons removes the inline buttons from a sent message.
	ClearButtons(messageID int) error
}
