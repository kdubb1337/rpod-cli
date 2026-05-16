# Changelog

## v0.6.1 (2026-05-16)

### Fixed

- `rpod pod ssh-info`, `pod exec`, `pod cp`, and `pod wait` now work on
  pods created/resumed via the RunPod web UI. RunPod's REST
  `GET /pods/<id>` returns port info in two different shapes: API-created
  pods get `runtime.ports[]`, while UI-created and resumed pods get an
  empty `runtime.ports` plus a top-level `publicIp` + `portMappings`
  object (`{"22": 12885}`). Endpoint discovery previously only consulted
  `runtime.ports`, so UI pods silently fell back to the slow
  `ssh.runpod.io` proxy even when a direct TCP endpoint existed.
  `Pod.DeriveSSHInfo` and `Pod.HasPort` now consult both shapes and
  cross-reference the spec `Ports` list (`"8000/http"`) to classify
  HTTP vs TCP ports from `portMappings`.

## v0.6.0 (2026-05-16)

### Added

- `rpod datacenter list` (alias `rpod dc list`) — surfaces RunPod's
  datacenters and the GPU SKUs available in each. Backed by the GraphQL
  `dataCenters { gpuAvailability { ... } }` query; works without an API
  key. Inverts `gpu list`: instead of "what does this GPU cost?" it
  answers "where is this GPU?".
- Filters: `--gpu-type` (substring, prunes per-DC GPU list and drops DCs
  with no matches), `--location`, `--listed-only`, `--storage`,
  `--in-stock` (drops `available=false` entries).

### Changed

- `agent-context` schema bumped to v3 to cover the new `datacenter`
  command surface.

## v0.5.0 (2026-05-16)

### Added

- `rpod pod wait <id>` — block until the pod reaches `--status` (default
  `RUNNING`) and every `--port` (repeatable) is published in `runtime.ports`.
  Exponential backoff capped at `--interval`, `--timeout` defaults to 5m.
  Exits `124` on timeout, `3` if the pod disappears.
- `rpod pod create --wait [--wait-port N ...] [--wait-timeout 5m]` — chains
  create→poll so a single call returns a fully-reachable pod.
- `rpod pod ssh-info <id>` — derives every reachable endpoint from
  `runtime.ports`. Returns `{direct, proxy, http_proxy, public_tcp}` with a
  ready-to-run `ssh ...` command string. Proxy SSH
  (`<podID>@ssh.runpod.io`) is always populated as the fallback.
- `rpod pod url <id> <port>` — prints
  `https://<podID>-<port>.proxy.runpod.net` on a single stdout line; pipe
  directly into `curl`.
- `rpod pod exec <id> -- <cmd...>` — runs a command (or opens an
  interactive shell) over SSH. Auto-discovers the endpoint, attaches the
  user's stdio, propagates the remote exit code.
- `rpod pod cp <src> <dst>` — scp wrapper with the same endpoint discovery
  (`-r` for recursive). Either side may be `<podID>:<path>`.
- `rpod pod create --ssh-key-file <path>` — reads a public key and sets the
  `PUBLIC_KEY` env var so the container's sshd accepts it.
- `--gpu-type` is now repeatable on `pod create`. RunPod picks the first
  SKU with capacity, so callers no longer need to loop client-side.
- Capacity exhaustion gets a typed envelope: `error.code = "capacity"`,
  exit `6`. Detected from RunPod's free-text message ("no instances
  available", "out of capacity", etc.) so agents can branch on the code
  instead of substring-matching.

### Changed

- `agent-context` schema bumped to v2: adds `error_codes` map and exposes
  the new pod subcommands and flags.
- `api.Pod` gains a `Runtime` field (`{ports, gpus, container}`) populated
  by the REST API once a pod is running.

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
