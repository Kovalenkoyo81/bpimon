package alert

import (
	"fmt"

	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
)

type Memory struct {
	mem monitor.MemoryReader
	max int
}

func NewMemory(m monitor.MemoryReader, max int) *Memory {
	return &Memory{mem: m, max: max}
}

func (a *Memory) Name() string { return "Memory" }

func (a *Memory) Evaluate() (Verdict, string) {
	s, err := a.mem.Usage()
	if err != nil {
		return Unknown, err.Error()
	}
	if s.UsedPct >= a.max {
		return Firing, fmt.Sprintf("used %d%%", s.UsedPct)
	}
	return Clear, ""
}
