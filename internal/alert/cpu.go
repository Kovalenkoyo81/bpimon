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

func (a *CPU) Evaluate() (Verdict, string) {
	s, err := a.cpu.Usage()
	if err != nil {
		return Unknown, err.Error()
	}
	if s.UsagePct >= a.max {
		msg := fmt.Sprintf("usage %.1f%%", s.UsagePct)
		if top := monitor.TopProcs(3); top != "" {
			msg += "\n📋 Top: " + top
		}
		return Firing, msg
	}
	return Clear, ""
}
