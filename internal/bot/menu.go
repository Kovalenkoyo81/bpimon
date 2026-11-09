package bot

import (
	"strings"

	"bpimon/internal/monitor"
	"bpimon/internal/notifier"
)

func (h *Handler) buildCommandsMenu() string {
	var b strings.Builder
	b.WriteString("📊 Monitoring:\n")
	b.WriteString("/status — full system status\n")
	b.WriteString("/cpu — CPU & temperature\n")
	b.WriteString("/ram — RAM usage\n")
	b.WriteString("/disk — disk usage\n")
	b.WriteString("/smart — SMART disk health\n")
	if h.has(monitor.KindSD) {
		b.WriteString("/sd — SD cards\n")
	}
	b.WriteString("\n🐳 Docker:\n")
	b.WriteString("/docker — container status\n")
	for _, c := range h.containers() {
		b.WriteString(restartShortcut(c) + "\n")
	}
	b.WriteString("\n⚙️ System:\n")
	b.WriteString("/reboot — reboot (confirmation required)\n")
	b.WriteString("/poweroff — shutdown (confirmation required)\n")
	b.WriteString("/silence <min> — silence alerts (e.g. /silence 30)\n")
	b.WriteString("/silence off — resume alerts\n")
	b.WriteString("/silence — show silence status\n")
	b.WriteString("/version — bpimon version & updates\n")
	b.WriteString("/help — show this menu")
	return b.String()
}

func (h *Handler) SendCommandsMenu(s notifier.Notifier) {
	_ = s.Send(h.buildCommandsMenu())
}
