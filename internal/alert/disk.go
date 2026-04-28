package alert

import (
	"bpimon/internal/monitor"
	"fmt"
	"strings"
)

type Disk struct {
	disk monitor.DiskReader
	max  int
}

func NewDisk(d monitor.DiskReader, max int) *Disk {
	return &Disk{disk: d, max: max}
}

func (a *Disk) Name() string { return "Disk" }

func (a *Disk) Check() (bool, string) {
	ds, err := a.disk.Usage()
	if err != nil {
		return false, ""
	}
	var over []string
	for _, d := range ds {
		if d.UsedPct >= a.max {
			over = append(over, fmt.Sprintf("%s %d%%", d.Mount, d.UsedPct))
		}
	}
	if len(over) > 0 {
		return true, strings.Join(over, ", ")
	}
	return false, ""
}
