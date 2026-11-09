package bot

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/buildinfo"
	"github.com/Kovalenkoyo81/bpimon/internal/update"
)

type fakeSource struct {
	r   update.Release
	err error
}

func (f fakeSource) Latest(context.Context) (update.Release, error) { return f.r, f.err }

func reporter(src update.Source) *VersionReporter {
	return &VersionReporter{Build: buildinfo.New("v1.6.2", "2026-11-09"), Updates: src}
}

func TestStartupWithNewerRelease(t *testing.T) {
	v := reporter(fakeSource{r: update.Release{Tag: "v1.7.0", URL: "https://r"}})
	msg, tag := v.Startup(0)
	if !strings.Contains(msg, "bpimon v1.6.2 started") || !strings.Contains(msg, "v1.7.0 available: https://r") {
		t.Fatalf("unexpected startup message: %q", msg)
	}
	if tag != "v1.7.0" {
		t.Fatalf("expected announced tag v1.7.0, got %q", tag)
	}
	if strings.Contains(msg, "reboot") {
		t.Fatalf("no reboot line expected: %q", msg)
	}
}

func TestStartupAfterRebootUpToDate(t *testing.T) {
	v := reporter(fakeSource{r: update.Release{Tag: "v1.6.2"}})
	msg, tag := v.Startup(2*time.Minute + 3*time.Second)
	if !strings.Contains(msg, "After system reboot, uptime 2m 3s") || tag != "" || strings.Contains(msg, "available") {
		t.Fatalf("unexpected: %q tag=%q", msg, tag)
	}
}

func TestReportVariants(t *testing.T) {
	cases := []struct {
		src  update.Source
		want string
	}{
		{fakeSource{r: update.Release{Tag: "v1.7.0", URL: "https://r"}}, "v1.7.0 available"},
		{fakeSource{r: update.Release{Tag: "v1.6.2"}}, "Latest release: v1.6.2"},
		{fakeSource{err: errors.New("offline")}, "Could not check"},
	}
	for _, c := range cases {
		got := reporter(c.src).Report()
		if !strings.Contains(got, "bpimon v1.6.2") || !strings.Contains(got, "Built 2026-11-09") || !strings.Contains(got, c.want) {
			t.Errorf("Report() = %q, want it to contain %q", got, c.want)
		}
	}
}

func TestReportWithoutUpdates(t *testing.T) {
	got := reporter(nil).Report()
	if strings.Contains(got, "release") || strings.Contains(got, "updates") {
		t.Fatalf("no update info expected when disabled: %q", got)
	}
}

func TestVersionCommand(t *testing.T) {
	h := NewHandler(HandlerConfig{Version: reporter(fakeSource{r: update.Release{Tag: "v1.6.2"}})})
	if got := h.Handle("/version", 1, nil); !strings.Contains(got, "bpimon v1.6.2") {
		t.Fatalf("/version = %q", got)
	}
}
