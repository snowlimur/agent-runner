# Backend Codemap

Updated: 2026-02-17

## Go CLI (agent-cli)

### Entry Point

`main.go` — Dispatches to `run` or `stats` subcommand.

### Package: cli

Implements the `run` and `stats` commands.

**run command** (`run.go`):
1. Parses flags: `--json`, `--model`, `--file`, `--pipeline`
2. Loads `.agent-cli/config.toml` via `config.Load()`
3. Calls `runner.RunDockerStreaming()` with stream hooks
4. Stream hooks parse each stdout line as JSON, update `ProgressPrinter`, collect metrics
5. Extracts final result (single prompt) or pipeline result from stream
6. Persists `RunRecord` + artifacts to `.agent-cli/runs/<timestamp>-<id>/`
7. Prints summary or raw JSON

**stats command** (`stats.go`):
- Aggregates all run records from `.agent-cli/runs/`
- Outputs table or JSON with token counts, costs, tool usage, event counts

**progress printer** (`progress.go`):
- Formats real-time progress lines: `HH:MM:SS [label] message`
- Tracks tool lifecycle (start/done/output), todo transitions, pipeline task bindings
- Pipeline mode prefixes lines with `[stage_id/task_id]`

### Package: config

Loads and validates `.agent-cli/config.toml` using a hand-written TOML parser.

**Sections:**
- `[docker]` — image, model (sonnet|opus), enable_dind
- `[auth]` — github_token, claude_token
- `[workspace]` — source_workspace_dir (absolute path)
- `[git]` — user_name, user_email

**Derived paths:**
- `ConfigPath()` → `<cwd>/.agent-cli/config.toml`
- `RunsDir()` → `<cwd>/.agent-cli/runs`

### Package: result

Parses Claude Code streaming JSON output.

**Stream event types:** `system`, `assistant`, `user`, `pipeline_event`, `result`

**Key types:**
- `StreamEvent` — Union envelope for all event types
- `AgentResult` — Final result with usage, cost, model usage breakdown
- `NormalizedMetrics` — Flattened token/cost metrics with per-model breakdown
- `ParsedResult` — Raw JSON + parsed agent result + normalized metrics

### Package: runner

Manages Docker container lifecycle using the Docker Engine API.

**Flow:** cleanup stale → pull image → create → start → stream logs → wait → cleanup

**Features:**
- Stale container cleanup by CWD hash label
- Host network mode (bridge for DinD)
- Read-only source workspace bind mount
- Graceful interrupt handling with signal-aware cleanup
- Log demuxing (Docker multiplexed stdout/stderr)

### Package: stats

Persists and aggregates run records.

**Storage:** `.agent-cli/runs/<YYYYMMDDTHHMMSS>-<hex_id>/`
- `stats.json` — Full run record
- `prompt.md` — Prompt content
- `output.log` — Combined stdout+stderr

**Aggregation:** Sums across all runs — tokens, cost, duration, tool use counts, event counts, per-model breakdowns.

## TypeScript Entrypoint (images/entrypoint)

### Entry Point

`entrypoint.ts` → `main.ts:runEntrypoint()`

### Startup Sequence

1. Parse CLI args (commander): `--model`, `--pipeline`, `--debug`, `[taskArgs...]`
2. Copy read-only source mount → writable `/workspace`
3. Authenticate GitHub CLI + configure git
4. Optionally start Docker-in-Docker daemon
5. Resolve execution mode:
   - **Pipeline mode:** Parse YAML plan → execute stages/tasks
   - **Prompt mode:** Run single `claude` process
   - **Interactive mode:** Launch `claude` with no prompt

### Pipeline Executor (`pipeline-executor.ts`)

Executes a validated `PipelinePlan`:
- Iterates stages sequentially
- Each stage runs tasks sequentially or in parallel (worker pool)
- Emits structured `pipeline_event` JSON on stdout for CLI parsing
- Prepares isolated workspaces per task (shared / worktree / snapshot_ro)
- Binds Claude sessions to tasks via `task_session_bind` events

### Pipeline Plan Parser (`pipeline-plan.ts`)

Validates YAML pipeline plans:
- Version check (`v1`)
- Cascading defaults: plan → stage → task (model, verbosity, on_error, workspace)
- Prompt resolution: inline `prompt` or `prompt_file` (relative to /workspace)
- Safety: parallel tasks with shared workspace must declare `read_only` or `allow_shared_writes`

### DinD Module (`dind.ts`)

Optional Docker-in-Docker support:
- Launches `dockerd` via `sudo` with configurable storage driver
- Falls back from overlay2 → vfs
- Health-checks via `docker info`
- Graceful shutdown with SIGTERM → wait → SIGKILL
