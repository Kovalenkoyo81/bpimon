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

func (a *Smart) Check() (bool, string) {
	h, err := a.s.Health()
	if err != nil {
		return false, ""
	}
	if h.Health != "PASSED" {
		return true, "FAILED"
	}
	if h.Temp >= 0 && h.Temp >= a.temp {
		return true, fmt.Sprintf("temp %d°C", h.Temp)
	}
	if h.Life >= 0 && h.Life <= a.life {
		return true, fmt.Sprintf("life %d%%", h.Life)
	}
	return false, ""
}
