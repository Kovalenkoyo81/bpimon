package alert

import (
	"fmt"

	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
)

type CPUTemp struct {
	cpu monitor.CPUTempReader
	max float64
}

func NewCPUTemp(c monitor.CPUTempReader, max float64) *CPUTemp {
	return &CPUTemp{cpu: c, max: max}
}

func (a *CPUTemp) Name() string { return "CPU Temp" }

func (a *CPUTemp) Evaluate() (Verdict, string) {
	t, err := a.cpu.TempC()
	if err != nil {
		return Unknown, err.Error()
	}
	if t >= a.max {
		return Firing, fmt.Sprintf("%.1f°C", t)
	}
	return Clear, ""
}
