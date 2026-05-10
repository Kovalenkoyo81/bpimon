package monitor

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var mmcErrKeywords = []string{"error", "failed", "timeout", "eio"}

var dmesgCache struct {
	sync.Mutex
	data      []byte
	fetchedAt time.Time
}

const dmesgCacheTTL = 60 * time.Second

func dmesgOutput() ([]byte, error) {
	dmesgCache.Lock()
	defer dmesgCache.Unlock()
	if time.Since(dmesgCache.fetchedAt) > dmesgCacheTTL {
		out, err := exec.Command("dmesg").Output()
		if err != nil {
			return nil, fmt.Errorf("dmesg: %w", err)
		}
		dmesgCache.data = out
		dmesgCache.fetchedAt = time.Now()
	}
	return append([]byte(nil), dmesgCache.data...), nil
}

type kmsgResult struct {
	errorCount int
	lastError  string
}

// hostName resolves a block device to its MMC host controller name (e.g. "mmc0", "mmc1").
// This name is universal across all Linux kernels and appears in dmesg as-is.
// e.g. mmcblk0 → sysfs path contains "mmc_host/mmc0" → returns "mmc0"
func hostName(blockDev string) string {
	target, err := os.Readlink("/sys/block/" + blockDev + "/device")
	if err != nil {
		return ""
	}
	for _, part := range strings.Split(filepath.ToSlash(target), "/") {
		if strings.HasPrefix(part, "mmc") && !strings.Contains(part, ":") && !strings.HasSuffix(part, ".mmc") {
			return part // "mmc0", "mmc1", etc.
		}
	}
	return ""
}

// readKernelErrors scans the kernel ring buffer for errors related to
// the given block device and its MMC host controller.
func readKernelErrors(blockDev string) (kmsgResult, error) {
	host := hostName(blockDev)

	out, err := dmesgOutput()
	if err != nil {
		return kmsgResult{}, err
	}

	var result kmsgResult
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, blockDev) && (host == "" || !strings.Contains(line, host)) {
			continue
		}
		lower := strings.ToLower(line)
		for _, kw := range mmcErrKeywords {
			if strings.Contains(lower, kw) {
				result.errorCount++
				result.lastError = trimDmesgLine(line)
				break
			}
		}
	}

	return result, nil
}

func trimDmesgLine(line string) string {
	if i := strings.Index(line, "] "); i != -1 {
		line = line[i+2:]
	}
	if len(line) > 100 {
		line = line[:100] + "…"
	}
	return line
}
