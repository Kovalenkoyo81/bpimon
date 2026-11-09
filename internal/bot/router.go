package bot

import (
	"strings"

	"bpimon/internal/log"
	"bpimon/internal/monitor"
	"bpimon/internal/notifier"
)

const usageHint = "Commands: /status /cpu /ram /disk /smart /sd /docker /restart <container> /reboot /poweroff /silence <min>|off /version"

// Handle parses a chat message from userID and returns the response
// ("" for none). s receives asynchronous follow-up messages.
func (h *Handler) Handle(text string, userID int64, s notifier.Notifier) string {
	args := parseCommand(text)
	if len(args) == 0 {
		return ""
	}
	command := args[0]

	log.Info.Printf("cmd=%q userID=%d", command, userID)

	if !h.limiter.Allow(userID) {
		log.Warn.Printf("rate limit exceeded: userID=%d cmd=%q", userID, command)
		return ""
	}

	if name, ok := strings.CutPrefix(command, "/restart_"); ok {
		return h.cmdRestartShortcut(name, userID)
	}

	switch command {
	case "/help":
		return h.buildCommandsMenu()
	case "/status":
		return h.collect()
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
			return "No SD cards detected on this system"
		}
		return h.collect(monitor.KindSD)
	case "/smart":
		if !h.has(monitor.KindSmart) {
			return "⚠️ SMART unavailable: smartctl not found"
		}
		return h.collect(monitor.KindSmart)
	case "/docker":
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
		return usageHint
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

func (h *Handler) cmdVersion() string {
	if h.version == nil {
		return "Version info unavailable"
	}
	return h.version.Report()
}
