package alert

import (
	"bpimon/internal/monitor"
	"fmt"
)

type Memory struct {
	mem monitor.MemoryReader
	max int
}

func NewMemory(m monitor.MemoryReader, max int) *Memory {
	return &Memory{mem: m, max: max}
}

func (a *Memory) Name() string { return "Memory" }

func (a *Memory) Check() (bool, string) {
	s, err := a.mem.Usage()
	if err != nil {
		return false, ""
	}
	if s.UsedPct >= a.max {
		return true, fmt.Sprintf("used %d%%", s.UsedPct)
	}
	return false, ""
}
