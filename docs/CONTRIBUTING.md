# Contributing

## Setup

Requires Go (the version in [go.mod](../go.mod) or newer) and `make`. Tests run on Linux; some read `/proc`.

```bash
git clone git@github.com:Kovalenkoyo81/bpimon.git
cd bpimon
make test
```

To run the bot locally without touching a real chat, set `telegram.enabled: false` in a copy of `config.yaml.example`. Alerts are then printed to stdout:

```bash
cp config.yaml.example config.yaml   # set telegram.enabled: false
go run ./cmd/bpimon -config config.yaml
```

## Checks

CI runs these on every pull request, and both jobs (`check`, `build`) must pass before merging:

```bash
gofmt -l .           # must print nothing
make lint            # go vet + staticcheck
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
make dist            # static binaries for all architectures
```

Imports are grouped as stdlib, third-party, then `github.com/Kovalenkoyo81/bpimon/...` (goimports with `-local github.com/Kovalenkoyo81/bpimon`).

## Code conventions

- **One responsibility per type and file.** Prefer a new small file over growing an existing one.
- **Small interfaces, declared by the consumer.** Optional capabilities are separate interfaces detected with a type assertion.
- **Never turn an error into "all good".** Alerts return `Unknown` with the reason.
- **Every external command has a timeout**: `monitor.output()` or `exec.CommandContext`.
- **Only `main` exits.** Libraries return errors.
- **Logging** with `log/slog` and key/value pairs: `slog.Warn("send question failed", "alert", name, "err", err)`.
- **Tests** for every behaviour change, run with `-race`. Use stubs for readers, the notifier and `bot.SystemActions`; tests must not run real `docker` or `systemctl`.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the package layout and how to add a metric.

## Git workflow

`main` is protected: changes go through a pull request with green `check` and `build`, and force pushes are blocked.

```bash
git switch -c fix/short-description
# commit, then
git push -u origin fix/short-description
# open a pull request on GitHub, merge when CI is green
git switch main && git pull && git branch -d fix/short-description
```

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `ci:`, `build:`, `style:`, with an optional scope such as `fix(alert):`.

Add a line to the `[Unreleased]` section of [CHANGELOG.md](../CHANGELOG.md) for every user-visible change.

## Releases

1. In `CHANGELOG.md`, rename `[Unreleased]` to the new version with today's date, add an empty `[Unreleased]` above it, and update the compare links at the bottom. Put the most important change first: the bot shows the first lines in its "new version available" message.
2. Merge that change to `main` through a pull request.
3. Tag the merged commit and push the tag:

   ```bash
   git switch main && git pull
   git log --oneline -1        # make sure this is the merge you want to release
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```

The release workflow runs the full CI, takes the release notes from the `CHANGELOG.md` section (and fails if it is missing), builds the binaries with `make dist` and publishes them with `sha256sums.txt`.

- Never move or reuse a published tag; release a new patch version instead.
- Pushing more than three tags at once does not trigger workflows.
- Check the notes locally with `scripts/release-notes.sh vX.Y.Z`.
