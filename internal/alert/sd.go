package alert

import (
	"fmt"

	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
)

type SD struct {
	s monitor.SDReader
}

func NewSD(s monitor.SDReader) *SD {
	return &SD{s: s}
}

func (a *SD) Name() string { return a.s.Name() }

func (a *SD) Evaluate() (Verdict, string) {
	h, err := a.s.Health()
	if err != nil {
		return Unknown, err.Error()
	}
	if h.ErrorCount > 0 {
		msg := fmt.Sprintf("kernel errors: %d", h.ErrorCount)
		if h.LastError != "" {
			msg += " — " + h.LastError
		}
		return Firing, msg
	}
	return Clear, ""
}
