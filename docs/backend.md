# Backend Codemap

Updated: 2026-02-19

## Go CLI (agent-cli)

### Entry Point

`main.go` — dispatches to `run` or `stats` subcommand via `RunCommand` / `StatsCommand`.

### Package: cli

**`RunCommand`** (`run.go`):
1. Parse flags: `--json`, `--model`, `--file`, `--pipeline`, `--var`, `--debug`
2. Load `.agent-cli/config.toml` via `config.Load()`
3. Call `runner.RunDockerStreaming()` with stream hooks
4. Stream hooks: parse stdout JSON lines → feed `ProgressTUI`, accumulate `NormalizedMetrics`, bind session→node for pipeline usage attribution
5. Extract final result: `pipeline_result` (pipeline mode) or `AgentResult` (single prompt)
6. Persist `RunRecord` + `output.ndjson` + `output.log` to `.agent-cli/runs/<timestamp>-<id>/`
7. Print TUI summary or raw JSON

**Pipeline v2 specifics:**
- Consumes `pipeline_event` stream events (`node_start`, `node_session_bind`, `node_finish`, `transition_taken`, ...)
- Binds `session_id → node_run_id` for per-node usage attribution
- Scans stdout in reverse for the last `pipeline_result` JSON object

**`StatsCommand`** (`stats.go`):
- Aggregates all `stats.json` records from `.agent-cli/runs/`
- Outputs table or JSON with token counts, costs, durations, per-model totals

### Package: config

Hand-written TOML parser (no external deps). Loads and validates `.agent-cli/config.toml`.

**Sections:**
- `[docker]` — `image`, `model` (sonnet|opus), `mode` (none|dind|dood), `dind_storage_driver`, `run_idle_timeout_sec` (default 7200), `pipeline_task_idle_timeout_sec` (default 1800)
- `[auth]` — `github_token`, `claude_token`
- `[workspace]` — `source_workspace_dir` (absolute path, required)
- `[git]` — `user_name`, `user_email`

### Package: result

Parses Claude Code's streaming JSON protocol.

**`ParseStreamLine(line)`** — classifies each stdout line:
- `StreamLineNonJSON` — plain text
- `StreamLineJSONEvent` — dispatches by `type`: `system`, `assistant`, `user`, `pipeline_event`, `result`
- `StreamLineInvalidJSON` — malformed JSON

**`ExtractFinalResultFromStream(lines)`** — finds last `type=result` event for single-prompt runs.

**`PipelineEvent`** fields: `event`, `node_id`, `node_run_id`, `session_id`, `status`, `kind`, `model`, `from_node`, `to_node`, `when`, `exit_code`, `error_message`, `idle_timeout_sec`, `timeout_sec`.

### Package: runner

Manages Docker container lifecycle via Docker Engine API.

**Flow:** cleanup stale containers (by CWD hash label) → best-effort pull → create → start → stream logs (idle timeout enforced) → wait → cleanup.

**Sentinel errors:** `ErrInterrupted` (Ctrl+C/SIGTERM), `ErrIdleTimeout` (no stdout/stderr for N sec).

**Docker modes:** `none` (standard), `dind` (privileged + DinD daemon), `dood` (Docker socket mount).

Container labels: `agent-cli.managed=true`, `agent-cli.cwd_hash=<sha256>`.

### Package: stats

Persists and aggregates run records.

**Storage per run:** `.agent-cli/runs/<YYYYMMDDTHHMMSS>-<hex_id>/`
- `stats.json` — full `RunRecord`
- `output.ndjson` — valid JSON object lines (NDJSON)
- `output.log` — non-JSON lines (stdout first, then stderr)

---

## TypeScript Entrypoint (images/entrypoint)

### Entry Point

`entrypoint.ts` → `main.ts:runEntrypoint()`

### Startup Sequence

1. `resolveEntrypointArgs()` — parse `--model`, `--pipeline`, `--debug`, `[taskArgs...]`
2. `prepareWorkspaceFromReadOnlySource()` — copy read-only source mount → `/workspace`
3. `configureGit()` — set `user.name`/`user.email`, force `ssh://git@github.com/` to `https://github.com/`, add `safe.directory=/workspace`
4. `ensureGitHubAuthAndSetupGit()` — run `gh auth status`, `gh config set git_protocol https`, then `gh auth setup-git`
5. `startDinD()` — optional, when `ENABLE_DIND=true`
6. Mode dispatch:
   - **Pipeline:** `resolvePipelinePlan()` → `executePipelinePlan()`
   - **Prompt:** `runSinglePrompt()`
   - **Interactive:** `runInteractive()`

### Pipeline Plan Parser (`pipeline-plan.ts`)

Validates YAML v2 graph DSL at startup (fail-fast):
- `version: v2`, `entry`, `nodes`, `defaults`, `limits`
- Executable nodes: `run.kind` (agent|command) + `transitions[]`
- Terminal nodes: `terminal: true`, `terminal_status`, `exit_code`, `message`
- Built-in terminal nodes: `success` and `fail` are always present; explicit override is allowed only as terminal nodes
- `entry` cannot be `success` or `fail`
- Template var substitution for inline `prompt` and `cmd` fields (`{{UPPER_SNAKE}}`)
- Schema loading: reads + parses `decision.schema_file` JSON for each agent node

### Pipeline Executor (`pipeline-executor.ts`)

Executes the plan as a state machine:
- Starts from `entry`, evaluates transitions top-to-bottom, follows first match
- Adds implicit fallback transition `run.status == "error" -> fail` only when no semantically equivalent check already exists in node transitions
- Enforces `max_iterations` and `max_same_node_hits` limits
- Emits `pipeline_event` JSON on stdout per node lifecycle
- **Agent nodes:** spawns `claude --dangerously-skip-permissions --model <m> --verbose --output-format stream-json --json-schema <schema-json> -p <prompt>`; reads `result.structured_output` for the decision payload; validates against JSON schema
- **Command nodes:** runs shell command in `cwd`; routes on `exit_code`
- Emits final `pipeline_result` JSON

### Condition Evaluator (`condition-eval.ts`)

Compiles and evaluates transition `when` expressions against `PipelineConditionScope`:
- `run.exit_code`, `run.status`, `run.timed_out`
- `decision.<field>`
- `node.id`, `node.kind`, `node.attempt`
- `pipeline.iteration`, `pipeline.total_node_runs`
