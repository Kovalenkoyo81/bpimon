package alert

import (
	"bpimon/internal/monitor"
	"testing"
)

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
		got, _ := a.Check()
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
		got, _ := a.Check()
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
		got, _ := a.Check()
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
	fire, msg := a.Check()
	if !fire {
		t.Fatal("expected alert for /data at 90%")
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestDiskNoAlert(t *testing.T) {
	a := NewDisk(stubDisk{[]monitor.DiskUsage{{Mount: "/", UsedPct: 50}}}, 85)
	fire, _ := a.Check()
	if fire {
		t.Fatal("no alert expected at 50%")
	}
}

// --- Smart ---

func TestSmartAlertHealthFailed(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Device: "/dev/sda", Health: "FAILED", Temp: 30, Life: 90}}, 50, 10)
	fire, msg := a.Check()
	if !fire || msg != "FAILED" {
		t.Errorf("expected FAILED alert, got fire=%v msg=%q", fire, msg)
	}
}

func TestSmartAlertHighTemp(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 55, Life: 90}}, 50, 10)
	fire, _ := a.Check()
	if !fire {
		t.Fatal("expected temp alert at 55°C with max=50")
	}
}

func TestSmartAlertLowLife(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 30, Life: 8}}, 50, 10)
	fire, _ := a.Check()
	if !fire {
		t.Fatal("expected life alert at 8% with threshold=10")
	}
}

func TestSmartAlertLifeUnavailable(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 30, Life: -1}}, 50, 10)
	fire, _ := a.Check()
	if fire {
		t.Fatal("no alert expected when life is unavailable (-1)")
	}
}

func TestSmartNoAlert(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: 30, Life: 90}}, 50, 10)
	fire, _ := a.Check()
	if fire {
		t.Fatal("no alert expected for healthy disk")
	}
}

func TestSmartAlertTempUnavailable(t *testing.T) {
	a := NewSmart(stubSmart{monitor.SmartHealth{Health: "PASSED", Temp: -1, Life: -1}}, 50, 10)
	fire, _ := a.Check()
	if fire {
		t.Fatal("no alert expected when temp is unavailable (-1)")
	}
}

// --- Docker ---

func TestDockerAlertStopped(t *testing.T) {
	a := NewDocker(stubDocker{monitor.DockerHealth{Container: "app", Status: "exited", Running: false}})
	fire, _ := a.Check()
	if !fire {
		t.Fatal("expected alert for stopped container")
	}
}

func TestDockerNoAlertRunning(t *testing.T) {
	a := NewDocker(stubDocker{monitor.DockerHealth{Container: "app", Status: "running", Running: true}})
	fire, _ := a.Check()
	if fire {
		t.Fatal("no alert expected for running container")
	}
}

// --- SD ---

func TestSDAlertErrors(t *testing.T) {
	a := NewSD(stubSD{monitor.SDHealth{Device: "mmcblk0", ErrorCount: 3, LastError: "timeout error"}})
	fire, msg := a.Check()
	if !fire {
		t.Fatal("expected SD error alert")
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestSDNoAlert(t *testing.T) {
	a := NewSD(stubSD{monitor.SDHealth{Device: "mmcblk0", ErrorCount: 0}})
	fire, _ := a.Check()
	if fire {
		t.Fatal("no alert expected with 0 errors")
	}
}
