package bot

import (
	"bpimon/internal/alert"
	"bpimon/internal/log"
	"bpimon/internal/monitor"
	"bpimon/internal/notifier"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// containerProvider is a monitor.Provider that also exposes its container name.
// Keeps the handler decoupled from the concrete monitor.Docker type.
type containerProvider interface {
	monitor.Provider
	ContainerName() string
}

type Handler struct {
	Providers []monitor.Provider
	Admins    []int64
	Silencer  alert.Silencer

	canReboot   bool
	canPoweroff bool

	confirm *Confirmator
	limiter *RateLimiter
}

func (h *Handler) InitPermissions() {
	h.canReboot = checkCommand("systemctl")
	h.canPoweroff = checkCommand("systemctl")
	h.confirm = NewConfirmator(60 * time.Second)
	h.limiter = NewRateLimiter(10*time.Second, 5)
}

func checkCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func (h *Handler) isAdmin(id int64) bool {
	for _, a := range h.Admins {
		if a == id {
			return true
		}
	}
	return false
}

func (h *Handler) Handle(text string, userID int64, s notifier.Notifier) string {
	// Strip @botname suffix from commands in group chats: /reboot@MyBot → /reboot
	cmd := text
	if i := strings.Index(cmd, "@"); i != -1 && strings.HasPrefix(cmd, "/") {
		cmd = cmd[:i]
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	command := strings.ToLower(parts[0])

	log.Info.Printf("cmd=%q userID=%d", command, userID)

	if !h.limiter.Allow(userID) {
		log.Warn.Printf("rate limit exceeded: userID=%d cmd=%q", userID, command)
		return ""
	}

	if strings.HasPrefix(command, "/restart_") {
		if !h.isAdmin(userID) {
			return "⚠️ Only admins can restart containers"
		}
		container := h.findContainerByNormalized(command[len("/restart_"):])
		if container == "" {
			return fmt.Sprintf("⚠️ Unknown container: %s", command[len("/restart_"):])
		}
		code := h.confirm.Request(userID, "docker:"+container, 4)
		return fmt.Sprintf("🔐 Type `/confirm %s` to confirm restart of %s.", code, container)
	}

	switch command {

	case "/help":
		return h.buildCommandsMenu()

	case "/status":
		return h.collectByPrefix("")

	case "/cpu":
		return h.collectByName("CPU")

	case "/ram":
		return h.collectByName("RAM")

	case "/disk":
		return h.collectByName("Disk")

	case "/sd":
		return h.collectByPrefix("sd")

	case "/smart":
		if !h.hasProviderByPrefix("smart") {
			return "⚠️ SMART unavailable: smartctl not found"
		}
		return h.collectByPrefix("smart")

	case "/docker":
		return h.collectByPrefix("docker")

	case "/restart":
		if !h.isAdmin(userID) {
			return "⚠️ Only admins can restart containers"
		}
		if len(parts) >= 2 {
			// /restart syncthing — direct with argument
			normalized := strings.ReplaceAll(parts[1], "-", "_")
			container := h.findContainerByNormalized(normalized)
			if container == "" {
				return fmt.Sprintf("⚠️ Unknown container: %s", parts[1])
			}
			code := h.confirm.Request(userID, "docker:"+container, 4)
			return fmt.Sprintf("🔐 Type `/confirm %s` to confirm restart of %s.", code, container)
		}
		var names []string
		for _, p := range h.Providers {
			if d, ok := p.(containerProvider); ok {
				names = append(names, "/restart_"+strings.ReplaceAll(d.ContainerName(), "-", "_"))
			}
		}
		if len(names) == 0 {
			return "No Docker containers configured"
		}
		return "Available containers:\n" + strings.Join(names, "\n")

	case "/silence":
		if !h.isAdmin(userID) {
			return "⚠️ Only admins can manage alert silence"
		}
		if h.Silencer == nil {
			return "⚠️ Alert manager not available"
		}
		if len(parts) < 2 {
			until := h.Silencer.SilencedUntil()
			if until.IsZero() || time.Now().After(until) {
				return "🔔 Alerts active"
			}
			remaining := time.Until(until).Round(time.Minute)
			return fmt.Sprintf("🔕 Alerts silenced until %s UTC (%v remaining)", until.UTC().Format("15:04"), remaining)
		}
		if strings.ToLower(parts[1]) == "off" {
			h.Silencer.Unsilence()
			return "🔔 Alerts resumed"
		}
		mins, err := strconv.Atoi(parts[1])
		if err != nil || mins <= 0 {
			return "⚠️ Specify minutes: /silence 30"
		}
		if mins > 10080 {
			return "⚠️ Maximum silence duration is 10080 minutes (7 days)"
		}
		d := time.Duration(mins) * time.Minute
		h.Silencer.Silence(d)
		until := time.Now().Add(d).UTC().Format("15:04 UTC")
		return fmt.Sprintf("🔕 Alerts silenced for %dm (until %s)", mins, until)

	case "/confirm":
		if len(parts) < 2 {
			return "Usage: /confirm <code>"
		}
		if action, ok := h.confirm.Validate(userID, parts[1]); ok {
			return h.execute(action, s)
		}
		return "⚠️ Invalid or expired code"

	case "/reboot":
		if !h.isAdmin(userID) {
			return "⚠️ Only admins can reboot the server"
		}
		if !h.canReboot {
			return "⚠️ Reboot unavailable: systemctl not found"
		}
		code := h.confirm.Request(userID, "reboot", 6)
		return fmt.Sprintf("🔐 Type `/confirm %s` within 60 seconds to confirm reboot.", code)

	case "/poweroff":
		if !h.isAdmin(userID) {
			return "⚠️ Only admins can power off the server"
		}
		if !h.canPoweroff {
			return "⚠️ Poweroff unavailable: systemctl not found"
		}
		code := h.confirm.Request(userID, "poweroff", 6)
		return fmt.Sprintf("🔐 Type `/confirm %s` within 60 seconds to confirm poweroff.", code)

	default:
		return "Commands: /status /cpu /ram /disk /smart /sd /docker /restart <container> /reboot /poweroff /silence <min>|off"
	}
}

func (h *Handler) execute(action string, s notifier.Notifier) string {
	switch {
	case action == "reboot":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "systemctl", "reboot").Run(); err != nil {
			log.Error.Printf("reboot failed: %v", err)
			return "❌ Failed to initiate reboot"
		}
		return "🔄 Server reboot initiated"
	case action == "poweroff":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "systemctl", "poweroff").Run(); err != nil {
			log.Error.Printf("poweroff failed: %v", err)
			return "❌ Failed to initiate poweroff"
		}
		return "🔌 Server poweroff initiated"
	case strings.HasPrefix(action, "docker:"):
		name := action[len("docker:"):]
		_ = s.Send("🔄 Restarting " + name + "...")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if err := exec.CommandContext(ctx, "docker", "restart", name).Run(); err != nil {
				if ctx.Err() != nil {
					_ = s.Send(fmt.Sprintf("⏱ Timeout restarting %s (90s exceeded)", name))
				} else {
					_ = s.Send(fmt.Sprintf("❌ Failed to restart %s: %v", name, err))
				}
				return
			}
			out, err := exec.Command("docker", "inspect", "--format", "{{.State.Status}}|{{.State.StartedAt}}", name).Output()
			if err != nil {
				_ = s.Send(fmt.Sprintf("🔄 Container %s restarted", name))
				return
			}
			parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
			status := parts[0]
			icon := "✅"
			warning := ""
			if status != "running" {
				icon = "❌"
				warning = "\n⚠️ Container did not stay running"
			}
			started := ""
			if len(parts) == 2 {
				if t, err := time.Parse(time.RFC3339Nano, parts[1]); err == nil {
					started = "\n🕐 Started: " + t.UTC().Format("02 Jan 2006, 15:04:05 UTC")
				}
			}
			_ = s.Send(fmt.Sprintf("🔄 Container %s restarted\n%s Status: %s%s%s", name, icon, status, started, warning))
		}()
		return ""
	}
	return ""
}

func (h *Handler) findContainerByNormalized(normalized string) string {
	for _, p := range h.Providers {
		if d, ok := p.(containerProvider); ok {
			name := d.ContainerName()
			if strings.ReplaceAll(name, "-", "_") == normalized {
				return name
			}
		}
	}
	return ""
}

func (h *Handler) hasProviderByPrefix(prefix string) bool {
	for _, p := range h.Providers {
		if strings.HasPrefix(strings.ToLower(p.Name()), prefix) {
			return true
		}
	}
	return false
}

func (h *Handler) collectByName(name string) string {
	var b strings.Builder
	for _, p := range h.Providers {
		if strings.EqualFold(p.Name(), name) {
			s, err := p.Status()
			if err != nil {
				s = p.Name() + ": error reading status"
			}
			b.WriteString(s + "\n\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func (h *Handler) collectByPrefix(prefix string) string {
	var b strings.Builder
	for _, p := range h.Providers {
		if prefix == "" || strings.HasPrefix(strings.ToLower(p.Name()), prefix) {
			s, err := p.Status()
			if err != nil {
				s = p.Name() + ": error reading status"
			}
			b.WriteString(s + "\n\n")
		}
	}
	return strings.TrimSpace(b.String())
}

const rebootFlagFile = "/tmp/bpimon.booted"

func (h *Handler) SendRebootAlert(s notifier.Notifier) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	var uptimeSec float64
	fmt.Sscanf(string(b), "%f", &uptimeSec)
	if uptimeSec >= 300 {
		return
	}
	// Flag file lives in /tmp (cleared on every real reboot).
	// If it already exists, this is a manual bot restart — skip alert.
	if _, err := os.Stat(rebootFlagFile); err == nil {
		return
	}
	_ = os.WriteFile(rebootFlagFile, []byte{}, 0600)

	d := time.Duration(uptimeSec) * time.Second
	min := int(d.Minutes())
	sec := int(d.Seconds()) % 60
	_ = s.Send(fmt.Sprintf("🔁 Bot started after system reboot\n⏱ System uptime: %dm %ds", min, sec))
}

func (h *Handler) SendInitialStatus(s notifier.Notifier) {
	msg := h.collectByPrefix("")
	if msg != "" {
		_ = s.Send("📊 Initial system status:\n" + msg)
	}
}

func (h *Handler) buildCommandsMenu() string {
	var b strings.Builder
	b.WriteString("📊 Monitoring:\n")
	b.WriteString("/status — full system status\n")
	b.WriteString("/cpu — CPU & temperature\n")
	b.WriteString("/ram — RAM usage\n")
	b.WriteString("/disk — disk usage\n")
	b.WriteString("/smart — SMART disk health\n")
	b.WriteString("/sd — SD cards\n")
	b.WriteString("\n🐳 Docker:\n")
	b.WriteString("/docker — container status\n")
	for _, p := range h.Providers {
		if d, ok := p.(containerProvider); ok {
			b.WriteString("/restart_" + strings.ReplaceAll(d.ContainerName(), "-", "_") + "\n")
		}
	}
	b.WriteString("\n⚙️ System:\n")
	b.WriteString("/reboot — reboot (confirmation required)\n")
	b.WriteString("/poweroff — shutdown (confirmation required)\n")
	b.WriteString("/silence <min> — silence alerts (e.g. /silence 30)\n")
	b.WriteString("/silence off — resume alerts\n")
	b.WriteString("/silence — show silence status\n")
	b.WriteString("/help — show this menu")
	return b.String()
}

func (h *Handler) SendCommandsMenu(s notifier.Notifier) {
	_ = s.Send(h.buildCommandsMenu())
}
