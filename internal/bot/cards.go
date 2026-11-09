package bot

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
)

// sectionOrder is the order of sections in /status.
var sectionOrder = []monitor.Kind{
	monitor.KindCPU, monitor.KindRAM, monitor.KindDisk,
	monitor.KindSmart, monitor.KindSD, monitor.KindDocker,
}

func (h *Handler) has(k monitor.Kind) bool { return len(h.byKind(k)) > 0 }

func (h *Handler) byKind(k monitor.Kind) []monitor.Provider {
	var out []monitor.Provider
	for _, p := range h.providers {
		if p.Kind() == k {
			out = append(out, p)
		}
	}
	return out
}

// collect renders the cards of the given kinds (all kinds if none given),
// separated by blank lines.
func (h *Handler) collect(kinds ...monitor.Kind) format.HTML {
	if len(kinds) == 0 {
		kinds = sectionOrder
	}
	var cards []format.HTML
	for _, k := range kinds {
		if ps := h.byKind(k); len(ps) > 0 {
			cards = append(cards, renderSection(k, ps))
		}
	}
	return format.Join(cards, "\n\n")
}

// statusReport is /status: a header with the host and its uptime, then
// every card.
//
//	📊 bananapipro · up 3d 4h
func (h *Handler) statusReport() format.HTML {
	header := format.Concat(format.Text("📊 "), format.Bold(h.host))
	if up, err := monitor.SystemUptime(); err == nil {
		header = format.Concat(header, format.Text(" · up "+format.Duration(up)))
	}
	return format.Concat(header, format.Text("\n\n"), h.collect())
}

func renderSection(k monitor.Kind, ps []monitor.Provider) format.HTML {
	if k == monitor.KindDocker {
		return dockerCard(ps)
	}
	var cards []format.HTML
	for _, p := range ps {
		cards = append(cards, renderCard(p))
	}
	return format.Join(cards, "\n\n")
}

func renderCard(p monitor.Provider) format.HTML {
	switch v := p.(type) {
	case monitor.CPUSampler:
		return cpuCard(v, p)
	case monitor.MemoryReader:
		return memoryCard(v)
	case monitor.DiskReader:
		return diskCard(v)
	case monitor.SmartReader:
		return smartCard(v)
	case monitor.SDReader:
		return sdCard(v)
	}
	return format.Bold(p.Name())
}

// unavailable is a card for a provider whose data could not be read.
func unavailable(title string, err error) format.HTML {
	return format.Fmt("<b>%s</b> ⚠️ %s", title, err)
}

// 🖥 CPU   9.0% · 39.4°C
// top: nginx 45.2%, python 12.1%
func cpuCard(s monitor.CPUSampler, p monitor.Provider) format.HTML {
	st, err := s.Sample()
	if err != nil {
		return unavailable("🖥 CPU", err)
	}
	line := format.Fmt("<b>🖥 CPU</b>   %s", format.Code(fmt.Sprintf("%.1f%%", st.UsagePct)))
	if t, ok := p.(monitor.CPUTempReader); ok {
		if c, err := t.TempC(); err == nil {
			line = format.Fmt("%s · %s", line, format.Code(fmt.Sprintf("%.1f°C", c)))
		}
	}
	var top format.HTML
	if procs := monitor.TopProcs(3); procs != "" {
		top = format.Italic("top: " + procs)
	}
	return format.Lines(line, top)
}

// 🧠 RAM   43% · 413 / 958 MB
func memoryCard(r monitor.MemoryReader) format.HTML {
	m, err := r.Usage()
	if err != nil {
		return unavailable("🧠 RAM", err)
	}
	used := m.TotalMB - min(m.FreeMB, m.TotalMB)
	return format.Fmt("<b>🧠 RAM</b>   %s · %s",
		format.Code(fmt.Sprintf("%d%%", m.UsedPct)),
		format.Code(memoryAmount(used, m.TotalMB)))
}

// memoryAmount shows MB on small boards and GB from 1 GB up.
func memoryAmount(usedMB, totalMB uint64) string {
	if totalMB < 1024 {
		return fmt.Sprintf("%d / %d MB", usedMB, totalMB)
	}
	return fmt.Sprintf("%.1f / %.1f GB", float64(usedMB)/1024, float64(totalMB)/1024)
}

// 💾 Disk
// /         28% · 3.9 / 14.2 GB
// /mnt/500  56% · 61.1 / 109.5 GB
func diskCard(r monitor.DiskReader) format.HTML {
	ds, err := r.Usage()
	if err != nil {
		return unavailable("💾 Disk", err)
	}
	width := 0
	for _, d := range ds {
		width = max(width, len(d.Mount))
	}
	lines := []format.HTML{format.Bold("💾 Disk")}
	for _, d := range ds {
		lines = append(lines, format.Fmt("%s %s · %s",
			format.Code(fmt.Sprintf("%-*s", width, d.Mount)),
			format.Code(fmt.Sprintf("%3d%%", d.UsedPct)),
			format.Code(fmt.Sprintf("%.1f / %.1f GB", d.UsedGB, d.TotalGB))))
	}
	return format.Lines(lines...)
}

// 🔬 SMART /dev/sda (SSD)
// ✅ PASSED · 35°C · life 86%
func smartCard(r monitor.SmartReader) format.HTML {
	title := format.Fmt("<b>🔬 SMART</b> %s", r.DeviceName())
	h, err := r.Health()
	if err != nil {
		return format.Lines(title, format.Fmt("⚠️ %s", err))
	}
	if h.DiskType != "" {
		title = format.Fmt("%s (%s)", title, h.DiskType)
	}
	icon := "✅"
	if h.Health != "PASSED" {
		icon = "❌"
	}
	parts := []format.HTML{format.Text(icon + " " + h.Health)}
	if h.Temp >= 0 {
		parts = append(parts, format.Code(fmt.Sprintf("%d°C", h.Temp)))
	}
	if h.Life >= 0 {
		parts = append(parts, format.Fmt("life %s", format.Code(fmt.Sprintf("%d%%", h.Life))))
	}
	var warn format.HTML
	if h.Reallocated > 0 || h.Pending > 0 || h.Uncorrectable > 0 {
		warn = format.Fmt("⚠️ reallocated %d · pending %d · uncorrectable %d", h.Reallocated, h.Pending, h.Uncorrectable)
	}
	return format.Lines(title, format.Join(parts, " · "), warn)
}

// 💳 SD mmcblk0 (USD00)
// ✅ no errors
func sdCard(r monitor.SDReader) format.HTML {
	h, err := r.Health()
	if err != nil {
		return unavailable("💳 "+r.Name(), err)
	}
	title := format.Fmt("<b>💳 SD</b> %s", filepath.Base(h.Device))
	if h.Name != "" {
		title = format.Fmt("%s (%s)", title, h.Name)
	}
	if h.ErrorCount == 0 {
		return format.Lines(title, format.Text("✅ no errors"))
	}
	var last format.HTML
	if h.LastError != "" {
		last = format.Italic("last: " + h.LastError)
	}
	return format.Lines(title, format.Fmt("❌ %d kernel errors", h.ErrorCount), last)
}

// 🐳 Docker 29.1.3
// ✅ syncthing
// ❌ picoclaw-gateway · exited
func dockerCard(ps []monitor.Provider) format.HTML {
	title := format.Bold("🐳 Docker")
	var lines []format.HTML
	for _, p := range ps {
		switch v := p.(type) {
		case monitor.DockerDaemonReader:
			if ver, err := v.Ping(); err == nil {
				title = format.Fmt("%s %s", title, ver)
			} else {
				title = format.Fmt("%s ❌ daemon unavailable", title)
			}
		case monitor.DockerReader:
			lines = append(lines, containerLine(v))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, format.Italic("no containers monitored"))
	}
	return format.Lines(append([]format.HTML{title}, lines...)...)
}

func containerLine(r monitor.DockerReader) format.HTML {
	h, err := r.Health()
	switch {
	case err != nil:
		return format.Fmt("⚠️ %s · state unknown", r.ContainerName())
	case h.Running:
		return format.Text("✅ " + r.ContainerName())
	default:
		return format.Fmt("❌ %s · %s", r.ContainerName(), format.Code(strings.TrimSpace(h.Status)))
	}
}
