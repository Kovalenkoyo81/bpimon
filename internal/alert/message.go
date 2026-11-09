package alert

import (
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
)

// Headliner is implemented by alerts with a custom first line, e.g.
// "picoclaw-gateway is down" instead of the alert name.
type Headliner interface {
	Headline(firing bool) string
}

func headline(a Alert, firing bool) string {
	if h, ok := a.(Headliner); ok {
		return h.Headline(firing)
	}
	if firing {
		return a.Name()
	}
	return a.Name() + " back to normal"
}

// firingMessage: what happened first (it is what a push notification
// shows), then details, then the time.
//
//	🚨 picoclaw-gateway is down · bananapipro
//	status: exited · for 1h 0m
//	21:35 UTC
func firingMessage(title, host, detail string, ongoing time.Duration, at time.Time) format.HTML {
	body := format.Text(detail)
	if ongoing > 0 {
		body = format.Lines(body, format.Text("for "+format.Duration(ongoing)))
	}
	return format.Lines(
		format.Concat(format.Text("🚨 "), format.Bold(title), hostSuffix(host)),
		body,
		stamp(at),
	)
}

// ✅ picoclaw-gateway is back · bananapipro
// after 2h 14m
// 23:49 UTC
func recoveryMessage(title, host string, lasted time.Duration, at time.Time) format.HTML {
	var body format.HTML
	if lasted > 0 {
		body = format.Text("after " + format.Duration(lasted))
	}
	return format.Lines(
		format.Concat(format.Text("✅ "), format.Bold(title), hostSuffix(host)),
		body,
		stamp(at),
	)
}

func hostSuffix(host string) format.HTML {
	if host == "" {
		return format.HTML{}
	}
	return format.Text(" · " + host)
}

func stamp(at time.Time) format.HTML {
	return format.Italic(at.UTC().Format("15:04 UTC"))
}
