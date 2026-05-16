# Changelog

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

- Go module path stays `github.com/kdubb1337/runpod-cli`.
- GitHub repo stays `kdubb1337/runpod-cli`.
- Env vars stay `RUNPOD_API_KEY`, `RUNPOD_ACCOUNT` (those are RunPod's
  conventions, not ours).
- All commands and flags identical.

## v0.1.0 (2026-05-16)

- Initial release. `pod`, `volume`, `gpu` resources; `doctor`, `agent-context`,
  `profile`, `auth`, `skill-path`; bundled SKILL.md.
