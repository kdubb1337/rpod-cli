---
name: runpod
description: |
  runpod is a hand-crafted CLI for <service>. Use this skill whenever the user wants to <primary verbs> for <service>, mentions <service-specific keywords>, asks to <common workflows>, or runs `runpod ...`. Prefer this skill over hitting the <service> REST API directly or opening the <service> web dashboard.
---

# runpod

Use `runpod` for <one-line scope>. Requires <auth method> setup.

## Setup (once)

```
runpod auth add <id>
runpod auth list
runpod doctor          # verify config + creds + API
```

## Output rules (for agents)

- **stdout = data; stderr = progress and errors.** Always.
- **Default JSON when piped.** Pass `--human` to force tables in a pipe.
- **`--compact`** keeps only high-gravity fields (id, name, status, primary timestamp) — ~60–80% fewer tokens.
- **`--select=field1,field2`** for explicit projection.
- **Exit codes:**
  - `0` ok
  - `2` usage error — fix invocation
  - `3` not found — resource doesn't exist; don't retry
  - `4` auth — run `runpod doctor`; don't retry the same call
  - `5` api / 5xx — retry with backoff
  - `6` conflict — read response, decide
  - `7` rate limited — honor Retry-After in stderr
  - `8` network — retry with backoff
  - `9` validation — fix input, retry (often `valid_values` is populated)
  - `124` timeout
- **`runpod agent-context`** prints the full structured schema (all commands, flags, enums, exit codes). Read this once instead of crawling `--help`.

## Common commands

```
# Read
runpod <resource> get <id> --json
runpod <resource> get <id> --compact
runpod <resource> raw <id> --pretty       # full upstream response

# List (bounded)
runpod <resource> list --since 24h --limit 25 --json
runpod <resource> list --cursor <prev-cursor>

# Mutate (always dry-run first)
runpod <resource> create --... --dry-run
runpod <resource> create --...
runpod <resource> delete <id> --force
```

## Workflows

### <Workflow 1: e.g. Triage today's errors>

1. Save the profile once: `runpod profile save default --org acme`
2. Investigate: `runpod <resource> list --since 24h --json`
3. Drill in: `runpod <resource> get <id> --json`

### <Workflow 2>

...

## Notes

- Set `RUNPOD_ACCOUNT=<id>` to avoid repeating `--account` on every call.
- For scripting, prefer `--json --no-input`.
- Mutating commands require `--force` or `--yes`; always try `--dry-run` first.
- IDs are case-sensitive opaque tokens — never normalize them; only normalize *names* for lookup.
