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
