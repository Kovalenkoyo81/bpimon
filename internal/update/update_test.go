package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.7.0", "v1.6.2", true},
		{"v1.6.10", "v1.6.9", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.6.2", "v1.6.2", false},
		{"v1.6.1", "v1.6.2", false},
		{"v1.6.2", "v1.6.2-3-gabc123", false}, // describe build of the same tag
		{"v1.6.3", "v1.6.2-3-gabc123", true},
		{"v1.7.0", "dev", false},
		{"garbage", "v1.0.0", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.latest, c.current); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestGitHubLatestAndCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/repos/o/r/releases/latest" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header is required by GitHub")
		}
		_, _ = w.Write([]byte(`{"tag_name":"v1.7.0","html_url":"https://x/v1.7.0","body":"notes"}`))
	}))
	defer srv.Close()

	g := NewGitHub("o/r")
	g.BaseURL = srv.URL
	for i := 0; i < 3; i++ {
		r, err := g.Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if r.Tag != "v1.7.0" || r.URL != "https://x/v1.7.0" || r.Notes != "notes" {
			t.Fatalf("unexpected release %+v", r)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 request thanks to cache, got %d", hits.Load())
	}
}

func TestGitHubHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	g := NewGitHub("o/r")
	g.BaseURL = srv.URL
	if _, err := g.Latest(context.Background()); err == nil {
		t.Fatal("expected error on HTTP 403")
	}
}

type stubSource struct {
	r   Release
	err error
}

func (s *stubSource) Latest(context.Context) (Release, error) { return s.r, s.err }

type sink struct {
	mu   sync.Mutex
	msgs []string
}

func (s *sink) Send(t string) error {
	s.mu.Lock()
	s.msgs = append(s.msgs, t)
	s.mu.Unlock()
	return nil
}

func TestWatcherAnnouncesEachReleaseOnce(t *testing.T) {
	src := &stubSource{r: Release{Tag: "v1.7.0", URL: "u", Notes: "* feat: x"}}
	out := &sink{}
	w := &Watcher{Source: src, Current: "v1.6.2", Notifier: out, Interval: time.Hour}

	w.check(context.Background())
	w.check(context.Background())
	if len(out.msgs) != 1 {
		t.Fatalf("expected 1 announcement, got %v", out.msgs)
	}
	if !strings.Contains(out.msgs[0], "v1.7.0") || !strings.Contains(out.msgs[0], "feat: x") {
		t.Fatalf("announcement must include tag and notes: %q", out.msgs[0])
	}

	src.r.Tag = "v1.8.0"
	w.check(context.Background())
	if len(out.msgs) != 2 {
		t.Fatalf("expected a new announcement for v1.8.0, got %v", out.msgs)
	}
}

func TestWatcherSkipsAlreadyShownAndOldAndErrors(t *testing.T) {
	out := &sink{}
	w := &Watcher{Source: &stubSource{r: Release{Tag: "v1.7.0"}}, Current: "v1.6.2", Notifier: out}
	w.MarkAnnounced("v1.7.0")
	w.check(context.Background())

	w.Source = &stubSource{r: Release{Tag: "v1.6.2"}}
	w.check(context.Background())

	w.Source = &stubSource{err: errors.New("offline")}
	w.check(context.Background())

	if len(out.msgs) != 0 {
		t.Fatalf("expected no announcements, got %v", out.msgs)
	}
}

func TestTrimNotes(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "line", "")
	}
	got := trimNotes(strings.Join(lines, "\r\n"))
	n := strings.Count(got, "\n") + 1
	if n != maxNotesLines+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("expected %d lines ending with …, got %d: %q", maxNotesLines+1, n, got)
	}
}
