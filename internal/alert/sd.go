package alert

import (
	"bpimon/internal/monitor"
	"fmt"
)

type SD struct {
	s monitor.SDReader
}

func NewSD(s monitor.SDReader) *SD {
	return &SD{s: s}
}

func (a *SD) Name() string { return a.s.Name() }

func (a *SD) Check() (bool, string) {
	h, err := a.s.Health()
	if err != nil {
		return false, ""
	}
	if h.ErrorCount > 0 {
		msg := fmt.Sprintf("kernel errors: %d", h.ErrorCount)
		if h.LastError != "" {
			msg += " — " + h.LastError
		}
		return true, msg
	}
	return false, ""
}
