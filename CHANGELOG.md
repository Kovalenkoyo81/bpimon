# Changelog

All notable changes to bpimon. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/).

The release workflow publishes the section of the tagged version as the GitHub release notes, and the bot shows the first lines of it in the "new version available" message. Keep the most important change first.

## [Unreleased]

### Added
- `docs/ARCHITECTURE.md` and `docs/CONTRIBUTING.md`.
- Release notes are taken from this changelog.

### Changed
- README restructured: how alerts work, full configuration reference, operations, troubleshooting.

## [1.7.1] - 2026-11-09

### Fixed
- Monitoring no longer stops when an external command hangs (docker, smartctl, df, dmesg, ps): every command times out after 10 s.
- A read error no longer looks like "all good": the alert state is kept and the reason is logged once.
- CPU usage race between the alert loop and `/cpu`; `/cpu` no longer shifts the alert's measurement window.
- `/command@BotName argument` lost its argument in group chats.

### Changed
- Logs use `log/slog` (`level=INFO msg=... key=value`). Set `BPIMON_DEBUG=1` to log full bot responses.
- Internal restructuring: `bot.Dispatcher`, `internal/system`, providers selected by kind. Module path is now `github.com/Kovalenkoyo81/bpimon`.
- CI runs staticcheck; `make lint` runs vet and staticcheck locally.

## [1.7.0] - 2026-11-09

### Added
- A stopped container triggers a question with buttons: **Stopped by admin** (silent until it runs again) or **Crashed** (reminders until it recovers). Only admins can answer; only the latest question keeps its buttons.
- The startup message shows the bpimon version; a newer GitHub release is announced once, with its notes.
- `/version`: version, build date, platform, uptime and the latest release.
- Separate "Docker daemon unavailable" alert; container alerts keep their state while the daemon is down.

### Fixed
- Alert state was never saved (`-state` had no effect): it is now written atomically on every change.
- A removed container (`docker rm`) produced a false "back to normal"; it is now reported as `removed`.
- Inline buttons did not work when Telegram had a stale `allowed_updates` setting for the bot token.

## [1.6.2] - 2026-10-08

### Security
- `install.sh` verifies the SHA-256 of every download against the release's `sha256sums.txt`; the systemd unit and config example come from the same release.

### Changed
- Static release binaries (`CGO_ENABLED=0`) built with the latest stable Go; CI runs gofmt, vet, race tests and govulncheck before every release.
- Dependabot keeps GitHub Actions and Go modules up to date.

## [1.6.1] - 2026-05-27

### Added
- Top CPU processes in `/cpu`, `/status` and CPU alerts.

## [1.5.2] - 2026-05-10

### Fixed
- SELinux (RHEL): restore the file context after installing the binary.

## [1.5.1] - 2026-05-10

### Added
- `install.sh`: one-command installation without Go.

## [1.5.0] - 2026-05-10

### Added
- x86 thermal zones (Intel, AMD); SD card commands hidden on machines without SD cards; release binaries on GitHub.

## [1.4.2] - 2026-05-10

### Added
- Docker container auto-discovery (`bpi-auto-discover`).

### Changed
- Local install is the primary flow; default cross-compile target is `linux/arm64`.

## [1.4.1] - 2026-05-10

First tagged release.

[Unreleased]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.7.1...HEAD
[1.7.1]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.6.2...v1.7.0
[1.6.2]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.6.1...v1.6.2
[1.6.1]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.5.2...v1.6.1
[1.5.2]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.5.1...v1.5.2
[1.5.1]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.4.2...v1.5.0
[1.4.2]: https://github.com/Kovalenkoyo81/bpimon/compare/v1.4.1...v1.4.2
[1.4.1]: https://github.com/Kovalenkoyo81/bpimon/releases/tag/v1.4.1
