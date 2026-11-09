package monitor

import (
	"bufio"
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
	out, err := output("df", "-P")
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
