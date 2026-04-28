package alert

import (
	"bpimon/internal/notifier"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubAlert lets tests control what Check() returns.
type stubAlert struct {
	name   string
	firing bool
	msg    string
}

func (s *stubAlert) Name() string         { return s.name }
func (s *stubAlert) Check() (bool, string) { return s.firing, s.msg }

// collectingNotifier captures sent messages.
type collectingNotifier struct {
	mu   sync.Mutex
	msgs []string
}

func (n *collectingNotifier) Send(text string) error {
	n.mu.Lock()
	n.msgs = append(n.msgs, text)
	n.mu.Unlock()
	return nil
}

func (n *collectingNotifier) SendAlert(text string) error {
	return n.Send(text)
}

func (n *collectingNotifier) all() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]string, len(n.msgs))
	copy(out, n.msgs)
	return out
}

func (n *collectingNotifier) contains(sub string) bool {
	for _, m := range n.all() {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

func runTick(m *Manager) {
	if m.IsSilenced() {
		return
	}
	for _, a := range m.alerts {
		name := a.Name()
		ok, msg := a.Check()
		if ok {
			if m.lastSent[name].IsZero() || time.Since(m.lastSent[name]) >= m.cooldown {
				_ = m.notifier.Send("🚨 " + name + ": " + msg)
				m.lastSent[name] = time.Now()
				m.active[name] = true
			}
		} else if m.active[name] {
			_ = m.notifier.Send("✅ " + name + ": back to normal")
			m.active[name] = false
		}
	}
}

func TestManagerFiresAlert(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: true, msg: "usage 95%"}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Millisecond)

	runTick(m)

	if !n.contains("🚨 CPU") {
		t.Fatalf("expected alert message, got %v", n.all())
	}
}

func TestManagerRecoveryAfterAlert(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: true, msg: "usage 95%"}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Millisecond)

	runTick(m) // fires alert, sets active
	a.firing = false
	runTick(m) // condition cleared → recovery

	if !n.contains("✅ CPU") {
		t.Fatalf("expected recovery message, got %v", n.all())
	}
}

func TestManagerNoRecoveryWithoutPriorAlert(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: false}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Millisecond)

	runTick(m) // never fired → no recovery

	if n.contains("✅") {
		t.Fatalf("unexpected recovery message: %v", n.all())
	}
}

func TestManagerRecoverySentOnce(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: true, msg: "usage 95%"}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Millisecond)

	runTick(m)
	a.firing = false
	runTick(m) // recovery sent
	runTick(m) // still not firing — no second recovery

	count := 0
	for _, msg := range n.all() {
		if strings.Contains(msg, "✅ CPU") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 recovery message, got %d: %v", count, n.all())
	}
}

func TestManagerCooldownSuppressDuplicateAlerts(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: true, msg: "usage 95%"}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Hour) // long cooldown

	runTick(m)
	runTick(m) // still firing but in cooldown

	count := 0
	for _, msg := range n.all() {
		if strings.Contains(msg, "🚨 CPU") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 alert (cooldown), got %d: %v", count, n.all())
	}
}

func TestManagerNotifier(t *testing.T) {
	_ = notifier.Dummy{} // ensure Dummy satisfies notifier.Notifier
}
