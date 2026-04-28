package monitor

import "fmt"

type SmartHealth struct {
	Device        string
	DiskType      string
	Health        string
	Temp          int
	Life          int // -1 = unavailable
	Reallocated   int // -1 = unavailable
	Pending       int // -1 = unavailable
	Uncorrectable int // -1 = unavailable
}

type Smart struct {
	Device string
}

func (s Smart) Name() string       { return "SMART " + s.Device }
func (s Smart) DeviceName() string { return s.Device }

func (s Smart) Health() (SmartHealth, error) {
	h, err := runSmartctl(s.Device)
	if err != nil {
		return SmartHealth{}, err
	}
	h.Device = s.Device
	return h, nil
}

func (s Smart) Status() (string, error) {
	h, err := s.Health()
	if err != nil {
		return "", err
	}

	tempStr := "n/a"
	if h.Temp >= 0 {
		tempStr = fmt.Sprintf("%d°C", h.Temp)
	}

	out := fmt.Sprintf("🔬 SMART %s [%s]\nHealth: %s\nTemp: %s", h.Device, h.DiskType, h.Health, tempStr)

	if h.Life >= 0 {
		out += fmt.Sprintf("\nLife: %d%%", h.Life)
	}

	if h.Reallocated >= 0 {
		out += fmt.Sprintf("\nReallocated: %d", h.Reallocated)
		out += fmt.Sprintf("\nPending: %d", h.Pending)
		out += fmt.Sprintf("\nUncorrectable: %d", h.Uncorrectable)
	}

	return out, nil
}
