package update

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/notifier"
)

const maxNotesLines = 15

// Watcher periodically checks for a newer release and announces each new
// release once.
type Watcher struct {
	Source   Source
	Current  string
	Notifier notifier.Notifier
	Interval time.Duration

	announced string // last tag announced (or shown at startup)
}

// MarkAnnounced records a tag already shown to the user (e.g. in the
// startup message) so the watcher does not announce it again.
func (w *Watcher) MarkAnnounced(tag string) { w.announced = tag }

func (w *Watcher) Run(ctx context.Context) {
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			w.check(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Watcher) check(ctx context.Context) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	r, err := w.Source.Latest(cctx)
	if err != nil {
		slog.Warn("update check failed", "err", err)
		return
	}
	if !IsNewer(r.Tag, w.Current) || r.Tag == w.announced {
		return
	}
	if err := w.Notifier.Send(Announcement(r, w.Current)); err != nil {
		slog.Warn("update announcement failed", "err", err)
		return
	}
	w.announced = r.Tag
}

// Announcement is the full "new version" message with release notes.
//
//	🆕 bpimon v1.8.0 is available
//	running v1.7.1 · release notes
//
//	Added
//	• ...
func Announcement(r Release, current string) format.HTML {
	head := format.Lines(
		format.Concat(format.Text("🆕 "), format.Bold("bpimon "+r.Tag+" is available")),
		format.Fmt(`running %s · <a href="%s">release notes</a>`, current, r.URL),
	)
	if notes := renderNotes(r.Notes); !notes.IsZero() {
		return format.Concat(head, format.Text("\n\n"), notes)
	}
	return head
}

// renderNotes turns the first maxNotesLines non-empty lines of Markdown
// release notes into Telegram HTML: "### X" becomes a bold heading, list
// items get a bullet, inline Markdown markers are dropped.
func renderNotes(notes string) format.HTML {
	var out []format.HTML
	lines := strings.Split(strings.ReplaceAll(notes, "\r\n", "\n"), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if len(out) == maxNotesLines {
			out = append(out, format.Text("…"))
			break
		}
		switch {
		case strings.HasPrefix(l, "#"):
			out = append(out, format.Bold(stripMarkdown(strings.TrimLeft(l, "# "))))
		case strings.HasPrefix(l, "- "), strings.HasPrefix(l, "* "):
			out = append(out, format.Text("• "+stripMarkdown(l[2:])))
		default:
			out = append(out, format.Text(stripMarkdown(l)))
		}
	}
	return format.Lines(out...)
}

var mdMarkers = strings.NewReplacer("**", "", "__", "", "`", "")

func stripMarkdown(s string) string { return mdMarkers.Replace(s) }
