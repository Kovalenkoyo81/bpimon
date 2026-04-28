package alert

import (
	"bpimon/internal/monitor"
	"fmt"
)

type CPUTemp struct {
	cpu monitor.CPUTempReader
	max float64
}

func NewCPUTemp(c monitor.CPUTempReader, max float64) *CPUTemp {
	return &CPUTemp{cpu: c, max: max}
}

func (a *CPUTemp) Name() string { return "CPU Temp" }

func (a *CPUTemp) Check() (bool, string) {
	t, err := a.cpu.TempC()
	if err != nil {
		return false, ""
	}
	if t >= a.max {
		return true, fmt.Sprintf("%.1f°C", t)
	}
	return false, ""
}
