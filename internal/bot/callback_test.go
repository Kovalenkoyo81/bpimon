package bot

import (
	"strings"
	"testing"

	"github.com/Kovalenkoyo81/bpimon/internal/alert"
)

type stubResolver struct {
	err  error
	name string
	r    alert.Resolution
}

func (s *stubResolver) Resolve(name string, r alert.Resolution) error {
	s.name, s.r = name, r
	return s.err
}

func newCB(err error) (*CallbackHandler, *stubResolver) {
	res := &stubResolver{err: err}
	return &CallbackHandler{Resolver: res, IsAdmin: func(id int64) bool { return id == 1 }}, res
}

func TestCallbackNonAdminRejected(t *testing.T) {
	h, res := newCB(nil)
	out := h.Handle(alert.CallbackData("Docker app", alert.Intentional), 2, "@bob")
	if out.Note != "" || !strings.Contains(out.Toast, "Only admins") || res.name != "" {
		t.Fatalf("non-admin must be rejected without resolving: %+v %+v", out, res)
	}
}

func TestCallbackAdminIntentional(t *testing.T) {
	h, res := newCB(nil)
	out := h.Handle(alert.CallbackData("Docker app", alert.Intentional), 1, "@yuri")
	if res.name != "Docker app" || res.r != alert.Intentional {
		t.Fatalf("resolver not called correctly: %+v", res)
	}
	if !strings.Contains(out.Note, "@yuri") || !strings.Contains(out.Note, "Stopped by admin") {
		t.Fatalf("unexpected note: %q", out.Note)
	}
}

func TestCallbackAdminIncident(t *testing.T) {
	h, res := newCB(nil)
	out := h.Handle(alert.CallbackData("Docker app", alert.Incident), 1, "@yuri")
	if res.r != alert.Incident || !strings.Contains(out.Note, "crash") {
		t.Fatalf("unexpected: %+v %+v", res, out)
	}
}

func TestCallbackAlreadyRecovered(t *testing.T) {
	h, _ := newCB(alert.ErrNotActive)
	out := h.Handle(alert.CallbackData("Docker app", alert.Intentional), 1, "@yuri")
	if !strings.Contains(out.Toast, "back to normal") || out.Note == "" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestCallbackGarbage(t *testing.T) {
	h, res := newCB(nil)
	out := h.Handle("nonsense", 1, "@yuri")
	if out.Note != "" || res.name != "" {
		t.Fatalf("garbage must be ignored: %+v", out)
	}
}
