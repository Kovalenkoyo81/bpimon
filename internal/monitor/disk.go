package monitor

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Disk struct{}

type DiskUsage struct {
	Device  string
	Mount   string
	UsedPct int
	UsedGB  float64
	TotalGB float64
}

func (Disk) Name() string { return "Disk" }

func (Disk) Usage() ([]DiskUsage, error) {
	out, err := exec.Command("df", "-P").Output()
	if err != nil {
		return nil, err
	}

	var res []DiskUsage
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Scan() // skip header

	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 || !strings.HasPrefix(fields[0], "/dev") {
			continue
		}
		total, _ := strconv.ParseFloat(fields[1], 64)
		used, _ := strconv.ParseFloat(fields[2], 64)
		pct, _ := strconv.Atoi(strings.TrimSuffix(fields[4], "%"))

		res = append(res, DiskUsage{
			Device:  fields[0],
			Mount:   fields[5],
			UsedPct: pct,
			UsedGB:  used / 1024 / 1024,
			TotalGB: total / 1024 / 1024,
		})
	}
	return res, nil
}

func (d Disk) Status() (string, error) {
	ds, err := d.Usage()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, du := range ds {
		fmt.Fprintf(&b, "💾 %s(%s):%d%%(%.1fGB/%.1fGB)\n",
			du.Device, du.Mount, du.UsedPct, du.UsedGB, du.TotalGB)
	}
	return b.String(), nil
}
