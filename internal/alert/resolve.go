package alert

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Resolution is an admin's answer to an Askable alert.
type Resolution int

const (
	unresolved Resolution = iota
	// Intentional: the condition was caused on purpose. The alert stays
	// silent until it clears, and no recovery message is sent.
	Intentional
	// Incident: a real problem. Plain reminders continue every cooldown
	// and a recovery message is sent when it clears.
	Incident
)

// Resolver records answers to Askable alerts.
type Resolver interface {
	Resolve(alertName string, r Resolution) error
}

var (
	ErrNotActive = errors.New("alert is not active")
	ErrAmbiguous = errors.New("alert name prefix matches several alerts")
)

// Callback data format: "res:<i|c>:<alert name>".
const (
	callbackPrefix  = "res:"
	maxCallbackData = 64 // Telegram limit, bytes
)

// CallbackData encodes a resolution button's payload. The alert name is
// truncated to fit Telegram's limit; Resolve matches truncated names by prefix.
func CallbackData(alertName string, r Resolution) string {
	code := "i"
	if r == Incident {
		code = "c"
	}
	head := callbackPrefix + code + ":"
	return head + truncateUTF8(alertName, maxCallbackData-len(head))
}

// truncateUTF8 cuts s to at most n bytes without splitting a rune.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// ParseCallbackData is the inverse of CallbackData.
func ParseCallbackData(data string) (alertName string, r Resolution, ok bool) {
	rest, found := strings.CutPrefix(data, callbackPrefix)
	if !found || len(rest) < 3 || rest[1] != ':' {
		return "", unresolved, false
	}
	switch rest[0] {
	case 'i':
		r = Intentional
	case 'c':
		r = Incident
	default:
		return "", unresolved, false
	}
	return rest[2:], r, true
}
