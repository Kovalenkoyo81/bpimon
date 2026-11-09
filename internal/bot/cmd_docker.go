package bot

import (
	"errors"
	"strings"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
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

var errOnlyAdminsRestart = format.Text("⚠️ Only admins can restart containers")

// cmdRestartShortcut handles /restart_<name> (dashes written as underscores).
func (h *Handler) cmdRestartShortcut(normalized string, userID int64) format.HTML {
	if !h.IsAdmin(userID) {
		return errOnlyAdminsRestart
	}
	container := h.findContainerByNormalized(normalized)
	if container == "" {
		return format.Fmt("⚠️ Unknown container: %s", normalized)
	}
	return h.requestRestart(userID, container)
}

// cmdRestart handles "/restart <name>" and lists containers for bare /restart.
func (h *Handler) cmdRestart(args []string, userID int64) format.HTML {
	if !h.IsAdmin(userID) {
		return errOnlyAdminsRestart
	}
	if len(args) >= 1 {
		container := h.findContainerByNormalized(strings.ReplaceAll(args[0], "-", "_"))
		if container == "" {
			return format.Fmt("⚠️ Unknown container: %s", args[0])
		}
		return h.requestRestart(userID, container)
	}
	lines := []format.HTML{format.Bold("Restart a container:")}
	for _, c := range h.containers() {
		lines = append(lines, format.Text(restartShortcut(c)))
	}
	if len(lines) == 1 {
		return format.Text("No Docker containers configured")
	}
	return format.Lines(lines...)
}

func (h *Handler) requestRestart(userID int64, container string) format.HTML {
	code := h.confirm.Request(userID, actionDockerPrefix+container, 4)
	return format.Fmt("🔐 Send %s to restart <b>%s</b>.", format.Code("/confirm "+code), container)
}

// restartContainer restarts in the background and reports the outcome to s.
func (h *Handler) restartContainer(name string, s notifier.Notifier) {
	_ = s.Send(format.Fmt("🔄 Restarting <b>%s</b>…", name))
	go func() {
		if err := h.system.RestartContainer(name); err != nil {
			if errors.Is(err, system.ErrTimeout) {
				_ = s.Send(format.Fmt("⏱ Timeout restarting <b>%s</b> (90 s exceeded)", name))
			} else {
				_ = s.Send(format.Fmt("❌ Failed to restart <b>%s</b>: %s", name, err))
			}
			return
		}
		st, err := h.system.ContainerState(name)
		if err != nil {
			_ = s.Send(format.Fmt("🔄 <b>%s</b> restarted", name))
			return
		}
		_ = s.Send(restartReport(name, st))
	}()
}

// ✅ syncthing restarted
// status: running · started 21:35:02 UTC
func restartReport(name string, st system.ContainerState) format.HTML {
	icon, verb := "✅", "restarted"
	if st.Status != "running" {
		icon, verb = "❌", "did not stay running"
	}
	details := format.Fmt("status: %s", format.Code(st.Status))
	if !st.StartedAt.IsZero() {
		details = format.Fmt("%s · started %s", details, st.StartedAt.UTC().Format("15:04:05 UTC"))
	}
	return format.Lines(format.Fmt("%s <b>%s</b> %s", icon, name, verb), details)
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
