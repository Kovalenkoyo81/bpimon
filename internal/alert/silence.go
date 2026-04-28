package alert

import "time"

type Silencer interface {
	Silence(d time.Duration)
	Unsilence()
	IsSilenced() bool
	SilencedUntil() time.Time
}
