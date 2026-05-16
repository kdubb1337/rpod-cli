# Changelog

## v0.4.0 (2026-05-16)

### Breaking

- `rpod skill-path` renamed to `rpod skill path` — folded into a new `skill`
  command group alongside `install`, `uninstall`, and `list`.

### Added

- `rpod skill install [agent...]` symlinks (or `--mode=copy` copies) the
  bundled SKILL.md into one or more agent skills directories. Known agents:
  `claude` (`~/.claude/skills`), `codex` (`~/.codex/skills`), `gemini`
  (`~/.gemini/skills`), `openhands` (`~/.openhands/microagents`), `agents`
  (`~/.agents/skills` cross-agent universal path). Override any path via
  `$RPOD_SKILLS_<AGENT>`. Supports `--all`, `--dir <path>`, `--force`,
  `--dry-run`.
- `rpod skill uninstall [agent...]` removes the installed skill.
- `rpod skill list` reports install status across all known agents,
  including whether each installed symlink still resolves to our source.

## v0.3.0 (2026-05-16)

### Breaking

- GitHub repo renamed `kdubb1337/runpod-cli` → `kdubb1337/rpod-cli`. GitHub
  301-redirects the old URL, but update your `git remote` and `go install`
  paths.
- Go module path renamed `github.com/kdubb1337/runpod-cli` →
  `github.com/kdubb1337/rpod-cli`. Anyone consuming this as a library (none
  expected) must update import paths. `go install github.com/kdubb1337/rpod-cli/cmd/rpod@latest`.
- Local clone dir convention now `~/git/cli/rpod` (matches the binary name).

### Unchanged from v0.2.0

- Binary `rpod`, config dir `~/.rpod/`, Homebrew formula `rpod`, GHCR image
  `ghcr.io/kdubb1337/rpod`.
- Env vars `RUNPOD_API_KEY`, `RUNPOD_ACCOUNT` (RunPod's conventions).
- All commands and flags identical.

## v0.2.0 (2026-05-16)

### Breaking

- Binary renamed `runpod` → `rpod`. The name `runpod` is already owned by
  RunPod's official Python CLI on PyPI, which silently shadowed our binary on
  any system where both were installed. Run `brew install kdubb1337/tap/rpod`
  to get the new binary.
- Config directory renamed `~/.runpod/` → `~/.rpod/`. If you'd already saved
  profiles, run `mv ~/.runpod ~/.rpod`.
- Homebrew formula renamed `runpod` → `rpod`. `brew uninstall runpod` then
  `brew install kdubb1337/tap/rpod`.
- Docker image renamed `ghcr.io/kdubb1337/runpod` → `ghcr.io/kdubb1337/rpod`.

### Unchanged

- Go module path stayed `github.com/kdubb1337/runpod-cli` in v0.2.0 (renamed
  to `rpod-cli` in v0.3.0).
- Env vars stay `RUNPOD_API_KEY`, `RUNPOD_ACCOUNT` (those are RunPod's
  conventions, not ours).
- All commands and flags identical.

## v0.1.0 (2026-05-16)

- Initial release. `pod`, `volume`, `gpu` resources; `doctor`, `agent-context`,
  `profile`, `auth`, `skill-path`; bundled SKILL.md.
