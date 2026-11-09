package bot

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
	"github.com/Kovalenkoyo81/bpimon/internal/notifier"
	"github.com/Kovalenkoyo81/bpimon/internal/system"
)

// containerProvider is a monitor.Provider that also exposes its container name.
// Keeps the handler decoupled from the concrete monitor.Docker type.
type containerProvider interface {
	monitor.Provider
	ContainerName() string
}

const actionDockerPrefix = "docker:"

// cmdRestartShortcut handles /restart_<name> (dashes written as underscores).
func (h *Handler) cmdRestartShortcut(normalized string, userID int64) string {
	if !h.IsAdmin(userID) {
		return "⚠️ Only admins can restart containers"
	}
	container := h.findContainerByNormalized(normalized)
	if container == "" {
		return fmt.Sprintf("⚠️ Unknown container: %s", normalized)
	}
	return h.requestRestart(userID, container)
}

// cmdRestart handles "/restart <name>" and lists containers for bare /restart.
func (h *Handler) cmdRestart(args []string, userID int64) string {
	if !h.IsAdmin(userID) {
		return "⚠️ Only admins can restart containers"
	}
	if len(args) >= 1 {
		container := h.findContainerByNormalized(strings.ReplaceAll(args[0], "-", "_"))
		if container == "" {
			return fmt.Sprintf("⚠️ Unknown container: %s", args[0])
		}
		return h.requestRestart(userID, container)
	}
	var names []string
	for _, c := range h.containers() {
		names = append(names, restartShortcut(c))
	}
	if len(names) == 0 {
		return "No Docker containers configured"
	}
	return "Available containers:\n" + strings.Join(names, "\n")
}

func (h *Handler) requestRestart(userID int64, container string) string {
	code := h.confirm.Request(userID, actionDockerPrefix+container, 4)
	return fmt.Sprintf("🔐 Type `/confirm %s` to confirm restart of %s.", code, container)
}

// restartContainer restarts in the background and reports the outcome to s.
func (h *Handler) restartContainer(name string, s notifier.Notifier) {
	_ = s.Send("🔄 Restarting " + name + "...")
	go func() {
		if err := h.system.RestartContainer(name); err != nil {
			if errors.Is(err, system.ErrTimeout) {
				_ = s.Send(fmt.Sprintf("⏱ Timeout restarting %s (90s exceeded)", name))
			} else {
				_ = s.Send(fmt.Sprintf("❌ Failed to restart %s: %v", name, err))
			}
			return
		}
		st, err := h.system.ContainerState(name)
		if err != nil {
			_ = s.Send(fmt.Sprintf("🔄 Container %s restarted", name))
			return
		}
		_ = s.Send(restartReport(name, st))
	}()
}

func restartReport(name string, st system.ContainerState) string {
	icon, warning := "✅", ""
	if st.Status != "running" {
		icon = "❌"
		warning = "\n⚠️ Container did not stay running"
	}
	started := ""
	if !st.StartedAt.IsZero() {
		started = "\n🕐 Started: " + st.StartedAt.UTC().Format("02 Jan 2006, 15:04:05 UTC")
	}
	return fmt.Sprintf("🔄 Container %s restarted\n%s Status: %s%s%s", name, icon, st.Status, started, warning)
}

func (h *Handler) containers() []string {
	var names []string
	for _, p := range h.providers {
		if d, ok := p.(containerProvider); ok {
			names = append(names, d.ContainerName())
		}
	}
	return names
}

func (h *Handler) findContainerByNormalized(normalized string) string {
	for _, name := range h.containers() {
		if strings.ReplaceAll(name, "-", "_") == normalized {
			return name
		}
	}
	return ""
}

func restartShortcut(container string) string {
	return "/restart_" + strings.ReplaceAll(container, "-", "_")
}
