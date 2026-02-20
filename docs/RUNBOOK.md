# Runbook

Updated: 2026-02-19

## Build & Deploy

### Build CLI

```bash
task cli:build
# Binary: agent-cli/bin/agent-cli

task cli:install
# Installs to $(go env GOPATH)/bin/agent-cli
```

### Build Docker Images

```bash
task image:build:all     # base + go (rust commented out)
task image:build:base    # claude:latest
task image:build:go      # claude:go
```

### Build Entrypoint

```bash
task image:entrypoint:build
# Output: images/entrypoint/dist/
```

## Health Check

```bash
# Verify Docker is available
docker info

# Verify images exist
docker images | grep claude

# Verify CLI binary
agent-cli --help
```

## Run Artifacts

Each `agent-cli run` persists to `.agent-cli/runs/<YYYYMMDDTHHMMSS>-<hex_id>/`:
- `stats.json` — full RunRecord with status, metrics, error info
- `output.ndjson` — all valid JSON object stdout lines
- `output.log` — all non-JSON lines (stdout first, then stderr)

```bash
# List runs (most recent last)
ls .agent-cli/runs/

# Inspect latest run record
cat .agent-cli/runs/$(ls .agent-cli/runs/ | tail -1)/stats.json | jq .

# Aggregate stats
agent-cli stats
agent-cli stats --json
```

## Container Lifecycle

### Stale Container Cleanup

agent-cli auto-cleans non-running containers matching the current CWD hash before each run.

```bash
# List managed containers
docker ps -a --filter label=agent-cli.managed=true

# Force remove all
docker rm -f $(docker ps -aq --filter label=agent-cli.managed=true)
```

### Idle Timeouts

- `docker.run_idle_timeout_sec` (default 7200) — whole-run idle timeout
- `docker.pipeline_task_idle_timeout_sec` (default 1800) — default per-node idle timeout (pipeline mode)

Pipeline v2 node-level overrides:
- `defaults.agent_idle_timeout_sec` / `defaults.command_timeout_sec` in plan YAML
- `nodes.<id>.run.idle_timeout_sec` (kind: agent)
- `nodes.<id>.run.timeout_sec` (kind: command)

Idle timeout resets on each new stdout/stderr chunk.

## Docker Runtime Modes

| Mode | Config | Container | Use Case |
|------|--------|-----------|----------|
| `none` | default | standard | most workloads |
| `dind` | `docker.mode = "dind"` | privileged + bridge network | workloads that need Docker |
| `dood` | `docker.mode = "dood"` | Docker socket mounted | workloads that share host Docker |

DinD falls back from `overlay2` to `vfs` storage driver if needed. Startup timeout: `DIND_STARTUP_TIMEOUT_SEC` (default 45s).

## Rollback

No deployment mechanism — built locally. To revert:

```bash
git checkout <previous-commit>
task install
```

## Common Issues

### Config file not found

```
error: config file not found: /path/.agent-cli/config.toml
```

Create `.agent-cli/config.toml`. See `docs/data.md` for the full schema. Required fields: `docker.image`, `auth.github_token`, `auth.claude_token`, `workspace.source_workspace_dir`, `git.user_name`, `git.user_email`.

### Docker image not available

```
error: create container: ...
```

Build images first: `task image:build:all`

### Pipeline result not found

```
error: pipeline result event not found in stream output
```

The entrypoint didn't emit a `pipeline_result` JSON object. Check:
- YAML plan file is valid (`version: v2`, all referenced nodes exist)
- Decision schema files exist at the paths specified in `decision.schema_file`
- Review `.agent-cli/runs/<latest>/output.ndjson` for pipeline_events and `.agent-cli/runs/<latest>/output.log` for error text

### Pipeline template variable errors

```
Missing template vars for <node_id>: ...
Unused template vars: ...
```

Every `{{UPPER_SNAKE}}` placeholder in inline `prompt` or `cmd` fields must have a matching `--var KEY=value`. Extra `--var` keys that are not used also fail. `prompt_file` content is not templated.

### Run idle timeout

```
error: run idle timeout: no log activity for ...
```

Increase `docker.run_idle_timeout_sec` in `.agent-cli/config.toml`. For pipeline node timeouts, increase `defaults.agent_idle_timeout_sec` in the pipeline YAML.

### Pipeline node idle timeout

When a node stops producing output longer than its idle timeout:
- Entrypoint emits `pipeline_event` `node_timeout`
- Node finishes with `status=error`
- agent-cli returns error with `node_id/node_run_id`

### Interrupted run

On Ctrl+C / SIGTERM, agent-cli:
1. Signals container to stop (10s timeout)
2. Force-removes container
3. Saves partial RunRecord with `status=error`, `error_type=interrupted`

### GitHub auth failure

```
Entrypoint failed: ...
```

Ensure `auth.github_token` in config is valid and has required scopes (`repo`, `read:org`). The entrypoint runs `gh auth status` before proceeding.

### Decision payload error

```
error: decision payload must be a JSON object in result.structured_output
```

The agent node's Claude invocation did not return a valid `structured_output` object. Verify the `decision.schema_file` is a valid JSON Schema and that the agent prompt instructs Claude to return a decision matching the schema.
