package alert

import (
	"bpimon/internal/notifier"
	"context"
	"errors"
	"os"
	"path/filepath"
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

func (s *stubAlert) Name() string          { return s.name }
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
	m.checkAll()
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

type askStub struct{ stubAlert }

func (*askStub) Choices() (string, string) { return "admin", "crash" }

// buttonNotifier records which messages carried buttons and which
// messages had their buttons cleared. Message IDs start at 1.
type buttonNotifier struct {
	collectingNotifier
	withButtons []string
	buttons     [][]notifier.Button
	cleared     []int
}

var _ notifier.ButtonSender = (*buttonNotifier)(nil)

func (n *buttonNotifier) SendAlertWithButtons(text string, b []notifier.Button) (int, error) {
	n.mu.Lock()
	n.withButtons = append(n.withButtons, text)
	n.buttons = append(n.buttons, b)
	id := len(n.withButtons)
	n.mu.Unlock()
	return id, n.Send(text)
}

func (n *buttonNotifier) ClearButtons(id int) error {
	n.mu.Lock()
	n.cleared = append(n.cleared, id)
	n.mu.Unlock()
	return nil
}

func countPrefix(n *collectingNotifier, sub string) int {
	c := 0
	for _, msg := range n.all() {
		if strings.Contains(msg, sub) {
			c++
		}
	}
	return c
}

func newAskSetup(cooldown time.Duration) (*askStub, *buttonNotifier, *Manager) {
	a := &askStub{stubAlert{name: "Docker app", firing: true, msg: "exited"}}
	n := &buttonNotifier{}
	return a, n, NewManager([]Alert{a}, n, time.Minute, cooldown)
}

func TestAskableSendsButtons(t *testing.T) {
	_, n, m := newAskSetup(time.Hour)
	runTick(m)

	if len(n.buttons) != 1 || len(n.buttons[0]) != 2 {
		t.Fatalf("expected one message with 2 buttons, got %v", n.buttons)
	}
	name, r, ok := ParseCallbackData(n.buttons[0][0].Data)
	if !ok || name != "Docker app" || r != Intentional {
		t.Fatalf("bad intentional button data %q", n.buttons[0][0].Data)
	}
	name, r, ok = ParseCallbackData(n.buttons[0][1].Data)
	if !ok || name != "Docker app" || r != Incident {
		t.Fatalf("bad incident button data %q", n.buttons[0][1].Data)
	}
}

func TestAskableUnansweredRepeatsWithButtons(t *testing.T) {
	_, n, m := newAskSetup(0)
	runTick(m)
	runTick(m)

	if len(n.withButtons) != 2 {
		t.Fatalf("expected question repeated with buttons, got %d: %v", len(n.withButtons), n.all())
	}
}

func TestAskableIntentionalSilencesUntilRecovery(t *testing.T) {
	a, n, m := newAskSetup(0)
	runTick(m)
	if err := m.Resolve("Docker app", Intentional); err != nil {
		t.Fatal(err)
	}
	runTick(m)
	runTick(m)
	if c := countPrefix(&n.collectingNotifier, "🚨"); c != 1 {
		t.Fatalf("expected silence after Intentional, got %d alerts: %v", c, n.all())
	}

	a.firing = false
	runTick(m)
	if countPrefix(&n.collectingNotifier, "✅") != 0 {
		t.Fatalf("no recovery message expected after Intentional: %v", n.all())
	}

	a.firing = true
	runTick(m) // new incident: resolution was reset, ask again
	if len(n.withButtons) != 2 {
		t.Fatalf("expected a new question after recovery, got %d: %v", len(n.withButtons), n.all())
	}
}

func TestAskableIncidentRemindsWithoutButtons(t *testing.T) {
	a, n, m := newAskSetup(0)
	runTick(m)
	if err := m.Resolve("Docker app", Incident); err != nil {
		t.Fatal(err)
	}
	runTick(m)

	if len(n.withButtons) != 1 {
		t.Fatalf("reminder after Incident must not carry buttons: %v", n.withButtons)
	}
	if c := countPrefix(&n.collectingNotifier, "🚨"); c != 2 {
		t.Fatalf("expected reminder after Incident, got %d: %v", c, n.all())
	}

	a.firing = false
	runTick(m)
	if countPrefix(&n.collectingNotifier, "✅") != 1 {
		t.Fatalf("expected recovery message after Incident: %v", n.all())
	}
}

func TestAskableNewIncidentIgnoresCooldown(t *testing.T) {
	a, n, m := newAskSetup(time.Hour)
	runTick(m)
	a.firing = false
	runTick(m)
	a.firing = true
	runTick(m)

	if len(n.withButtons) != 2 {
		t.Fatalf("new incident must be asked even within cooldown, got %d: %v", len(n.withButtons), n.all())
	}
}

func TestResolveInactiveButStillFiringIsAccepted(t *testing.T) {
	// e.g. right after a restart, before the first check
	_, n, m := newAskSetup(0)
	if err := m.Resolve("Docker app", Intentional); err != nil {
		t.Fatalf("firing alert must be resolvable even if not marked active: %v", err)
	}
	runTick(m)
	if len(n.all()) != 0 {
		t.Fatalf("resolved as Intentional — nothing should be sent: %v", n.all())
	}
}

func TestResolveInactiveAndClear(t *testing.T) {
	a, _, m := newAskSetup(0)
	a.firing = false
	if err := m.Resolve("Docker app", Intentional); !errors.Is(err, ErrNotActive) {
		t.Fatalf("expected ErrNotActive, got %v", err)
	}
}

func TestResolveInactiveAndUnknown(t *testing.T) {
	a := &verdictStub{name: "Docker app", v: Unknown}
	m := NewManager([]Alert{a}, &buttonNotifier{}, time.Minute, 0)
	if err := m.Resolve("Docker app", Intentional); !errors.Is(err, ErrUnknown) {
		t.Fatalf("expected ErrUnknown, got %v", err)
	}
}

func TestOnlyLatestQuestionKeepsButtons(t *testing.T) {
	_, n, m := newAskSetup(0)
	runTick(m) // question #1
	runTick(m) // question #2 → clears #1
	runTick(m) // question #3 → clears #2
	if len(n.cleared) != 2 || n.cleared[0] != 1 || n.cleared[1] != 2 {
		t.Fatalf("expected buttons cleared on #1 and #2, got %v", n.cleared)
	}
}

func TestRecoveryClearsButtons(t *testing.T) {
	a, n, m := newAskSetup(time.Hour)
	runTick(m) // question #1
	a.firing = false
	runTick(m)
	if len(n.cleared) != 1 || n.cleared[0] != 1 {
		t.Fatalf("recovery must clear the open question's buttons, got %v", n.cleared)
	}
}

func TestResolveClearsLatestQuestion(t *testing.T) {
	_, n, m := newAskSetup(0)
	runTick(m) // #1
	runTick(m) // #2, clears #1
	if err := m.Resolve("Docker app", Incident); err != nil {
		t.Fatal(err)
	}
	if n.cleared[len(n.cleared)-1] != 2 {
		t.Fatalf("resolve must clear the latest question #2, got %v", n.cleared)
	}
	runTick(m) // reminder without buttons, nothing more to clear
	if len(n.cleared) != 2 {
		t.Fatalf("unexpected extra clears: %v", n.cleared)
	}
}

func TestAskableFallsBackWithoutButtonSender(t *testing.T) {
	a := &askStub{stubAlert{name: "Docker app", firing: true, msg: "exited"}}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, time.Hour)
	runTick(m)
	if countPrefix(n, "🚨 Docker app") != 1 {
		t.Fatalf("expected plain alert fallback: %v", n.all())
	}
}

func TestLongNameTruncatedAndResolvedByPrefix(t *testing.T) {
	long := "Docker " + strings.Repeat("x", 80)
	a := &askStub{stubAlert{name: long, firing: true, msg: "exited"}}
	n := &buttonNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, 0)
	runTick(m)

	if len(n.buttons) != 1 {
		t.Fatalf("long name must still get buttons: %v", n.all())
	}
	for _, b := range n.buttons[0] {
		if len(b.Data) > maxCallbackData {
			t.Fatalf("callback data %d bytes > %d", len(b.Data), maxCallbackData)
		}
	}
	name, r, _ := ParseCallbackData(n.buttons[0][0].Data)
	if err := m.Resolve(name, r); err != nil {
		t.Fatalf("resolve by truncated name: %v", err)
	}
	runTick(m)
	if c := countPrefix(&n.collectingNotifier, "🚨"); c != 1 {
		t.Fatalf("expected silence after resolve, got %d", c)
	}
}

func TestResolveAmbiguousPrefix(t *testing.T) {
	a1 := &askStub{stubAlert{name: "Docker app-one", firing: true}}
	a2 := &askStub{stubAlert{name: "Docker app-two", firing: true}}
	m := NewManager([]Alert{a1, a2}, &buttonNotifier{}, time.Minute, 0)
	runTick(m)
	if err := m.Resolve("Docker app-", Intentional); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("expected ErrAmbiguous, got %v", err)
	}
	if err := m.Resolve("Docker app-one", Intentional); err != nil {
		t.Fatalf("exact name must win: %v", err)
	}
}

func TestTruncateUTF8KeepsRunes(t *testing.T) {
	if got := truncateUTF8("abcд", 4); got != "abc" {
		t.Fatalf("must not split a rune, got %q", got)
	}
}

// verdictStub drives the manager through Evaluator.
type verdictStub struct {
	name string
	v    Verdict
}

func (s *verdictStub) Name() string                { return s.name }
func (s *verdictStub) Check() (bool, string)       { return s.v == Firing, "" }
func (s *verdictStub) Evaluate() (Verdict, string) { return s.v, "down" }

func TestUnknownKeepsActiveStateWithoutMessages(t *testing.T) {
	a := &verdictStub{name: "X", v: Firing}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, 0)
	runTick(m) // fires

	a.v = Unknown
	runTick(m)
	runTick(m)
	if len(n.all()) != 1 {
		t.Fatalf("Unknown must send nothing (no recovery, no reminder): %v", n.all())
	}
	if !m.active["X"] {
		t.Fatal("Unknown must keep the alert active")
	}

	a.v = Clear
	runTick(m)
	if countPrefix(n, "✅ X") != 1 {
		t.Fatalf("recovery expected once state is known again: %v", n.all())
	}
}

func TestUnknownDoesNotFire(t *testing.T) {
	a := &verdictStub{name: "X", v: Unknown}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, 0)
	runTick(m)
	if len(n.all()) != 0 {
		t.Fatalf("Unknown on a clear alert must send nothing: %v", n.all())
	}
}

func TestStateSavedOnChangeAndRestored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	_, _, m := newAskSetup(time.Hour)
	m.SetStatePath(path)
	runTick(m) // fires → saved immediately, no shutdown needed
	if err := m.Resolve("Docker app", Intentional); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state must be written on change: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("expected only the state file, no temp leftovers: %v", entries)
	}

	// "restart": a fresh manager with the same state path
	_, n2, m2 := newAskSetup(0)
	m2.SetStatePath(path)
	m2.loadState()
	runTick(m2)
	if len(n2.all()) != 0 {
		t.Fatalf("Intentional answer must survive a restart, got %v", n2.all())
	}
}

func TestLoadStateMissingOrCorrupt(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(nil, &collectingNotifier{}, time.Minute, 0)
	m.SetStatePath(filepath.Join(dir, "missing.json"))
	m.loadState() // must not panic

	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte("{not json"), 0600)
	m.SetStatePath(bad)
	m.loadState()
	if len(m.active) != 0 {
		t.Fatal("corrupt state must be ignored")
	}
}

func TestRunSavesStateOnShutdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	m := NewManager(nil, &collectingNotifier{}, time.Hour, 0)
	m.SetStatePath(path)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	cancel()
	<-done
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state must be saved when Run returns: %v", err)
	}
}

func TestCallbackDataRoundTrip(t *testing.T) {
	for _, r := range []Resolution{Intentional, Incident} {
		name, got, ok := ParseCallbackData(CallbackData("Docker my-app", r))
		if !ok || name != "Docker my-app" || got != r {
			t.Fatalf("round trip failed for %v: %q %v %v", r, name, got, ok)
		}
	}
	for _, bad := range []string{"", "res:", "res:x:foo", "res:i", "other:i:foo"} {
		if _, _, ok := ParseCallbackData(bad); ok {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestManagerRepeatingAlertRefiresAfterCooldown(t *testing.T) {
	a := &stubAlert{name: "CPU", firing: true, msg: "usage 95%"}
	n := &collectingNotifier{}
	m := NewManager([]Alert{a}, n, time.Minute, 0)

	runTick(m)
	runTick(m)

	if c := countPrefix(n, "🚨 CPU"); c != 2 {
		t.Fatalf("expected repeating alert to re-fire, got %d: %v", c, n.all())
	}
}

func TestManagerNotifier(t *testing.T) {
	_ = notifier.Dummy{} // ensure Dummy satisfies notifier.Notifier
}
