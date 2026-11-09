package bot

import (
	"bpimon/internal/monitor"
	"bpimon/internal/system"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- test doubles ---

type mockProvider struct{ name, status string }

func (m mockProvider) Name() string { return m.name }
func (m mockProvider) Kind() monitor.Kind {
	switch strings.Fields(m.name)[0] {
	case "CPU":
		return monitor.KindCPU
	case "RAM":
		return monitor.KindRAM
	case "Disk":
		return monitor.KindDisk
	case "SMART":
		return monitor.KindSmart
	case "SD":
		return monitor.KindSD
	}
	return 0
}
func (m mockProvider) Status() (string, error) { return m.status, nil }

// mockDockerProvider satisfies monitor.ContainerProvider without calling docker.
type mockDockerProvider struct{ container string }

func (m mockDockerProvider) Name() string       { return "Docker " + m.container }
func (m mockDockerProvider) Kind() monitor.Kind { return monitor.KindDocker }
func (m mockDockerProvider) Status() (string, error) {
	return "🐳 " + m.container + " — ✅ running", nil
}
func (m mockDockerProvider) ContainerName() string { return m.container }

// mockSender is thread-safe — needed because execute() writes from a goroutine.
type mockSender struct {
	mu   sync.Mutex
	msgs []string
}

func (m *mockSender) Send(text string) error {
	m.mu.Lock()
	m.msgs = append(m.msgs, text)
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

// --- tests ---

func TestHandleEmpty(t *testing.T) {
	h := newHandler(nil, nil)
	if resp := h.Handle("", 1, &mockSender{}); resp != "" {
		t.Errorf("expected empty, got %q", resp)
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	h := newHandler(nil, nil)
	resp := h.Handle("/unknown", 1, &mockSender{})
	if !strings.Contains(resp, "Commands:") {
		t.Errorf("expected help text, got %q", resp)
	}
}

func TestHandleStripBotname(t *testing.T) {
	h := newHandler(nil, []monitor.Provider{mockProvider{"CPU", "cpu_status"}})
	resp := h.Handle("/cpu@MyBot", 1, &mockSender{})
	if resp != "cpu_status" {
		t.Errorf("expected cpu_status after strip, got %q", resp)
	}
}

func TestHandleStatusCollectsAll(t *testing.T) {
	providers := []monitor.Provider{
		mockProvider{"CPU", "cpu_out"},
		mockProvider{"RAM", "ram_out"},
	}
	resp := newHandler(nil, providers).Handle("/status", 1, &mockSender{})
	if !strings.Contains(resp, "cpu_out") || !strings.Contains(resp, "ram_out") {
		t.Errorf("/status missing providers: %q", resp)
	}
}

func TestHandleCPUFiltersOtherProviders(t *testing.T) {
	providers := []monitor.Provider{
		mockProvider{"CPU", "cpu_line"},
		mockProvider{"RAM", "ram_line"},
	}
	resp := newHandler(nil, providers).Handle("/cpu", 1, &mockSender{})
	if !strings.Contains(resp, "cpu_line") || strings.Contains(resp, "ram_line") {
		t.Errorf("/cpu should return only CPU provider: %q", resp)
	}
}

func TestHandleRAM(t *testing.T) {
	providers := []monitor.Provider{mockProvider{"RAM", "ram_line"}, mockProvider{"CPU", "cpu_line"}}
	resp := newHandler(nil, providers).Handle("/ram", 1, &mockSender{})
	if !strings.Contains(resp, "ram_line") || strings.Contains(resp, "cpu_line") {
		t.Errorf("/ram should return only RAM provider: %q", resp)
	}
}

func TestHandleDocker(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"syncthing"}}
	resp := newHandler(nil, providers).Handle("/docker", 1, &mockSender{})
	if !strings.Contains(resp, "syncthing") {
		t.Errorf("/docker should include container status: %q", resp)
	}
}

func TestHandleRestartNonAdmin(t *testing.T) {
	h := newHandler([]int64{999}, nil)
	resp := h.Handle("/restart", 1, &mockSender{})
	if !strings.Contains(resp, "Only admins") {
		t.Errorf("expected admin error, got %q", resp)
	}
}

func TestHandleRestartMenuAdmin(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	resp := newHandler([]int64{1}, providers).Handle("/restart", 1, &mockSender{})
	if !strings.Contains(resp, "/restart_myapp") {
		t.Errorf("expected container menu, got %q", resp)
	}
}

func TestHandleRestartMenuWithHyphen(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"my-app"}}
	resp := newHandler([]int64{1}, providers).Handle("/restart", 1, &mockSender{})
	if !strings.Contains(resp, "/restart_my_app") {
		t.Errorf("expected hyphen replaced with underscore, got %q", resp)
	}
}

func TestHandleRestartUnderscoreRequestsConfirm(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"my-app"}}
	resp := newHandler([]int64{1}, providers).Handle("/restart_my_app", 1, &mockSender{})
	if !strings.Contains(resp, "/confirm") {
		t.Errorf("expected confirm prompt, got %q", resp)
	}
}

func TestHandleRestartUnknownContainer(t *testing.T) {
	resp := newHandler([]int64{1}, nil).Handle("/restart_unknown", 1, &mockSender{})
	if !strings.Contains(resp, "Unknown container") {
		t.Errorf("expected unknown container error, got %q", resp)
	}
}

func TestHandleRestartNonAdminUnderscore(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	resp := newHandler([]int64{999}, providers).Handle("/restart_myapp", 1, &mockSender{})
	if !strings.Contains(resp, "Only admins") {
		t.Errorf("expected admin error, got %q", resp)
	}
}

func TestHandleConfirmMissingCode(t *testing.T) {
	resp := newHandler(nil, nil).Handle("/confirm", 1, &mockSender{})
	if !strings.Contains(resp, "Usage:") {
		t.Errorf("expected usage hint, got %q", resp)
	}
}

func TestHandleConfirmWrongCode(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	h := newHandler([]int64{1}, providers)
	h.Handle("/restart_myapp", 1, &mockSender{})
	resp := h.Handle("/confirm 9999", 1, &mockSender{})
	if !strings.Contains(resp, "Invalid") {
		t.Errorf("expected invalid code response, got %q", resp)
	}
}

func TestHandleConfirmDockerSendsRestartingMessage(t *testing.T) {
	providers := []monitor.Provider{mockDockerProvider{"myapp"}}
	h := newHandler([]int64{1}, providers)
	resp := h.Handle("/restart_myapp", 1, &mockSender{})

	start := strings.Index(resp, "/confirm ") + len("/confirm ")
	end := strings.Index(resp[start:], "`")
	code := resp[start : start+end]

	s := &mockSender{}
	result := h.Handle("/confirm "+code, 1, s)

	// execute() sends "Restarting..." immediately and returns ""
	if result != "" {
		t.Errorf("expected empty return for docker action, got %q", result)
	}
	if !strings.Contains(s.first(), "Restarting") {
		t.Errorf("expected 'Restarting myapp...' as first message, got %q", s.first())
	}
}

type kindProvider struct {
	kind         monitor.Kind
	name, status string
}

func (k kindProvider) Name() string            { return k.name }
func (k kindProvider) Kind() monitor.Kind      { return k.kind }
func (k kindProvider) Status() (string, error) { return k.status, nil }

// Commands select providers by Kind, not by display name.
func TestCommandsSelectByKindNotName(t *testing.T) {
	h := newHandler(nil, []monitor.Provider{
		kindProvider{monitor.KindDocker, "Whale engine", "daemon_ok"},
		kindProvider{monitor.KindRAM, "Docker-looking RAM", "ram_ok"},
	})
	got := h.Handle("/docker", 1, nil)
	if !strings.Contains(got, "daemon_ok") || strings.Contains(got, "ram_ok") {
		t.Fatalf("/docker = %q", got)
	}
	if got := h.Handle("/ram", 1, nil); got != "ram_ok" {
		t.Fatalf("/ram = %q", got)
	}
}

func TestConfirmRestartRunsActionAndReports(t *testing.T) {
	sys := &fakeSystem{}
	h := newHandlerWith([]int64{1}, []monitor.Provider{mockDockerProvider{"my-app"}}, sys)
	resp := h.Handle("/restart my-app", 1, &mockSender{})
	code := strings.TrimSuffix(strings.Fields(resp[strings.Index(resp, "/confirm "):])[1], "`")

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
	if len(s.msgs) < 2 || !strings.Contains(s.msgs[1], "✅ Status: running") {
		t.Fatalf("expected success report, got %v", s.msgs)
	}
}

func TestRebootUnavailableWithoutSystemctl(t *testing.T) {
	h := newHandlerWith([]int64{1}, nil, &fakeSystem{canPower: false})
	if got := h.Handle("/reboot", 1, nil); !strings.Contains(got, "unavailable") {
		t.Fatalf("/reboot = %q", got)
	}
}

func TestConfirmRebootCallsSystem(t *testing.T) {
	sys := &fakeSystem{canPower: true}
	h := newHandlerWith([]int64{1}, nil, sys)
	resp := h.Handle("/reboot", 1, nil)
	code := strings.Fields(resp[strings.Index(resp, "/confirm "):])[1]
	code = strings.Trim(code, "`")
	if got := h.Handle("/confirm "+code, 1, nil); !strings.Contains(got, "reboot initiated") {
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
