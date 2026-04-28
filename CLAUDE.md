# bpimon — Banana Pi Monitor Bot

Telegram bot for Linux system monitoring. Written in Go, deployed as a systemd service.

## Architecture

**SRP strictly enforced.** Each concern is its own type/file. Small focused structs, no fat handlers.

```
cmd/bpimon/main.go          — entry point, wire-up
internal/
  monitor/                  — metric providers
    cpu.go                  — CPU usage + temperature (/sys/class/thermal)
    memory.go               — RAM (Name() = "RAM")
    disk.go                 — disk usage via df -P
    smart.go                — SMART provider (wraps smartctl.go)
    smartctl.go             — smartctl -a -j JSON parser + Available()
    mmc.go                  — SD/MMC via sysfs + DiscoverSDCards()
    kmsg.go                 — dmesg parser for MMC errors
    docker.go               — Docker container status + Available()
    factory.go              — NewProviders(smart, mmc, docker []string)
    interfaces.go           — Provider, Availabler, CPUReader, ...
  alert/
    manager.go              — ticker + cooldown + state persistence + silence expiry
    factory.go              — FromProviders(providers, thresholds) []Alert
    silence.go              — Silencer interface
    cpu.go, cpu_temp.go, memory.go, disk.go, smart.go, sd.go, docker.go
  bot/
    handler.go              — Handle(cmd, userID, notifier) → response string
    confirm.go              — Confirmator (timed confirmation codes)
    ratelimit.go            — RateLimiter (sliding window)
    bot.go                  — Telegram API wrapper (Send, SendAlert, UpdatesChan)
  config/
    config.go               — YAML load + env overrides + validation
  notifier/
    interface.go            — Notifier, AlertSender interfaces
    dummy.go                — no-op implementation for testing
  log/
    log.go                  — log.Info, log.Warn, log.Error
config.yaml.example         — config template (copy to config.yaml and fill in)
```

## Key interfaces

- `monitor.Provider` — `Name() string`, `Status() (string, error)`
- `monitor.Availabler` — `Available() bool` (factory skips if false)
- `notifier.Notifier` — `Send(text string) error`
- `notifier.AlertSender` — extends Notifier with `SendAlert(text string) error` (adds timestamp)
- `alert.Silencer` — `Silence(d)`, `Unsilence()`, `IsSilenced()`, `SilencedUntil()`

## Deploy

```bash
# Build + deploy in one step
make deploy VERSION=v1.4.0

# Or step by step
make build VERSION=v1.4.0
ssh root@host "systemctl stop bpimon"
scp bpimon root@host:/tmp/bpimon_new
ssh root@host "mv /tmp/bpimon_new /usr/local/bin/bpimon && chmod +x /usr/local/bin/bpimon && systemctl start bpimon"

# Logs
make logs
make follow
```

Override server: `make deploy SERVER=user@host`

## Systemd env vars

```
BPIMON_TELEGRAM_TOKEN=...
BPIMON_TELEGRAM_CHATID=...    # group chat — negative number
BPIMON_TELEGRAM_ADMINS=...    # comma-separated user IDs
```

## Alert state persistence

Pass `-state /var/lib/bpimon/alert_state.json` to persist active alert state across restarts.
Add this flag to `ExecStart` in the systemd unit.

## Bot commands

| Command | Description |
|---------|-------------|
| /status | Full system status |
| /cpu | CPU usage + temperature |
| /ram | RAM usage |
| /disk | Disk usage |
| /smart | SMART disk health |
| /sd | SD card status |
| /docker | Docker container status |
| /restart \<name\> | Restart container (confirmation required) |
| /reboot | Reboot server (confirmation required) |
| /poweroff | Shutdown server (confirmation required) |
| /silence \<min\> | Silence alerts for N minutes |
| /silence off | Resume alerts |
| /silence | Show current silence status |
| /help | Show this menu |

## Important details

- Group chats: Telegram appends `@BotName` to commands — stripped in handler.go
- Admin check via `cfg.Telegram.Admins` (from env)
- SMART Life `-1` = unavailable (not shown)
- CPU temp: `/sys/class/thermal/thermal_zone*/temp`, prefers zones named `cpu`, `soc`, `arm`
- Docker restart: 90s timeout; sends status message on completion
- Rate limit: 5 commands per 10 seconds per user
- Confirmation codes: 4-digit for containers, 6-digit for reboot/poweroff, expire in 60s
