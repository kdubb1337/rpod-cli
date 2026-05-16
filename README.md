# rpod

Agent-native CLI for [RunPod](https://runpod.io) — GPU pods, network volumes, and GPU type discovery.

> The binary is `rpod`, not `runpod`. RunPod ships an official Python CLI on PyPI named `runpod` and a Go CLI named `runpodctl`; this project uses `rpod` to avoid PATH collisions. The repo and Go module path remain `runpod-cli`.

## Install

```
brew install kdubb1337/tap/rpod
# or
go install github.com/kdubb1337/runpod-cli/cmd/rpod@latest
```

## Getting an API key

1. Sign in at [runpod.io](https://www.runpod.io).
2. Open **Settings → API Keys**: <https://www.runpod.io/console/user/settings>
3. Click **+ API Key**, give it a name, choose **Read & Write** permission (Read-only works for `pod list/get`, `volume list/get`, and `gpu list`, but blocks all mutations).
4. Copy the token — it looks like `rpa_xxxxxxxxxxxxxxxxxxxxxxxxxxxx` and is shown **only once**. Store it in your password manager.

`gpu list` works without an API key (it uses the public GraphQL endpoint).

## Quick start

```
export RUNPOD_API_KEY=rpa_xxxxxxxxxxxxxxxxxx
rpod doctor
rpod gpu list --filter 4090
rpod pod list --json
```

Or persist the key as a profile:

```
rpod auth add rpa_xxx --profile default
rpod profile list
```

## Resources

- `rpod pod {list,get,create,delete,start,stop}` — manage GPU pods (REST `/pods`)
- `rpod volume {list,get}` — inspect persistent network volumes (REST `/networkvolumes`)
- `rpod gpu list` — discover GPU type IDs and pricing (GraphQL; no auth required)

## Output rules

- stdout = data, stderr = humans
- Auto-JSON when piped; `--human` forces tables in a pipe
- `--compact` for high-gravity fields only; `--select` for explicit projection
- Exit codes: `0` ok, `2` usage, `3` not-found, `4` auth, `5` api, `6` conflict, `7` rate-limit, `8` network, `9` validation, `124` timeout

See `rpod agent-context` for the full schema.

## For agents

A bundled `SKILL.md` ships with the binary. Find it with:

```
rpod skill-path
```

Or read it directly at `skills/rpod/SKILL.md` in this repo.

## Development

```
make tools     # install pinned dev tools
make           # build
make ci        # fmt + lint + test + build
```

See `AGENTS.md` for the full contributor guide.
