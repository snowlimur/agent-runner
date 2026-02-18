# Runbook

Updated: 2026-02-17

## Build & Deploy

### Build CLI

```bash
task cli:build
# Binary: agent-cli/bin/agent-cli
```

### Build Docker Images

```bash
# All images
task image:build:all

# Individual
task image:build:base    # claude:latest
task image:build:go      # claude:go
task image:build:rust    # claude:rust
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
agent-cli help
```

## Run Artifacts

Each `agent-cli run` persists artifacts to `.agent-cli/runs/<timestamp>-<id>/`:
- `stats.json` (run record; prompt data is not stored)
- `output.log` (combined stdout/stderr)

```bash
# List runs
ls -la .agent-cli/runs/

# View latest run stats
cat .agent-cli/runs/$(ls .agent-cli/runs/ | tail -1)/stats.json | jq .

# Aggregate stats
agent-cli stats --json
```

## Container Lifecycle

### Stale Container Cleanup

agent-cli automatically cleans up non-running containers matching the current CWD hash before each run. Manual cleanup:

```bash
# List managed containers
docker ps -a --filter label=agent-cli.managed=true

# Force remove
docker rm -f $(docker ps -aq --filter label=agent-cli.managed=true)
```

### Image Pull

agent-cli attempts a best-effort image pull before each run. If the registry is unreachable, the local image is used.

### Idle Timeouts

`agent-cli` enforces idle-based timeouts:
- `docker.run_idle_timeout_sec` (default `7200`) for the whole run
- `docker.pipeline_task_idle_timeout_sec` (default `1800`) for pipeline tasks

Pipeline YAML can override per-task idle timeout via `task_idle_timeout_sec` on defaults/stage/task.
Idle timeout is measured from the last task stdout/stderr activity and resets on each new output chunk.

## Docker-in-Docker

When `enable_dind = true`:

- Container runs in **privileged** mode with **bridge** network
- Entrypoint starts `dockerd` via `sudo` before running Claude
- Falls back from `overlay2` to `vfs` storage driver
- Startup timeout: `DIND_STARTUP_TIMEOUT_SEC` (default 45s)

### Troubleshooting DinD

```bash
# Inside container, check daemon status
docker info

# View daemon logs (if --debug enabled)
# Logs appear prefixed with [dockerd]
```

## Common Issues

### Config file not found

```
error: config file not found: /path/.agent-cli/config.toml
```

Create `.agent-cli/config.toml` with required sections. See CONTRIB.md for format.

### Docker image not available

```
error: create container: ...
```

Build images first: `task image:build:all`

### Pipeline parse error

```
error: pipeline result event not found in stream output
```

Check that the YAML plan file is valid and references existing prompt files. Review the raw output in `.agent-cli/runs/<latest>/output.log`.

### Pipeline template variable errors (`--var`)

Inline `tasks[].prompt` can contain placeholders like `{{A_VAR}}`, supplied via:

```bash
agent-cli run --pipeline plan.yml --var A_VAR=value --var B_VAR=value
```

Failures are explicit:
- `Missing template vars for <stage>/<task>: ...` when a placeholder has no value
- `Unused template vars: ...` when a passed `--var` key is not used
- argument errors for invalid key format (must be `UPPER_SNAKE`) or duplicate key

Notes:
- `--var` works only with `--pipeline`
- `prompt_file` content is not templated

### Run idle timeout

```
error: run idle timeout: no log activity for ...
```

Increase `docker.run_idle_timeout_sec` in `.agent-cli/config.toml` if the run is expected to stay silent for long periods.

### Pipeline task idle timeout

When a pipeline task stops producing output longer than the configured idle timeout:
- entrypoint emits `pipeline_event` `task_timeout`
- task finishes with `status=error` and timeout message
- `agent-cli` returns a detailed error with `stage/task`

### Interrupted run

If a run is interrupted (Ctrl+C / SIGTERM), agent-cli:
1. Signals the container to stop (10s timeout)
2. Force-removes the container
3. Saves partial run record with status `error` and error_type `interrupted`

### GitHub auth failure

```
Entrypoint failed: ...
```

Ensure `auth.github_token` in config is valid and has required scopes. The entrypoint runs `gh auth status` before proceeding.

## Rollback

No deployment mechanism — images and CLI are built locally. To revert:

```bash
git checkout <previous-commit>
task install
```
