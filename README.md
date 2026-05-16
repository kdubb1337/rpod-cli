# runpod

Agent-native CLI for [RunPod](https://runpod.io) — GPU pods, network volumes, and GPU type discovery.

## Install

```
brew install kdubb1337/tap/runpod
# or
go install github.com/kdubb1337/runpod-cli/cmd/runpod@latest
```

## Quick start

```
export RUNPOD_API_KEY=rpa_xxxxxxxxxxxxxxxxxx
runpod doctor
runpod gpu list --filter 4090
runpod pod list --json
```

Or persist the key as a profile:

```
runpod auth add rpa_xxx --profile default
runpod profile list
```

## Resources

- `runpod pod {list,get,create,delete,start,stop}` — manage GPU pods (REST `/pods`)
- `runpod volume {list,get}` — inspect persistent network volumes (REST `/networkvolumes`)
- `runpod gpu list` — discover GPU type IDs and pricing (GraphQL; no auth required)

## Output rules

- stdout = data, stderr = humans
- Auto-JSON when piped; `--human` forces tables in a pipe
- `--compact` for high-gravity fields only; `--select` for explicit projection
- Exit codes: `0` ok, `2` usage, `3` not-found, `4` auth, `5` api, `6` conflict, `7` rate-limit, `8` network, `9` validation, `124` timeout

See `runpod agent-context` for the full schema.

## For agents

A bundled `SKILL.md` ships with the binary. Find it with:

```
runpod skill-path
```

Or read it directly at `skills/runpod/SKILL.md` in this repo.

## Development

```
make tools     # install pinned dev tools
make           # build
make ci        # fmt + lint + test + build
```

See `AGENTS.md` for the full contributor guide.
