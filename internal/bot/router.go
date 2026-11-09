package bot

import (
	"log/slog"
	"strings"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
	"github.com/Kovalenkoyo81/bpimon/internal/notifier"
)

const usageHint = "Commands: /status /cpu /ram /disk /smart /sd /docker /restart <container> /reboot /poweroff /silence <min>|off /version — /help for details"

// Handle parses a chat message from userID and returns the response
// (zero HTML for none). s receives asynchronous follow-up messages.
func (h *Handler) Handle(text string, userID int64, s notifier.Notifier) format.HTML {
	args := parseCommand(text)
	if len(args) == 0 {
		return format.HTML{}
	}
	command := args[0]

	slog.Info("command", "cmd", command, "user", userID)

	if !h.limiter.Allow(userID) {
		slog.Warn("rate limit exceeded", "cmd", command, "user", userID)
		return format.HTML{}
	}

	if name, ok := strings.CutPrefix(command, "/restart_"); ok {
		return h.cmdRestartShortcut(name, userID)
	}

	switch command {
	case "/help", "/start":
		return h.buildCommandsMenu()
	case "/status":
		return h.statusReport()
	case "/version":
		return h.cmdVersion()
	case "/cpu":
		return h.collect(monitor.KindCPU)
	case "/ram":
		return h.collect(monitor.KindRAM)
	case "/disk":
		return h.collect(monitor.KindDisk)
	case "/sd":
		if !h.has(monitor.KindSD) {
			return format.Text("No SD cards detected on this system")
		}
		return h.collect(monitor.KindSD)
	case "/smart":
		if !h.has(monitor.KindSmart) {
			return format.Text("⚠️ SMART unavailable: smartctl not found")
		}
		return h.collect(monitor.KindSmart)
	case "/docker":
		if !h.has(monitor.KindDocker) {
			return format.Text("Docker monitoring is not configured")
		}
		return h.collect(monitor.KindDocker)
	case "/restart":
		return h.cmdRestart(args[1:], userID)
	case "/silence":
		return h.cmdSilence(args[1:], userID)
	case "/confirm":
		return h.cmdConfirm(args[1:], userID, s)
	case "/reboot":
		return h.cmdReboot(userID)
	case "/poweroff":
		return h.cmdPoweroff(userID)
	default:
		return format.Text(usageHint)
	}
}

// parseCommand splits a message into fields, lower-cases the command and
// strips the @BotName suffix Telegram adds to it in group chats
// (/reboot@MyBot → /reboot). Arguments are kept as is.
func parseCommand(text string) []string {
	args := strings.Fields(text)
	if len(args) == 0 {
		return nil
	}
	cmd := args[0]
	if strings.HasPrefix(cmd, "/") {
		cmd, _, _ = strings.Cut(cmd, "@")
	}
	args[0] = strings.ToLower(cmd)
	return args
}

func (h *Handler) cmdVersion() format.HTML {
	if h.version == nil {
		return format.Text("Version info unavailable")
	}
	return h.version.Report()
}
