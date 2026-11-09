package monitor

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
