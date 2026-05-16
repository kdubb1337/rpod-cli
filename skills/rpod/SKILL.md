---
name: rpod
description: Hand-crafted CLI for RunPod (GPU cloud), invoked as `rpod` (binary is intentionally NOT named `runpod` to avoid colliding with RunPod's official Python CLI on PyPI and their `runpodctl` Go tool). Use when the user asks to list/create/delete/start/stop a RunPod pod, inspect network volumes, check available GPU types and pricing, or do anything against the RunPod REST API. Triggers on "rpod", "runpod", "rent a gpu", "spin up a pod", "GPU pod", "A100/H100/4090/A6000", "container disk", "network volume", "runpod.io", or any request to launch/manage GPU workloads on RunPod. Prefer this skill over hitting the RunPod REST API or web dashboard directly — it gives agents typed exit codes, --json by default, --dry-run on mutations, and structured error envelopes.
---

# rpod

`rpod` is a Go CLI for the RunPod platform. Use it for any agent-driven workflow that needs to manage GPU pods, inspect network volumes, or browse GPU types and pricing.

> **Binary name:** `rpod`, not `runpod`. RunPod's own Python CLI on PyPI already owns the name `runpod` (and they ship `runpodctl` as their official Go tool), so this CLI uses `rpod` to avoid PATH collisions.

## Setup (once)

**Get an API key:** sign in at <https://www.runpod.io>, open **Settings → API Keys** (<https://www.runpod.io/console/user/settings>), click **+ API Key**, choose **Read & Write** (Read-only blocks mutations like `pod create/delete/start/stop`), and copy the `rpa_*` token — it is shown only once.

```
export RUNPOD_API_KEY=rpa_xxxxxxxxxxxxxxxxxx
rpod doctor                       # verify config + creds + API reach
```

Or persist the key:

```
rpod auth add rpa_xxx --profile default
rpod profile list
```

`gpu list` works without an API key (uses the public GraphQL endpoint); everything else requires one.

## Output rules (for agents)

- **stdout = data; stderr = progress and errors.** Always.
- **Default JSON when piped.** Pass `--human` to force tables in a pipe.
- **`--compact`** keeps only high-gravity fields (id, name, status, primary timestamp) — ~60–80% fewer tokens.
- **`--select=field1,field2`** for explicit projection.
- **Exit codes:**
  - `0` ok
  - `2` usage — fix invocation, don't retry
  - `3` not found — resource missing; don't retry
  - `4` auth — run `rpod doctor`; don't retry the same call
  - `5` api / 5xx — retry with backoff
  - `6` conflict — read response, decide. **Includes `error.code = "capacity"` when a GPU SKU is out of stock; retry with a different `--gpu-type` or `--data-center`.**
  - `7` rate limited — back off, retry later
  - `8` network — retry with backoff
  - `9` validation — fix input, retry (`valid_values` is often populated)
  - `124` timeout
- **`rpod agent-context`** prints the full structured schema (commands, flags, enums, exit codes). Read this once instead of crawling `--help`.

## Common commands

### Pods

```
# Read
rpod pod list --json
rpod pod list --status RUNNING --compact
rpod pod get <pod-id> --json

# Mutate (always dry-run first)
rpod pod create --image runpod/pytorch:2.4.0 \
  --gpu-type "NVIDIA GeForce RTX 4090" --gpu-count 1 --dry-run
rpod pod create --name my-pod --image runpod/pytorch:2.4.0 \
  --gpu-type "NVIDIA RTX A6000" --container-disk 20 \
  --volume-id vol_abc --volume-mount /workspace

rpod pod stop  <pod-id>
rpod pod start <pod-id>
rpod pod delete <pod-id> --force
```

#### Block-until-ready, then push files and run

`pod create --wait` collapses the create-then-poll loop into one call; `pod cp` and `pod exec` wrap ssh/scp with endpoint discovery so scripts don't have to grep `runtime.ports`.

```
# Create + wait for SSH and HTTP 8000 to be exposed, all in one go
rpod pod create --image runpod/pytorch:2.4.0 \
  --gpu-type "NVIDIA H200" --gpu-type "NVIDIA H200 NVL" \
  --public-ip --ssh-key-file ~/.ssh/id_ed25519.pub \
  --port 22/tcp --port 8000/http \
  --wait --wait-port 22 --wait-port 8000 --wait-timeout 5m

# Or: poll an already-created pod
rpod pod wait <pod-id> --port 22 --port 8000 --timeout 5m

# Inspect what's reachable (direct vs proxy SSH, HTTP proxy URLs)
rpod pod ssh-info <pod-id> --json
rpod pod url      <pod-id> 8000          # https://<id>-8000.proxy.runpod.net

# Run commands and copy files
rpod pod exec <pod-id> -- nvidia-smi
rpod pod exec <pod-id> --identity ~/.ssh/id_ed25519
rpod pod cp   ./inference.py <pod-id>:/workspace/inference.py
rpod pod cp -r ./src <pod-id>:/workspace/src
```

Direct SSH (`root@<ip>:<publicPort>`) is preferred when `--public-ip` is set. Otherwise rpod falls back to RunPod's proxy form (`<pod-id>@ssh.runpod.io`); force the proxy with `--use-proxy`.

#### Capacity errors (out-of-stock GPUs)

When every requested GPU SKU is out of capacity, the call exits with **code 6** and `error.code = "capacity"` in the structured envelope on stderr — that's the signal to retry with a different `--gpu-type` or `--data-center`. Never substring-match the message; branch on `error.code`.

```
rpod pod create ... --json 2>err.json
test $? = 6 && jq -e '.error.code == "capacity"' err.json && retry_with_other_gpu
```

You can also pass multiple `--gpu-type` flags and let RunPod pick the first SKU with capacity:

```
rpod pod create --gpu-type "NVIDIA H200" --gpu-type "NVIDIA H200 NVL" ...
```

> `pod logs` is not in the RunPod REST API. Use the RunPod web dashboard, the GraphQL API, or `rpod pod exec <id> -- tail -f /workspace/...` against a known log file.

### Volumes

```
rpod volume list --json
rpod volume get vol_abc --json
```

### GPU types

```
rpod gpu list --json
rpod gpu list --filter A6000           # substring match
rpod gpu list --min-memory 48 --secure # ≥48 GB on Secure Cloud
```

The `id` field from `gpu list` is what `pod create --gpu-type` expects.

## Workflows

### Spin up a fresh pod for an experiment

1. Discover the GPU: `rpod gpu list --filter 4090 --compact`
2. Dry-run the create: `rpod pod create --image runpod/pytorch:2.4.0 --gpu-type <id> --container-disk 20 --dry-run`
3. Real create + block until SSH is up: add `--public-ip --ssh-key-file ~/.ssh/id_ed25519.pub --wait --wait-port 22 --wait-timeout 5m`
4. Push code and run: `rpod pod cp ./infer.py <id>:/workspace/ && rpod pod exec <id> -- python /workspace/infer.py`
5. Tear down: `rpod pod delete <id> --force` or `rpod pod stop <id>` to preserve the volume

If a create fails with `code=capacity` (exit 6), retry with a different `--gpu-type` from `rpod gpu list`, or pass several `--gpu-type` flags in one call.

### Attach a persistent network volume

1. Confirm the volume exists: `rpod volume get vol_abc`
2. Pass `--volume-id vol_abc --volume-mount /workspace` on `pod create`
3. The volume's data center pins where the pod can land — RunPod will reject mismatched data centers

### Triage a "why is my pod not running" question

1. `rpod pod get <id> --json` — check `desiredStatus`, `lastStatusChange`
2. For container logs, use the RunPod dashboard (no REST endpoint)
3. If exit code 5 from the API, retry with backoff; if exit 4, check `rpod doctor`

## Installing this skill into another agent

The CLI can drop a copy of itself into any supported agent's skills directory:

```
rpod skill install claude            # ~/.claude/skills/rpod
rpod skill install --all             # every known agent
rpod skill list                      # show install status
rpod skill uninstall openhands       # remove from one agent
```

Default mode is `--mode=symlink`; use `--mode=copy` for a snapshot install.

## Notes

- Set `RUNPOD_API_KEY` and you can skip `rpod auth add` entirely.
- Mutating commands require `--force` or `--yes`; always try `--dry-run` first.
- Pod and volume IDs are case-sensitive opaque tokens — never normalize them.
- GPU type IDs are human-readable strings *with spaces*: `"NVIDIA GeForce RTX 4090"`, `"NVIDIA RTX A6000"`, `"NVIDIA A100 80GB PCIe"`, `"NVIDIA H100 80GB HBM3"`. Discover with `rpod gpu list`, don't guess — and quote them on the CLI.
- `gpu list` uses RunPod's GraphQL endpoint (REST doesn't expose GPU types); it works without an API key.
- `pod create --cloud-type SECURE` (default) vs `COMMUNITY` — Secure is more reliable, Community is cheaper.
- For scripting, prefer `--json --no-input`.
