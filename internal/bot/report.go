package bot

import (
	"slices"
	"strings"

	"bpimon/internal/monitor"
)

func (h *Handler) has(k monitor.Kind) bool {
	for _, p := range h.providers {
		if p.Kind() == k {
			return true
		}
	}
	return false
}

// collect joins the status of all providers of the given kinds
// (all providers if none are given), in provider order.
func (h *Handler) collect(kinds ...monitor.Kind) string {
	var b strings.Builder
	for _, p := range h.providers {
		if len(kinds) > 0 && !slices.Contains(kinds, p.Kind()) {
			continue
		}
		s, err := p.Status()
		if err != nil {
			s = p.Name() + ": error reading status"
		}
		b.WriteString(s + "\n\n")
	}
	return strings.TrimSpace(b.String())
}
