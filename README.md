# bpimon

[![CI](https://github.com/Kovalenkoyo81/bpimon/actions/workflows/ci.yml/badge.svg)](https://github.com/Kovalenkoyo81/bpimon/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25-blue)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Telegram bot for Linux system monitoring on single-board computers — Raspberry Pi, Orange Pi, Banana Pi, and any Linux SBC.

Monitors CPU, RAM, disk, SMART drives, SD cards, and Docker containers. Sends alerts when thresholds are exceeded. Supports remote reboot/poweroff with confirmation.

![bpimon screenshot](assets/screenshot.jpg)

## Features

- CPU usage and temperature
- RAM usage
- Disk usage per partition
- SMART drive health (temperature, life remaining)
- SD/MMC card error monitoring via kernel log
- Docker container status and remote restart
- Alert system with cooldown, silence control, and state persistence
- Remote reboot / poweroff with confirmation code
- Rate limiting and admin-only commands
- Works on any Linux SBC: arm64, armv7, armv6, amd64

## Prerequisites

- Go 1.25 or later — [install](https://go.dev/dl/)
- A Telegram bot token — create one via [@BotFather](https://t.me/botfather)
- Your Telegram chat ID — get it from [@userinfobot](https://t.me/userinfobot)
- Optional: `smartctl` for SMART monitoring (`apt install smartmontools`)
- Optional: `docker` CLI for container monitoring

## Quick start

### 1. Clone

```bash
git clone https://github.com/kovalenkoyo81/bpimon.git
cd bpimon
cp config.yaml.example config.yaml
```

### 2. Configure

Open `config.yaml` and set your devices. Everything else can stay at defaults:

```yaml
devices:
  smart:
    - /dev/sda        # remove if no external drives
  docker:
    - my-container    # list your Docker containers, or remove section
```

Set your Telegram credentials as environment variables. The Makefile will write them to `/etc/bpimon/env` on the server (mode `600`, readable only by root) — they are never stored in the repository:

```bash
export BPIMON_TELEGRAM_TOKEN=your_bot_token
export BPIMON_TELEGRAM_CHATID=your_chat_id    # negative number for group chats
export BPIMON_TELEGRAM_ADMINS=123456789        # your Telegram user ID
```

Add these exports to `~/.bashrc` or `~/.zshrc` to avoid re-entering them each session.

### 3. Deploy to your SBC

```bash
make deploy SERVER=user@your-sbc-ip
```

The Makefile cross-compiles the binary, uploads everything to the server, writes credentials to `/etc/bpimon/env` (mode `600`), and starts the service. The version is picked up automatically from the latest git tag. Done.

**Default target architecture is `linux/arm v7`.** For other boards:

```bash
make deploy SERVER=user@host GOARCH=arm64   # Raspberry Pi 4/5, Orange Pi 5
make deploy SERVER=user@host GOARCH=amd64   # x86_64
```

### 4. Check it works

```bash
make status SERVER=user@your-sbc-ip
make logs   SERVER=user@your-sbc-ip
```

The bot will send an initial status message to your chat on startup.

---

## Configuration reference

| Field | Default | Description |
|-------|---------|-------------|
| `telegram.enabled` | `true` | Enable Telegram bot |
| `thresholds.cpu` | `85` | CPU usage alert threshold (%) |
| `thresholds.cpu_temp` | `70` | CPU temperature alert threshold (°C) |
| `thresholds.ram` | `90` | RAM usage alert threshold (%) |
| `thresholds.disk` | `85` | Disk usage alert threshold (%) |
| `thresholds.smart_temp` | `55` | SMART drive temperature threshold (°C) |
| `thresholds.smart_life` | `20` | SMART drive remaining life threshold (%) |
| `thresholds.interval_min` | `2` | Alert check interval (minutes) |
| `thresholds.cooldown_min` | `30` | Minimum time between repeated alerts (minutes) |
| `devices.smart` | `[]` | Block devices to monitor with SMART (e.g. `/dev/sda`) |
| `devices.mmc` | `[]` | SD/MMC devices to monitor (e.g. `mmcblk0`). Auto-discovered if omitted. |
| `devices.docker` | `[]` | Docker containers to monitor |

### Environment variables

| Variable | Description |
|----------|-------------|
| `BPIMON_TELEGRAM_TOKEN` | Bot token from @BotFather |
| `BPIMON_TELEGRAM_CHATID` | Target chat ID (negative for group chats) |
| `BPIMON_TELEGRAM_ADMINS` | Comma-separated list of admin user IDs |

## Bot commands

| Command | Description |
|---------|-------------|
| `/status` | Full system status |
| `/cpu` | CPU usage and temperature |
| `/ram` | RAM usage |
| `/disk` | Disk usage per partition |
| `/smart` | SMART drive health |
| `/sd` | SD card error status |
| `/docker` | Docker container status |
| `/restart <name>` | Restart a Docker container (confirmation required) |
| `/reboot` | Reboot the server (confirmation required) |
| `/poweroff` | Shut down the server (confirmation required) |
| `/silence <minutes>` | Silence alerts for N minutes |
| `/silence off` | Resume alerts |
| `/silence` | Show current silence status |
| `/help` | Show command list |

Admin commands (`/restart`, `/reboot`, `/poweroff`, `/silence`) require the sender's ID to be listed in `BPIMON_TELEGRAM_ADMINS`.

## Makefile reference

```bash
make build              # cross-compile binary
make deploy             # build + upload + start on SERVER
make test               # run tests
make vet                # run go vet
make logs               # last 50 log lines from SERVER
make follow             # live log stream from SERVER
make status             # systemctl status on SERVER
```

Key variables: `SERVER`, `GOOS`, `GOARCH`, `GOARM`. `VERSION` is auto-detected from the latest git tag.

## Troubleshooting

**Bot doesn't respond**
Check that the bot is running (`make status`) and that you sent the message to the correct chat. In group chats, the bot must have access to messages — disable privacy mode via @BotFather (`/setprivacy → Disable`).

**"SMART unavailable"**
Install smartmontools: `apt install smartmontools`. The bot skips SMART silently if `smartctl` is not found.

**"docker not found"**
The bot skips Docker monitoring if the `docker` CLI is not in PATH. Make sure docker is installed and accessible by root.

**No temperature reading**
Some boards expose CPU temperature under non-standard thermal zone names. The bot searches for zones named `cpu`, `soc`, `arm`, `pkg`, `core`. If your board uses a different name, open an issue.

**Chat ID is wrong**
Group chat IDs are negative numbers. Forward any message from the group to [@userinfobot](https://t.me/userinfobot) to get the correct ID.

## License

MIT — see [LICENSE](LICENSE)
