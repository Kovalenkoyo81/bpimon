package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CPUStat struct {
	UsagePct float64
}

type CPU struct {
	prevIdle  uint64
	prevTotal uint64
}

func NewCPU() *CPU {
	return &CPU{}
}

func (c *CPU) Name() string { return "CPU" }

func (c *CPU) Usage() (CPUStat, error) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return CPUStat{}, err
	}

	fields := strings.Fields(strings.Split(string(b), "\n")[0])[1:]
	var idle, total uint64
	for i, f := range fields {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
		if i == 3 {
			idle = v
		}
	}

	diffIdle := idle - c.prevIdle
	diffTotal := total - c.prevTotal
	c.prevIdle = idle
	c.prevTotal = total

	if diffTotal == 0 {
		return CPUStat{}, nil
	}

	return CPUStat{UsagePct: (1 - float64(diffIdle)/float64(diffTotal)) * 100}, nil
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
	"pkg",          // matches x86_pkg_temp if not already caught
	"core",         // coretemp (Intel per-die) or core zones
	"acpi",         // acpitz — broad fallback, lower priority
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

func (c *CPU) Status() (string, error) {
	s, err := c.Usage()
	if err != nil {
		return "", err
	}
	line := "🖥 CPU: " + strconv.FormatFloat(s.UsagePct, 'f', 1, 64) + "%"
	if t, err := c.TempC(); err == nil {
		line += fmt.Sprintf(" | %.1f°C", t)
	}
	return line, nil
}
