# Backend Codemap

Updated: 2026-02-18

## Go CLI (agent-cli)

### Entry Point

`main.go` - Dispatches to `run` or `stats` subcommand.

### Package: cli

Implements the `run` and `stats` commands.

**run command** (`run.go`):
1. Parses flags: `--json`, `--model`, `--file`, `--pipeline`, `--var`
2. Loads `.agent-cli/config.toml` via `config.Load()`
3. Calls `runner.RunDockerStreaming()` with stream hooks
4. Stream hooks parse stdout JSON lines, feed progress TUI, and accumulate metrics
5. Extracts final `result` (single prompt) or `pipeline_result` (pipeline mode)
6. Persists `RunRecord` + artifacts to `.agent-cli/runs/<timestamp>-<id>/`
7. Prints TUI summary or raw JSON

**Pipeline mode specifics (v2):**
- consumes `pipeline_event` node-level events (`node_start`, `node_session_bind`, `node_finish`, `transition_taken`, ...)
- binds `session_id -> node_run_id` for usage attribution
- enriches `pipeline.node_runs[].normalized` from stream `result` events

**stats command** (`stats.go`):
- Aggregates all run records from `.agent-cli/runs/`
- Outputs table or JSON with token counts, costs, durations, and per-model totals

### Package: config

Loads and validates `.agent-cli/config.toml` using a hand-written TOML parser.

**Sections:**
- `[docker]` - image, model (sonnet|opus), mode, dind_storage_driver, run_idle_timeout_sec, pipeline_task_idle_timeout_sec
- `[auth]` - github_token, claude_token
- `[workspace]` - source_workspace_dir (absolute path)
- `[git]` - user_name, user_email

### Package: result

Parses Claude Code streaming JSON output.

**Stream event types:** `system`, `assistant`, `user`, `pipeline_event`, `result`

**Pipeline v2 parsed fields:**
- node-level ids: `node_id`, `node_run_id`, `session_id`
- transition info: `from_node`, `to_node`, `when`
- plan summary: `entry_node`, `terminal_node`, `terminal_status`, `node_run_count`, `failed_node_count`, `exit_code`

### Package: runner

Manages Docker container lifecycle using the Docker Engine API.

**Flow:** cleanup stale -> pull image -> create -> start -> stream logs -> wait -> cleanup

### Package: stats

Persists and aggregates run records.

**Storage:** `.agent-cli/runs/<YYYYMMDDTHHMMSS>-<hex_id>/`
- `stats.json` - Full run record
- `output.ndjson` - Valid JSON object logs (NDJSON)
- `output.log` - Non-JSON-object lines (stdout first, then stderr)

## TypeScript Entrypoint (images/entrypoint)

### Entry Point

`entrypoint.ts` -> `main.ts:runEntrypoint()`

### Startup Sequence

1. Parse CLI args (`--model`, `--pipeline`, `--debug`, `[taskArgs...]`)
2. Copy source mount to writable `/workspace`
3. Configure GitHub auth + git identity
4. Optionally start DinD
5. Resolve mode:
   - **Pipeline mode (v2):** parse graph plan and execute state machine
   - **Prompt mode:** run single Claude process
   - **Interactive mode:** run Claude interactive

### Pipeline Plan Parser (`pipeline-plan.ts`)

Validates YAML v2 graph DSL:
- `version: v2`, `entry`, `nodes`
- executable nodes (`run + transitions`) vs terminal nodes (`terminal_status + exit_code`)
- node kinds: `agent` and `command`
- compile-time validation of `transitions[].when` expressions
- `decision.schema_file` loading for `kind: agent`

### Pipeline Executor (`pipeline-executor.ts`)

Executes the plan as a state machine:
- starts from `entry` and follows first matching transition
- enforces `max_iterations` and `max_same_node_hits`
- emits node-level `pipeline_event` stream
- always runs agent nodes with `--verbose --output-format stream-json --json-schema <schema-json>`
- parses final `type=result` payload into decision JSON and validates schema
- emits final `pipeline_result` with `node_runs[]`
