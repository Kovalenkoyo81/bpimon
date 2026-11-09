package bot

import (
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
	"github.com/Kovalenkoyo81/bpimon/internal/system"
)

// --- test doubles ---

type stubCPU struct{ pct float64 }

func (stubCPU) Name() string                       { return "CPU" }
func (stubCPU) Kind() monitor.Kind                 { return monitor.KindCPU }
func (s stubCPU) Sample() (monitor.CPUStat, error) { return monitor.CPUStat{UsagePct: s.pct}, nil }

type stubRAM struct{ pct int }

func (stubRAM) Name() string       { return "RAM" }
func (stubRAM) Kind() monitor.Kind { return monitor.KindRAM }
func (s stubRAM) Usage() (monitor.MemoryStat, error) {
	return monitor.MemoryStat{TotalMB: 1000, FreeMB: uint64(1000 - s.pct*10), UsedPct: s.pct}, nil
}

// mockDockerProvider is a running container, without calling docker.
type mockDockerProvider struct{ container string }

func (m mockDockerProvider) Name() string          { return "Docker " + m.container }
func (m mockDockerProvider) Kind() monitor.Kind    { return monitor.KindDocker }
func (m mockDockerProvider) ContainerName() string { return m.container }
func (m mockDockerProvider) Health() (monitor.DockerHealth, error) {
	return monitor.DockerHealth{Container: m.container, Status: "running", Running: true}, nil
}

// mockSender is thread-safe — needed because execute() writes from a goroutine.
type mockSender struct {
	mu   sync.Mutex
	msgs []string
}

func (m *mockSender) Send(msg format.HTML) error {
	m.mu.Lock()
	m.msgs = append(m.msgs, msg.String())
	m.mu.Unlock()
	return nil
}

func (m *mockSender) first() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.msgs) == 0 {
		return ""
	}
	return m.msgs[0]
}

// fakeSystem records host actions instead of running them.
type fakeSystem struct {
	mu         sync.Mutex
	canPower   bool
	restartErr error
	restarted  []string
	rebooted   bool
}

func (f *fakeSystem) CanPower() bool { return f.canPower }
func (f *fakeSystem) Reboot() error  { f.mu.Lock(); f.rebooted = true; f.mu.Unlock(); return nil }
func (f *fakeSystem) Poweroff() error {
	return nil
}
func (f *fakeSystem) RestartContainer(name string) error {
	f.mu.Lock()
	f.restarted = append(f.restarted, name)
	f.mu.Unlock()
	return f.restartErr
}
func (f *fakeSystem) ContainerState(string) (system.ContainerState, error) {
	return system.ContainerState{Status: "running"}, nil
}

func newHandler(adminIDs []int64, providers []monitor.Provider) *Handler {
	return newHandlerWith(adminIDs, providers, &fakeSystem{canPower: true})
}

func newHandlerWith(adminIDs []int64, providers []monitor.Provider, sys SystemActions) *Handler {
	h := NewHandler(HandlerConfig{Providers: providers, Admins: adminIDs, System: sys})
	h.limiter = NewRateLimiter(10*time.Second, 100)
	return h
}

var confirmRe = regexp.MustCompile(`/confirm (\d+)`)

// confirmCode extracts the confirmation code from a prompt.
func confirmCode(t *testing.T, resp string) string {
	t.Helper()
	m := confirmRe.FindStringSubmatch(resp)
	if m == nil {
		t.Fatalf("no confirmation code in %q", resp)
	}
	return m[1]
}

// --- tests ---

func TestHandleEmpty(t *testing.T) {
	h := newHandler(nil, nil)
	if resp := h.Handle("", 1, &mockSender{}); !resp.IsZero() {
		t.Errorf("expected empty, got %q", resp)
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	h := newHandler(nil, nil)
	resp := h.Handle("/unknown", 1, &mockSender{}).String()
	if !strings.Contains(resp, "Commands:") {
		t.Errorf("expected help text, got %q", resp)
	}
}

func TestHandleStripBotname(t *testing.T) {
	h := newHandler(nil, []monitor.Provider{stubCPU{12.5}})
	resp := h.Handle("/cpu@MyBot", 1, &mockSender{}).String()
	if !strings.Contains(resp, "12.5%") {
		t.Errorf("expected CPU card after strip, got %q", resp)
	}
}

func TestHandleStatusCollectsAll(t *testing.T) {
	providers := []monitor.Provider{stubRAM{43}, stubCPU{12.5}}
	resp := newHandler(nil, providers).Handle("/status", 1, &mockSender{}).String()
	if !strings.Contains(resp, "12.5%") || !strings.Contains(resp, "43%") {
		t.Errorf("/status missing providers: %q", resp)
	}
}

func TestHandleCPUFiltersOtherProviders(t *testing.T) {
	providers := []monitor.Provider{stubCPU{12.5}, stubRAM{43}}
	resp := newHandler(nil, providers).Handle("/cpu", 1, &mockSender{}).String()
	if !strings.Contains(resp, "12.5%") || strings.Contains(resp, "RAM") {
		t.Errorf("/cpu should return only CPU provider: %q", resp)
	}
}

func TestHandleRAM(t *testing.T) {
	providers := []monitor.Provider{stubRAM{43}, stubCPU{12.5}}
	resp := newHandler(nil, providers).Handle("/ram", 1, &mockSender{}).String()
	if !strings.Contains(resp, "43%") || strings.Contains(resp, "CPU") {
		t.Errorf("/ram should return only RAM provider: %q", resp)
	}
}

func TestHandleDocker(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"syncthing"}}
	resp := newHandler(nil, providers).Handle("/docker", 1, &mockSender{}).String()
	if !strings.Contains(resp, "syncthing") {
		t.Errorf("/docker should include container status: %q", resp)
	}
}

func TestHandleRestartNonAdmin(t *testing.T) {
	h := newHandler([]int64{999}, nil)
	resp := h.Handle("/restart", 1, &mockSender{}).String()
	if !strings.Contains(resp, "Only admins") {
		t.Errorf("expected admin error, got %q", resp)
	}
}

func TestHandleRestartMenuAdmin(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	resp := newHandler([]int64{1}, providers).Handle("/restart", 1, &mockSender{}).String()
	if !strings.Contains(resp, "/restart_myapp") {
		t.Errorf("expected container menu, got %q", resp)
	}
}

func TestHandleRestartMenuWithHyphen(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"my-app"}}
	resp := newHandler([]int64{1}, providers).Handle("/restart", 1, &mockSender{}).String()
	if !strings.Contains(resp, "/restart_my_app") {
		t.Errorf("expected hyphen replaced with underscore, got %q", resp)
	}
}

func TestHandleRestartUnderscoreRequestsConfirm(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"my-app"}}
	resp := newHandler([]int64{1}, providers).Handle("/restart_my_app", 1, &mockSender{}).String()
	if !strings.Contains(resp, "/confirm") {
		t.Errorf("expected confirm prompt, got %q", resp)
	}
}

func TestHandleRestartUnknownContainer(t *testing.T) {
	resp := newHandler([]int64{1}, nil).Handle("/restart_unknown", 1, &mockSender{}).String()
	if !strings.Contains(resp, "Unknown container") {
		t.Errorf("expected unknown container error, got %q", resp)
	}
}

func TestHandleRestartNonAdminUnderscore(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	resp := newHandler([]int64{999}, providers).Handle("/restart_myapp", 1, &mockSender{}).String()
	if !strings.Contains(resp, "Only admins") {
		t.Errorf("expected admin error, got %q", resp)
	}
}

func TestHandleConfirmMissingCode(t *testing.T) {
	resp := newHandler(nil, nil).Handle("/confirm", 1, &mockSender{}).String()
	if !strings.Contains(resp, "Usage:") {
		t.Errorf("expected usage hint, got %q", resp)
	}
}

func TestHandleConfirmWrongCode(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	h := newHandler([]int64{1}, providers)
	h.Handle("/restart_myapp", 1, &mockSender{})
	resp := h.Handle("/confirm 9999", 1, &mockSender{}).String()
	if !strings.Contains(resp, "Invalid") {
		t.Errorf("expected invalid code response, got %q", resp)
	}
}

func TestHandleConfirmDockerSendsRestartingMessage(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	h := newHandler([]int64{1}, providers)
	resp := h.Handle("/restart_myapp", 1, &mockSender{}).String()

	code := confirmCode(t, resp)

	s := &mockSender{}
	result := h.Handle("/confirm "+code, 1, s).String()

	// execute() sends "Restarting..." immediately and returns ""
	if result != "" {
		t.Errorf("expected empty return for docker action, got %q", result)
	}
	if !strings.Contains(s.first(), "Restarting") {
		t.Errorf("expected 'Restarting myapp...' as first message, got %q", s.first())
	}
}

// renamedRAM reports RAM under an unusual display name.
type renamedRAM struct{ stubRAM }

func (renamedRAM) Name() string { return "Docker-looking memory" }

// Commands select providers by Kind, not by display name.
func TestCommandsSelectByKindNotName(t *testing.T) {
	h := newHandler(nil, []monitor.Provider{mockDockerProvider{"whale"}, renamedRAM{stubRAM{43}}})
	got := h.Handle("/docker", 1, nil).String()
	if !strings.Contains(got, "whale") || strings.Contains(got, "43%") {
		t.Fatalf("/docker = %q", got)
	}
	if got := h.Handle("/ram", 1, nil).String(); !strings.Contains(got, "43%") {
		t.Fatalf("/ram = %q", got)
	}
}

// Dynamic values are escaped: a hostile container name cannot break the
// HTML that Telegram would otherwise reject.
func TestOutputIsEscaped(t *testing.T) {
	h := newHandlerWith([]int64{1}, []monitor.Provider{mockDockerProvider{"a<b>&c"}}, &fakeSystem{})
	got := h.Handle("/docker", 1, nil).String()
	if strings.Contains(got, "a<b>") || !strings.Contains(got, "a&lt;b&gt;&amp;c") {
		t.Fatalf("container name not escaped: %q", got)
	}
	if got := h.Handle("/restart nope<x>", 1, nil).String(); strings.Contains(got, "<x>") {
		t.Fatalf("argument not escaped: %q", got)
	}
	if got := h.Handle("/help", 1, nil).String(); strings.Contains(got, "<min>") {
		t.Fatalf("menu not escaped: %q", got)
	}
}

func TestConfirmRestartRunsActionAndReports(t *testing.T) {
	sys := &fakeSystem{}
	h := newHandlerWith([]int64{1}, []monitor.Provider{mockDockerProvider{"my-app"}}, sys)
	resp := h.Handle("/restart my-app", 1, &mockSender{}).String()
	code := confirmCode(t, resp)

	s := &mockSender{}
	h.Handle("/confirm "+code, 1, s)
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		n := len(s.msgs)
		s.mu.Unlock()
		if n >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	sys.mu.Lock()
	defer sys.mu.Unlock()
	if len(sys.restarted) != 1 || sys.restarted[0] != "my-app" {
		t.Fatalf("expected restart of my-app, got %v", sys.restarted)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.msgs) < 2 || !strings.Contains(s.msgs[1], "✅ <b>my-app</b> restarted") {
		t.Fatalf("expected success report, got %v", s.msgs)
	}
}

func TestRebootUnavailableWithoutSystemctl(t *testing.T) {
	h := newHandlerWith([]int64{1}, nil, &fakeSystem{canPower: false})
	if got := h.Handle("/reboot", 1, nil).String(); !strings.Contains(got, "unavailable") {
		t.Fatalf("/reboot = %q", got)
	}
}

func TestConfirmRebootCallsSystem(t *testing.T) {
	sys := &fakeSystem{canPower: true}
	h := newHandlerWith([]int64{1}, nil, sys)
	resp := h.Handle("/reboot", 1, nil).String()
	code := confirmCode(t, resp)
	if got := h.Handle("/confirm "+code, 1, nil).String(); !strings.Contains(got, "reboot initiated") {
		t.Fatalf("/confirm = %q", got)
	}
	if !sys.rebooted {
		t.Fatal("Reboot was not called")
	}
}

func TestParseCommand(t *testing.T) {
	cases := map[string]string{
		"/Reboot@MyBot":            "/reboot",
		"/restart@MyBot syncthing": "/restart syncthing",
		"/silence 30":              "/silence 30",
		"/x a@b":                   "/x a@b",
		"  ":                       "",
	}
	for in, want := range cases {
		if got := strings.Join(parseCommand(in), " "); got != want {
			t.Errorf("parseCommand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMemoryAmount(t *testing.T) {
	if got := memoryAmount(413, 958); got != "413 / 958 MB" {
		t.Errorf("small board: %q", got)
	}
	if got := memoryAmount(7795, 13393); got != "7.6 / 13.1 GB" {
		t.Errorf("large machine: %q", got)
	}
}
