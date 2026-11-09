package update

import (
	"bpimon/internal/log"
	"bpimon/internal/notifier"
	"context"
	"strings"
	"time"
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
		log.Warn.Printf("update check: %v", err)
		return
	}
	if !IsNewer(r.Tag, w.Current) || r.Tag == w.announced {
		return
	}
	if err := w.Notifier.Send(Announcement(r, w.Current)); err != nil {
		log.Warn.Printf("update announcement: %v", err)
		return
	}
	w.announced = r.Tag
}

// Announcement is the full "new version" message with release notes.
func Announcement(r Release, current string) string {
	var b strings.Builder
	b.WriteString("🆕 bpimon " + r.Tag + " is available (running " + current + ")\n")
	b.WriteString(r.URL)
	if notes := trimNotes(r.Notes); notes != "" {
		b.WriteString("\n\n" + notes)
	}
	return b.String()
}

// trimNotes keeps the first maxNotesLines non-empty lines of release notes.
func trimNotes(notes string) string {
	var kept []string
	lines := strings.Split(strings.ReplaceAll(notes, "\r\n", "\n"), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if len(kept) == maxNotesLines {
			kept = append(kept, "…")
			break
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}
