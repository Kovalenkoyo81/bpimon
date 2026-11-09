package alert

import (
	"errors"
	"strings"
	"testing"

	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
)

// fires adapts Evaluate for table tests that only care about Firing.
func fires(a Alert) (bool, string) {
	v, msg := a.Evaluate()
	return v == Firing, msg
}

// --- stubs ---

type stubCPU struct{ pct float64 }

func (s stubCPU) Usage() (monitor.CPUStat, error) { return monitor.CPUStat{UsagePct: s.pct}, nil }

type stubCPUTemp struct{ temp float64 }

func (s stubCPUTemp) TempC() (float64, error) { return s.temp, nil }

type stubMem struct{ pct int }

func (s stubMem) Usage() (monitor.MemoryStat, error) { return monitor.MemoryStat{UsedPct: s.pct}, nil }

type stubDisk struct{ disks []monitor.DiskUsage }

func (s stubDisk) Usage() ([]monitor.DiskUsage, error) { return s.disks, nil }

type stubSmart struct{ h monitor.SmartHealth }

func (s stubSmart) Health() (monitor.SmartHealth, error) { return s.h, nil }
func (s stubSmart) DeviceName() string                   { return s.h.Device }

type stubDocker struct{ h monitor.DockerHealth }

func (s stubDocker) Health() (monitor.DockerHealth, error) { return s.h, nil }
func (s stubDocker) ContainerName() string                 { return s.h.Container }

type stubSD struct{ h monitor.SDHealth }

func (s stubSD) Health() (monitor.SDHealth, error) { return s.h, nil }
func (s stubSD) Name() string                      { return "SD " + s.h.Device }

// --- CPU ---

func TestCPUAlert(t *testing.T) {
	tests := []struct {
		pct, max float64
		want     bool
	}{
		{50, 80, false},
		{80, 80, true},
		{90, 80, true},
	}
	for _, tt := range tests {
		a := NewCPU(stubCPU{tt.pct}, tt.max)
		got, _ := fires(a)
		if got != tt.want {
			t.Errorf("CPU %.0f%% max=%.0f: got=%v want=%v", tt.pct, tt.max, got, tt.want)
		}
	}
}

// --- CPU Temp ---

func TestCPUTempAlert(t *testing.T) {
	tests := []struct {
		temp, max float64
		want      bool
	}{
		{60, 70, false},
		{70, 70, true},
		{80, 70, true},
	}
	for _, tt := range tests {
		a := NewCPUTemp(stubCPUTemp{tt.temp}, tt.max)
		got, _ := fires(a)
		if got != tt.want {
			t.Errorf("temp=%.0f max=%.0f: got=%v want=%v", tt.temp, tt.max, got, tt.want)
		}
	}
}

// --- Memory ---

func TestMemoryAlert(t *testing.T) {
	tests := []struct {
		pct, max int
		want     bool
	}{
		{70, 85, false},
		{85, 85, true},
		{95, 85, true},
	}
	for _, tt := range tests {
		a := NewMemory(stubMem{tt.pct}, tt.max)
		got, _ := fires(a)
		if got != tt.want {
			t.Errorf("mem=%d%% max=%d%%: got=%v want=%v", tt.pct, tt.max, got, tt.want)
		}
	}
}

// --- Disk ---

func TestDiskAlertFiresOnWorstPartition(t *testing.T) {
	a := NewDisk(stubDisk{[]monitor.DiskUsage{
		{Mount: "/", UsedPct: 50},
		{Mount: "/data", UsedPct: 90},
	}}, 85)
	fire, msg := fires(a)
	if !fire {
		t.Fatal("expected alert for /data at 90%")
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestDiskNoAlert(t *testing.T) {
	a := NewDisk(stubDisk{[]monitor.DiskUsage{{Mount: "/", UsedPct: 50}}}, 85)
	fire, _ := fires(a)
	if fire {
		t.Fatal("no alert expected at 50%")
	}
}

// --- Smart ---

func TestSmartAlertHealthFailed(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Device: "/dev/sda", Health: "FAILED", Temp: 30, Life: 90}}, 50, 10)
	fire, msg := fires(a)
	if !fire || msg != "FAILED" {
		t.Errorf("expected FAILED alert, got fire=%v msg=%q", fire, msg)
	}
}

func TestSmartAlertHighTemp(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 55, Life: 90}}, 50, 10)
	fire, _ := fires(a)
	if !fire {
		t.Fatal("expected temp alert at 55°C with max=50")
	}
}

func TestSmartAlertLowLife(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 30, Life: 8}}, 50, 10)
	fire, _ := fires(a)
	if !fire {
		t.Fatal("expected life alert at 8% with threshold=10")
	}
}

func TestSmartAlertLifeUnavailable(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 30, Life: -1}}, 50, 10)
	fire, _ := fires(a)
	if fire {
		t.Fatal("no alert expected when life is unavailable (-1)")
	}
}

func TestSmartNoAlert(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 30, Life: 90}}, 50, 10)
	fire, _ := fires(a)
	if fire {
		t.Fatal("no alert expected for healthy disk")
	}
}

func TestSmartAlertTempUnavailable(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: -1, Life: -1}}, 50, 10)
	fire, _ := fires(a)
	if fire {
		t.Fatal("no alert expected when temp is unavailable (-1)")
	}
}

// --- Docker ---

func TestDockerAlertStopped(t *testing.T) {
	a := NewDocker(stubDocker{monitor.DockerHealth{Container: "app", Status: "exited", Running: false}})
	fire, _ := fires(a)
	if !fire {
		t.Fatal("expected alert for stopped container")
	}
}

func TestDockerNoAlertRunning(t *testing.T) {
	a := NewDocker(stubDocker{monitor.DockerHealth{Container: "app", Status: "running", Running: true}})
	fire, _ := fires(a)
	if fire {
		t.Fatal("no alert expected for running container")
	}
}

// --- SD ---

func TestSDAlertErrors(t *testing.T) {
	a := NewSD(stubSD{monitor.SDHealth{Device: "mmcblk0", ErrorCount: 3, LastError: "timeout error"}})
	fire, msg := fires(a)
	if !fire {
		t.Fatal("expected SD error alert")
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestSDNoAlert(t *testing.T) {
	a := NewSD(stubSD{monitor.SDHealth{Device: "mmcblk0", ErrorCount: 0}})
	fire, _ := fires(a)
	if fire {
		t.Fatal("no alert expected with 0 errors")
	}
}

func TestDockerAlertIsAskable(t *testing.T) {
	var a Alert = NewDocker(stubDocker{monitor.DockerHealth{Container: "app"}})
	if _, ok := a.(Askable); !ok {
		t.Fatal("docker alert must be Askable: a stopped container may have been stopped on purpose")
	}
}

type errDocker struct{ err error }

func (s errDocker) Health() (monitor.DockerHealth, error) { return monitor.DockerHealth{}, s.err }
func (s errDocker) ContainerName() string                 { return "app" }

func TestDockerRemovedFires(t *testing.T) {
	a := NewDocker(stubDocker{monitor.DockerHealth{Container: "app", Status: monitor.StatusRemoved}})
	v, msg := a.Evaluate()
	if v != Firing || !strings.Contains(msg, "removed") {
		t.Fatalf("removed container must fire, got %v %q", v, msg)
	}
}

func TestDockerDaemonDownIsUnknown(t *testing.T) {
	a := NewDocker(errDocker{monitor.ErrDaemonUnavailable})
	if v, _ := a.Evaluate(); v != Unknown {
		t.Fatalf("daemon down must be Unknown, got %v", v)
	}
}

type stubDaemon struct{ err error }

func (s stubDaemon) Ping() (string, error) { return "29.1.3", s.err }

func TestDockerDaemonAlert(t *testing.T) {
	if ok, _ := fires(NewDockerDaemon(stubDaemon{})); ok {
		t.Fatal("reachable daemon must not fire")
	}
	if ok, _ := fires(NewDockerDaemon(stubDaemon{errors.New("down")})); !ok {
		t.Fatal("unreachable daemon must fire")
	}
}

type errReader struct{}

func (errReader) Usage() (monitor.CPUStat, error) { return monitor.CPUStat{}, errors.New("boom") }
func (errReader) TempC() (float64, error)         { return 0, errors.New("boom") }
func (errReader) Health() (monitor.SmartHealth, error) {
	return monitor.SmartHealth{}, errors.New("boom")
}
func (errReader) DeviceName() string { return "sda" }

type errMem struct{}

func (errMem) Usage() (monitor.MemoryStat, error) { return monitor.MemoryStat{}, errors.New("boom") }

type errDisk struct{}

func (errDisk) Usage() ([]monitor.DiskUsage, error) { return nil, errors.New("boom") }

type errSD struct{}

func (errSD) Health() (monitor.SDHealth, error) { return monitor.SDHealth{}, errors.New("boom") }
func (errSD) Name() string                      { return "SD x" }

// A failed read must never look like "all good".
func TestReadErrorsAreUnknown(t *testing.T) {
	alerts := []Alert{
		NewCPU(errReader{}, 80),
		NewCPUTemp(errReader{}, 80),
		NewMemory(errMem{}, 80),
		NewDisk(errDisk{}, 80),
		NewSmart(errReader{}, 50, 10),
		NewSD(errSD{}),
		NewDocker(errDocker{errors.New("boom")}),
	}
	for _, a := range alerts {
		v, reason := a.Evaluate()
		if v != Unknown || !strings.Contains(reason, "boom") {
			t.Errorf("%s: read error gave %v %q, want Unknown with reason", a.Name(), v, reason)
		}
	}
}

func TestDockerHeadline(t *testing.T) {
	a := NewDocker(stubDocker{monitor.DockerHealth{Container: "app", Status: "exited"}})
	if a.Headline(true) != "app is down" || a.Headline(false) != "app is back" {
		t.Fatalf("headlines: %q / %q", a.Headline(true), a.Headline(false))
	}
	if _, msg := a.Evaluate(); msg != "status: exited" {
		t.Fatalf("detail: %q", msg)
	}
}
