package alert

import (
	"bpimon/internal/monitor"
	"fmt"
)

type Smart struct {
	s    monitor.SmartReader
	temp int
	life int
}

func NewSmart(s monitor.SmartReader, temp, life int) *Smart {
	return &Smart{s: s, temp: temp, life: life}
}

func (a *Smart) Name() string {
	return "SMART " + a.s.DeviceName()
}

func (a *Smart) Evaluate() (Verdict, string) {
	h, err := a.s.Health()
	if err != nil {
		return Unknown, err.Error()
	}
	if h.Health != "PASSED" {
		return Firing, "FAILED"
	}
	if h.Temp >= 0 && h.Temp >= a.temp {
		return Firing, fmt.Sprintf("temp %d°C", h.Temp)
	}
	if h.Life >= 0 && h.Life <= a.life {
		return Firing, fmt.Sprintf("life %d%%", h.Life)
	}
	return Clear, ""
}
