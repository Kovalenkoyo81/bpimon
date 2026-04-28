package alert

import (
	"testing"
	"time"
)

func TestSilenceBlocksAlerts(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: true, msg: "usage 95%"}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Millisecond)

	m.Silence(time.Hour)
	runTick(m) // silenced — nothing should be sent

	if len(n.all()) != 0 {
		t.Fatalf("expected no messages while silenced, got %v", n.all())
	}
}

func TestUnsilenceResumesAlerts(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: true, msg: "usage 95%"}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Millisecond)

	m.Silence(time.Hour)
	runTick(m) // silenced — no alert

	m.Unsilence()
	runTick(m) // resumed — alert fires

	if !n.contains("🚨 CPU") {
		t.Fatalf("expected alert after unsilence, got %v", n.all())
	}
}

func TestIsSilencedExpires(t *testing.T) {
	m := NewManager(nil, &collectingNotifier{}, time.Minute, time.Millisecond)
	m.Silence(10 * time.Millisecond)

	if !m.IsSilenced() {
		t.Fatal("expected silenced immediately after Silence()")
	}
	time.Sleep(20 * time.Millisecond)
	if m.IsSilenced() {
		t.Fatal("expected silence to expire")
	}
}

func TestIsSilencedFalseByDefault(t *testing.T) {
	m := NewManager(nil, &collectingNotifier{}, time.Minute, time.Millisecond)
	if m.IsSilenced() {
		t.Fatal("new Manager should not be silenced")
	}
}
