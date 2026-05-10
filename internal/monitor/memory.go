package monitor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type MemoryStat struct {
	TotalMB uint64
	FreeMB  uint64
	UsedPct int
}

type Memory struct{}

func (Memory) Name() string { return "RAM" }

func (Memory) Usage() (MemoryStat, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryStat{}, err
	}
	defer f.Close()

	var total, free uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			fmt.Sscan(fields[1], &total)
		case "MemAvailable:":
			fmt.Sscan(fields[1], &free)
		}
	}

	if total == 0 {
		return MemoryStat{}, fmt.Errorf("MemTotal is zero in /proc/meminfo")
	}
	if free > total {
		free = total
	}
	used := total - free
	return MemoryStat{
		TotalMB: total / 1024,
		FreeMB:  free / 1024,
		UsedPct: int((used * 100) / total),
	}, nil
}

func (m Memory) Status() (string, error) {
	s, err := m.Usage()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("🧠 RAM\nTotal: %d MB\nFree: %d MB\nUsed: %d%%",
		s.TotalMB, s.FreeMB, s.UsedPct), nil
}
