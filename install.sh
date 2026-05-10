#!/usr/bin/env bash
# bpimon installer — downloads the correct binary for this machine and sets up
# a systemd service with credentials stored in /etc/bpimon/env (mode 600).
#
# Usage (non-interactive, pipe-safe):
#   export BPIMON_TELEGRAM_TOKEN=...
#   export BPIMON_TELEGRAM_CHATID=...
#   export BPIMON_TELEGRAM_ADMINS=...
#   curl -fsSL https://github.com/Kovalenkoyo81/bpimon/releases/latest/download/install.sh | sudo -E bash
#
# Usage (interactive):
#   curl -fsSL https://github.com/Kovalenkoyo81/bpimon/releases/latest/download/install.sh -o install.sh
#   sudo bash install.sh

set -euo pipefail

REPO="Kovalenkoyo81/bpimon"
BASE_URL="https://github.com/$REPO/releases/latest/download"
RAW_URL="https://raw.githubusercontent.com/$REPO/main"

# ── root check ────────────────────────────────────────────────────────────────
if [ "$(id -u)" != "0" ]; then
  echo "Error: run as root — sudo bash install.sh" >&2
  exit 1
fi

# ── architecture detection ────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)         BIN="bpimon_amd64" ;;
  aarch64|arm64)  BIN="bpimon_arm64" ;;
  armv7l)         BIN="bpimon_armv7" ;;
  armv6l)         BIN="bpimon_armv6" ;;
  *)
    echo "Error: unsupported architecture '$ARCH'" >&2
    echo "Supported: x86_64, aarch64, armv7l, armv6l" >&2
    exit 1
    ;;
esac

# ── credentials ───────────────────────────────────────────────────────────────
# Prefer environment variables; fall back to interactive prompts.
# Interactive prompts do not work when the script is piped — use env vars in that case.
TOKEN="${BPIMON_TELEGRAM_TOKEN:-}"
CHATID="${BPIMON_TELEGRAM_CHATID:-}"
ADMINS="${BPIMON_TELEGRAM_ADMINS:-}"

if [ -z "$TOKEN" ]; then
  read -rp "Telegram bot token: " TOKEN
fi
if [ -z "$CHATID" ]; then
  read -rp "Telegram chat ID (negative number for group chats): " CHATID
fi
if [ -z "$ADMINS" ]; then
  read -rp "Admin Telegram user ID(s), comma-separated (or press Enter to skip): " ADMINS || true
fi

if [ -z "$TOKEN" ]; then
  echo "Error: Telegram bot token is required" >&2
  exit 1
fi
if [ -z "$CHATID" ]; then
  echo "Error: Telegram chat ID is required" >&2
  exit 1
fi

# ── download binary ───────────────────────────────────────────────────────────
echo "→ Detected architecture: $ARCH → downloading $BIN"
curl -fsSL --progress-bar "$BASE_URL/$BIN" -o /tmp/bpimon_new
chmod +x /tmp/bpimon_new

VERSION=$(/tmp/bpimon_new -version 2>/dev/null || echo "unknown")

# ── install ───────────────────────────────────────────────────────────────────
echo "→ Installing bpimon ($VERSION)"

systemctl stop bpimon 2>/dev/null || true

mkdir -p /etc/bpimon /var/lib/bpimon
mv /tmp/bpimon_new /usr/local/bin/bpimon

# ── config ────────────────────────────────────────────────────────────────────
# Install the example config only on first install — never overwrite existing config.
if [ ! -f /etc/bpimon/config.yaml ]; then
  echo "→ Installing default config at /etc/bpimon/config.yaml"
  curl -fsSL "$RAW_URL/config.yaml.example" -o /etc/bpimon/config.yaml
  SHOW_CONFIG_NOTICE=true
else
  echo "→ Config already exists at /etc/bpimon/config.yaml — keeping as-is"
  SHOW_CONFIG_NOTICE=false
fi

# ── credentials ───────────────────────────────────────────────────────────────
echo "→ Writing credentials to /etc/bpimon/env (mode 600)"
printf 'BPIMON_TELEGRAM_TOKEN=%s\nBPIMON_TELEGRAM_CHATID=%s\nBPIMON_TELEGRAM_ADMINS=%s\n' \
  "$TOKEN" "$CHATID" "$ADMINS" > /etc/bpimon/env
chmod 600 /etc/bpimon/env

# ── systemd unit ──────────────────────────────────────────────────────────────
echo "→ Installing systemd unit"
curl -fsSL "$RAW_URL/deploy/bpimon.service" -o /etc/systemd/system/bpimon.service

systemctl daemon-reload
systemctl enable bpimon
systemctl start bpimon

# ── done ──────────────────────────────────────────────────────────────────────
echo ""
echo "✓ bpimon $VERSION installed and running."
echo ""
if [ "$SHOW_CONFIG_NOTICE" = "true" ]; then
  echo "  ⚠ Edit /etc/bpimon/config.yaml to configure your devices, then restart:"
  echo "    nano /etc/bpimon/config.yaml"
  echo "    systemctl restart bpimon"
  echo ""
fi
echo "  systemctl status bpimon"
echo "  journalctl -u bpimon -f"
