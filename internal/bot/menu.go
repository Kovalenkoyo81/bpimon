package bot

import (
	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
	"github.com/Kovalenkoyo81/bpimon/internal/notifier"
)

func (h *Handler) buildCommandsMenu() format.HTML {
	item := func(cmd, desc string) format.HTML { return format.Text(cmd + " — " + desc) }

	monitoring := []format.HTML{
		format.Bold("📊 Monitoring"),
		item("/status", "full system status"),
		item("/cpu", "CPU & temperature"),
		item("/ram", "RAM usage"),
		item("/disk", "disk usage"),
	}
	if h.has(monitor.KindSmart) {
		monitoring = append(monitoring, item("/smart", "SMART disk health"))
	}
	if h.has(monitor.KindSD) {
		monitoring = append(monitoring, item("/sd", "SD cards"))
	}

	var docker []format.HTML
	if h.has(monitor.KindDocker) {
		docker = append(docker, format.Bold("🐳 Docker"), item("/docker", "container status"))
		for _, c := range h.containers() {
			docker = append(docker, item(restartShortcut(c), "restart "+c))
		}
	}

	system := []format.HTML{
		format.Bold("⚙️ System"),
		item("/silence <min>", "silence alerts, e.g. /silence 30"),
		item("/silence off", "resume alerts"),
		item("/silence", "silence status"),
		item("/reboot", "reboot (confirmation required)"),
		item("/poweroff", "shut down (confirmation required)"),
		item("/version", "version and updates"),
		item("/help", "this menu"),
	}

	sections := []format.HTML{format.Lines(monitoring...)}
	if len(docker) > 0 {
		sections = append(sections, format.Lines(docker...))
	}
	sections = append(sections, format.Lines(system...))
	return format.Join(sections, "\n\n")
}

func (h *Handler) SendCommandsMenu(s notifier.Notifier) {
	_ = s.Send(h.buildCommandsMenu())
}
