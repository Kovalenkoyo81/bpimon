package bot

import (
	"bpimon/internal/log"
	"bpimon/internal/monitor"
	"strings"
	"sync"
	"testing"
	"time"
)

func init() {
	log.Init()
}

// --- test doubles ---

type mockProvider struct{ name, status string }

func (m mockProvider) Name() string            { return m.name }
func (m mockProvider) Status() (string, error) { return m.status, nil }

// mockDockerProvider satisfies monitor.ContainerProvider without calling docker.
type mockDockerProvider struct{ container string }

func (m mockDockerProvider) Name() string            { return "Docker " + m.container }
func (m mockDockerProvider) Status() (string, error) { return "🐳 " + m.container + " — ✅ running", nil }
func (m mockDockerProvider) ContainerName() string   { return m.container }

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

func newHandler(adminIDs []int64, providers []monitor.Provider) *Handler {
	return &Handler{
		Providers: providers,
		Admins:    adminIDs,
		confirm:   NewConfirmator(60 * time.Second),
		limiter:   NewRateLimiter(10*time.Second, 100),
	}
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
