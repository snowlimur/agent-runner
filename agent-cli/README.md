# agent-cli

`agent-cli` is a small Go CLI for launching agent Docker images and collecting run statistics.

## Configuration

`agent-cli` reads config from the current working directory:

`./.agent-cli/config.toml`

Example:

```toml
[docker]
image = "claude:go"
model = "opus"
enable_dind = false
run_idle_timeout_sec = 7200
pipeline_task_idle_timeout_sec = 1800

[auth]
github_token = "..."
claude_token = "..."

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "Your Name"
user_email = "you@example.com"
```

`docker.model` is optional. If omitted, `opus` is used.  
`docker.enable_dind` is optional. If omitted, `false` is used.
`docker.run_idle_timeout_sec` is optional. If omitted, `7200` is used.
`docker.pipeline_task_idle_timeout_sec` is optional. If omitted, `1800` is used.

## Commands

Run with inline prompt:

```bash
agent-cli run "build and test the project"
```

Run with model override:

```bash
agent-cli run --model sonnet "build and test the project"
```

By default, `run` prints compact live progress from stream-json events:
- init line with session/model
- tool start/done lines with `tool_use_id`
- in pipeline mode, each progress line is prefixed with `[stage_id/task_id]`
- todo status transitions
- final summary with `status` and token usage (`input/cache/output/total`)

Run with prompt file:

```bash
agent-cli run --file ./prompt.txt
```

Print raw JSON result:

```bash
agent-cli run --json "build and test the project"
```

`--json` prints only the final `type=result` JSON object (no live progress lines).

Show aggregated statistics:

```bash
agent-cli stats
```

Show statistics as JSON:

```bash
agent-cli stats --json
```

## Idle timeouts

`agent-cli` enforces idle-based timeouts (not wall-clock hard caps):
- run-level idle timeout (`docker.run_idle_timeout_sec`) for the whole container run
- pipeline task idle timeout (`docker.pipeline_task_idle_timeout_sec`) as the default for pipeline tasks

Idle timeout is measured from the **last stdout/stderr activity** and resets whenever new task output appears.

Pipeline plans can override task idle timeout:

```yaml
version: v1
defaults:
  task_idle_timeout_sec: 1800
stages:
  - id: main
    task_idle_timeout_sec: 1200
    tasks:
      - id: build_run
        task_idle_timeout_sec: 600
        prompt: "Build and run the project."
```

## DinD mode

`agent-cli` can enable Docker-in-Docker for runner containers when you need nested `docker`/`docker compose` in agent tasks.

Enable it in `./.agent-cli/config.toml`:

```toml
[docker]
image = "claude:go"
enable_dind = true
```

What this does:
- sets `ENABLE_DIND=1` inside the runner container
- starts runner container in `--privileged` mode
- runner entrypoint starts internal `dockerd` and exposes `DOCKER_HOST=unix:///var/run/docker.sock`

Notes:
- this is rootful DinD and requires privileged containers on the host
- privileged mode is security-sensitive; use only in trusted environments

## Stats storage

Each run creates a dedicated directory in:

`./.agent-cli/runs/<timestamp>-<run_id>`

Each run directory contains:
- `stats.json` with run metadata, normalized metrics, per-task pipeline usage metrics (when available), and error details when present (prompt data is not stored)
- `output.log` with raw container output (`stdout` followed by `stderr`)

Timestamp format is UTC compact:
`YYYYMMDDTHHMMSS.nnnnnnnnnZ`.

For pipeline runs, each task in `pipeline.tasks` can include `normalized` when usage data is available from stream `result` events. Task `normalized` includes:
- token counters (`input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`, `output_tokens`)
- cost/search counters (`cost_usd`, `web_search_requests`)
- per-model breakdown in `by_model` with the same fields
