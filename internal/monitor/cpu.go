package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CPUStat struct {
	UsagePct float64
}

// cpuTimes is a snapshot of the aggregate "cpu" line of /proc/stat.
type cpuTimes struct {
	idle, total uint64
}

func readCPUTimes() (cpuTimes, error) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	line, _, _ := strings.Cut(string(b), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, fmt.Errorf("unexpected /proc/stat format")
	}
	var t cpuTimes
	for i, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		t.total += v
		if i == 3 {
			t.idle = v
		}
	}
	return t, nil
}

// usageBetween is the busy percentage between two snapshots.
func usageBetween(prev, cur cpuTimes) float64 {
	dTotal := cur.total - prev.total
	if cur.total <= prev.total || dTotal == 0 {
		return 0
	}
	dIdle := cur.idle - prev.idle
	if cur.idle < prev.idle || dIdle > dTotal {
		return 0
	}
	return (1 - float64(dIdle)/float64(dTotal)) * 100
}

// CPU measures CPU usage and temperature.
//
// Usage() is for the alert loop: it reports the average since its previous
// call (i.e. over the alert interval). Status() is for on-demand commands
// and measures its own short window, so /cpu never shifts the alert's
// baseline. Both are safe for concurrent use.
type CPU struct {
	mu           sync.Mutex
	prev         cpuTimes
	read         func() (cpuTimes, error)
	statusWindow time.Duration
}

func NewCPU() *CPU {
	return &CPU{read: readCPUTimes, statusWindow: 500 * time.Millisecond}
}

func (c *CPU) Name() string { return "CPU" }

// Usage returns the average usage since the previous Usage call
// (since boot on the first call).
func (c *CPU) Usage() (CPUStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cur, err := c.read()
	if err != nil {
		return CPUStat{}, err
	}
	pct := usageBetween(c.prev, cur)
	c.prev = cur
	return CPUStat{UsagePct: pct}, nil
}

// instantUsage samples usage over statusWindow without touching Usage's state.
func (c *CPU) instantUsage() (float64, error) {
	a, err := c.read()
	if err != nil {
		return 0, err
	}
	time.Sleep(c.statusWindow)
	b, err := c.read()
	if err != nil {
		return 0, err
	}
	return usageBetween(a, b), nil
}

// cpuThermalKeywords lists thermal zone type substrings in priority order.
// Earlier entries win when multiple zones match. Covers x86 (Intel/AMD) and ARM SBCs.
var cpuThermalKeywords = []string{
	"x86_pkg_temp", // Intel package temperature — most accurate on Intel
	"k10temp",      // AMD CPU temperature
	"cpu",          // generic ARM SBC zones
	"soc",
	"ap",
	"arm",
	"pkg",  // matches x86_pkg_temp if not already caught
	"core", // coretemp (Intel per-die) or core zones
	"acpi", // acpitz — broad fallback, lower priority
}

func (c *CPU) TempC() (float64, error) {
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err != nil || len(zones) == 0 {
		return 0, fmt.Errorf("no thermal zones")
	}

	// Scan all zones and pick the one with the highest-priority keyword match.
	bestZone := ""
	bestPriority := len(cpuThermalKeywords)
	for _, tempPath := range zones {
		dir := filepath.Dir(tempPath)
		tb, _ := os.ReadFile(filepath.Join(dir, "type"))
		zoneType := strings.ToLower(strings.TrimSpace(string(tb)))
		for i, kw := range cpuThermalKeywords {
			if i >= bestPriority {
				break
			}
			if strings.Contains(zoneType, kw) {
				bestPriority = i
				bestZone = tempPath
				break
			}
		}
	}
	if bestZone == "" {
		bestZone = zones[0]
	}

	b, err := os.ReadFile(bestZone)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	return float64(v) / 1000, nil
}

// TopProcs returns a short string of the top n CPU-consuming processes,
// e.g. "nginx 45.2%, python 12.1%". Processes below 0.5% are omitted.
// Returns "" if ps is unavailable or nothing is consuming CPU.
func TopProcs(n int) string {
	out, err := output("ps", "-eo", "comm,%cpu", "--sort=-%cpu", "--no-headers", "-w")
	if err != nil {
		return ""
	}
	var parts []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pct, _ := strconv.ParseFloat(fields[len(fields)-1], 64)
		if pct < 0.5 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %.1f%%", fields[0], pct))
		if len(parts) >= n {
			break
		}
	}
	return strings.Join(parts, ", ")
}

func (c *CPU) Status() (string, error) {
	pct, err := c.instantUsage()
	if err != nil {
		return "", err
	}
	line := "🖥 CPU: " + strconv.FormatFloat(pct, 'f', 1, 64) + "%"
	if t, err := c.TempC(); err == nil {
		line += fmt.Sprintf(" | %.1f°C", t)
	}
	if top := TopProcs(3); top != "" {
		line += "\n📋 Top: " + top
	}
	return line, nil
}
