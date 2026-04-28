package notifier

type Notifier interface {
	Send(text string) error
}

// AlertSender extends Notifier with a timestamped alert method.
type AlertSender interface {
	Notifier
	SendAlert(text string) error
}
