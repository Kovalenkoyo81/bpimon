package update

import (
	"strconv"
	"strings"
)

// parseVersion turns "v1.6.2", "1.6.2" or a git-describe build such as
// "v1.6.2-3-gabc123" into [1 6 2]. A describe build is treated as its base tag.
func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// IsNewer reports whether latest is a strictly higher version than current.
// Unparseable versions (e.g. "dev") are never considered older.
func IsNewer(latest, current string) bool {
	l, ok1 := parseVersion(latest)
	c, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}
