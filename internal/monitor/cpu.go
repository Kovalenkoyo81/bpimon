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

var cpuThermalKeywords = []string{"cpu", "soc", "pkg", "core", "ap", "arm"}

func (c *CPU) TempC() (float64, error) {
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err != nil || len(zones) == 0 {
		return 0, fmt.Errorf("no thermal zones")
	}
	best := ""
	for _, tempPath := range zones {
		dir := filepath.Dir(tempPath)
		tb, _ := os.ReadFile(filepath.Join(dir, "type"))
		zoneType := strings.ToLower(strings.TrimSpace(string(tb)))
		for _, kw := range cpuThermalKeywords {
			if strings.Contains(zoneType, kw) {
				best = tempPath
				break
			}
		}
		if best != "" {
			break
		}
	}
	if best == "" {
		best = zones[0]
	}
	b, err := os.ReadFile(best)
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
