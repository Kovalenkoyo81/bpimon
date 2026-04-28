package alert

import (
	"bpimon/internal/monitor"
	"fmt"
)

type CPU struct {
	cpu monitor.CPUReader
	max float64
}

func NewCPU(c monitor.CPUReader, max float64) *CPU {
	return &CPU{cpu: c, max: max}
}

func (a *CPU) Name() string { return "CPU" }

func (a *CPU) Check() (bool, string) {
	s, err := a.cpu.Usage()
	if err != nil {
		return false, ""
	}
	if s.UsagePct >= a.max {
		return true, fmt.Sprintf("usage %.1f%%", s.UsagePct)
	}
	return false, ""
}
