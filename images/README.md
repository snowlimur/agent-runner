# images

The `images` directory contains runner Docker images and a Node.js entrypoint that:

1. Prepares a writable repository copy.
2. Checks GitHub authentication and configures git.
3. Runs Claude in interactive, prompt, or pipeline mode.
4. Optionally starts DinD (`dockerd` inside the container) or uses host Docker socket (DooD).

## Structure

| Path | Purpose |
| --- | --- |
| `Dockerfile` | Base image `claude:latest` (Node + system tools + Docker CLI/daemon + gh + task + entrypoint runtime). |
| `golang/Dockerfile` | Extension of the base image: Go toolchain and Go tooling. |
| `rust/Dockerfile` | Extension of the base image: Rust toolchain and cargo tooling. |
| `Taskfile.yml` | Local build tasks (`build:base`, `build:go`, `build:rust`, `build:all`). |
| `entrypoint/src/entrypoint.ts` | TypeScript entrypoint source executable. |
| `entrypoint/src/lib/*.ts` | Modular logic for CLI, pipeline, DinD, and workspace preparation. |
| `entrypoint/dist/*.js` | Compiled runtime artifacts executed in the container. |
| `golang/claude-config/settings.json` | LSP config for Go (`gopls`). |
| `rust/claude-config/settings.json` | LSP config for Rust (`rust-analyzer`). |
| `.dockerignore` | Excludes `node_modules/` from Docker build context. |

## Building Images

From the `images` directory:

```bash
task build:base
task build:go
task build:rust
task build:all
```

Default tags:

- `claude:latest` (base)
- `claude:go`
- `claude:rust`

## Base Image Contents

`Dockerfile` builds a general-purpose runner:

- Base: `node:20-bookworm-slim`.
- System packages: `git`, `curl`, `wget`, `jq`, `ripgrep`, `python3`, `build-essential`, `sudo`, `ca-certificates`.
- Docker stack: `docker-ce`, `docker-ce-cli`, `containerd.io`, `docker-buildx-plugin`, `docker-compose-plugin`.
- CLI tools: `task`, `gh`.
- User `claude` (uid/gid via `USER_ID` / `GROUP_ID`, defaults to `1000`) added to the `docker` group.
- Claude Code installation in `/home/claude/.local/bin`.
- Entrypoint runtime dependencies installed with `npm ci --omit=dev`.

Environment variables set in the base image:

- `PATH=/home/claude/.local/bin:${PATH}`
- `DOCKER_HOST=unix:///var/run/docker.sock`
- `DOCKER_TLS_CERTDIR=""`
- `ENABLE_DIND=0`
- `DIND_STORAGE_DRIVER=overlay2`
- `DIND_STARTUP_TIMEOUT_SEC=45`
- `CI=true`
- `TERM=xterm-256color`

## Language Images

### Go (`golang/Dockerfile`)

- Inherits from base (`ARG BASE_IMAGE=claude:latest`).
- Installs Go from official tarballs:
  - if `GO_DL_VERSION` is empty, fetches current stable from `https://go.dev/VERSION?m=text`;
  - otherwise pins to `GO_DL_VERSION` (for example `1.25.2`).
- Sets `GOROOT`, `GOPATH`, `PATH`.
- Installs tooling:
  - `golangci-lint`
  - `ginkgo`
  - `gopls`
  - `gofumpt`
- Copies `golang/claude-config/*` to `/home/claude/.claude/`.

### Rust (`rust/Dockerfile`)

- Inherits from base (`ARG BASE_IMAGE=claude:latest`).
- Installs system dependencies often needed for crates (`pkg-config`, `libssl-dev`).
- Installs stable Rust via `rustup`.
- Adds components:
  - `rustfmt`
  - `clippy`
  - `rust-analyzer`
- Installs `cargo-nextest`, `cargo-edit`.
- Copies `rust/claude-config/*` to `/home/claude/.claude/`.

## Entrypoint Architecture

Entrypoint source code is in `entrypoint/src/lib`:

- `main.ts` - launch orchestrator.
- `cli.ts` - CLI argument parsing and `-v/-vv` handling.
- `workspace-git.ts` - workspace preparation and git/gh bootstrap.
- `pipeline-plan.ts` - YAML plan loading and validation.
- `pipeline-executor.ts` - pipeline execution (sequential/parallel).
- `dind.ts` - DinD start/stop and signal handling.
- `constants.ts`, `utils.ts` - constants and shared helpers.

The runtime entrypoint in the image is compiled JavaScript:

- source: `entrypoint/src/entrypoint.ts`
- runtime: `/opt/entrypoint/dist/entrypoint.js`

### Startup Flow (`runEntrypoint`)

1. Parses args (`--debug`, `--model`, `--pipeline`, task args).
2. Copies source from `SOURCE_WORKSPACE_DIR` (default `/workspace-source`) into writable `/workspace`.
3. Runs `gh auth status` and `gh auth setup-git`.
4. Configures global git settings:
   - `user.name` from `GIT_USER_NAME` / `GIT_AUTHOR_NAME` / `GIT_COMMITTER_NAME` (fallback: `Claude Code Agent`);
   - `user.email` from `GIT_USER_EMAIL` / `GIT_AUTHOR_EMAIL` / `GIT_COMMITTER_EMAIL` (fallback: `claude-bot@local.docker`);
   - `safe.directory=/workspace`.
5. Starts `dockerd` if `ENABLE_DIND` is truthy; otherwise it can use a mounted host Docker socket.
6. Runs one mode:
   - pipeline (`--pipeline <path>`);
   - single prompt (when task args are present);
   - interactive (when no task args are present).

## CLI Modes

### 1) Interactive

Without task args:

```bash
entrypoint --model opus
```

Internally runs:

```bash
claude --dangerously-skip-permissions
```

### 2) Single Prompt

Example:

```bash
entrypoint --model sonnet -v "run unit tests"
```

`-v` / `-vv` are parsed from task args:

- `text` (default): `--output-format text`
- `-v`: `--output-format json`
- `-vv`: `--verbose --output-format stream-json`

### 3) Pipeline

Example:

```bash
entrypoint --pipeline .agent/pipeline.yml --model opus
```

Important: task args cannot be combined with `--pipeline`.

## YAML Pipeline Format (v2)

Plan validation is implemented in `pipeline-plan.ts`.

Supported values:

- `version`: only `v2`
- `defaults.model`: `sonnet` | `opus`
- `defaults.agent_idle_timeout_sec`: positive integer
- `defaults.command_timeout_sec`: positive integer
- `limits.max_iterations`: positive integer
- `limits.max_same_node_hits`: positive integer
- `nodes.<id>.run.kind`: `agent` | `command`
- terminal status: `success` | `blocked` | `failed` | `canceled`

Constraints:

- `entry` must reference an existing node id.
- `nodes` must be a non-empty mapping.
- Executable node:
  - requires `run` and non-empty `transitions`.
- Terminal node (`terminal: true`):
  - requires `terminal_status` and `exit_code`.
  - forbids `run` and `transitions`.
- Agent run:
  - requires exactly one of `prompt` or `prompt_file`.
  - requires `decision.schema_file`.
- Command run:
  - requires `cmd`.
- All paths (`prompt_file`, `decision.schema_file`, command `cwd`) must stay within `/workspace`.

### Pipeline Example

```yaml
version: v2
entry: planner
defaults:
  model: opus
  agent_idle_timeout_sec: 1800
  command_timeout_sec: 600
limits:
  max_iterations: 100
  max_same_node_hits: 25

nodes:
  planner:
    run:
      kind: agent
      prompt_file: prompts/planner.md
      decision:
        schema_file: schemas/planner.schema.json
    transitions:
      - when: 'decision.decision == "todo_ready"'
        to: implementer
      - when: 'decision.decision == "blocked"'
        to: blocked_stop

  implementer:
    run:
      kind: command
      cmd: "task implement TOP=1"
      cwd: "."
    transitions:
      - when: "run.exit_code == 0"
        to: reviewer
      - when: "run.exit_code != 0"
        to: blocked_stop

  reviewer:
    run:
      kind: agent
      prompt: "Review changes and return decision JSON."
      decision:
        schema_file: schemas/reviewer.schema.json
    transitions:
      - when: 'decision.decision == "approved"'
        to: done
      - when: 'decision.decision == "needs_fixes"'
        to: implementer

  blocked_stop:
    terminal: true
    terminal_status: blocked
    exit_code: 20
    message: "Pipeline blocked by validation errors."

  done:
    terminal: true
    terminal_status: success
    exit_code: 0
```

## Pipeline Execution and Events

`pipeline-executor.ts` behavior:

- Executes a state machine from `entry` until a terminal node is reached.
- Evaluates transitions top-to-bottom and takes the first matching `when`.
- Stops with system errors if no transition matches or limits are exceeded.
- For `kind: agent`, Claude is always started with:
  - `--verbose --output-format stream-json`
- Entrypoint parses stream-json for decisions and forwards each stdout line unchanged.

During execution, JSON events are printed:

- `pipeline_event.plan_start`
- `pipeline_event.node_start`
- `pipeline_event.node_session_bind` (when `system/init` with `session_id` is observed)
- `pipeline_event.node_timeout`
- `pipeline_event.node_finish`
- `pipeline_event.transition_taken`
- `pipeline_event.plan_finish`

At the end, an aggregated `pipeline_result` is printed. Exit code is:

- terminal node `exit_code`, if a terminal node was reached;
- `2` invalid plan;
- `3` no transition matched;
- `4` iteration limits exceeded;
- `5` node execution error without matching transition.

## DinD (Optional)

Enable with `ENABLE_DIND=1`.

Requirements:

- container must run with `--privileged`;
- mounting an external `/var/run/docker.sock` at the same time is not allowed (entrypoint exits with an error).

Behavior:

- `dockerd` starts via `sudo -n`.
- First attempt uses `DIND_STORAGE_DRIVER` (default `overlay2`).
- If `overlay2` reports fatal mount/driver errors, it retries with `vfs` immediately.
- If `overlay2` does not become ready quickly, it retries with `vfs` after a short 5s grace period.
- Readiness timeout comes from `DIND_STARTUP_TIMEOUT_SEC` (default `45` seconds).
- On `SIGINT`/`SIGTERM`, daemon shutdown is handled gracefully.

Run example:

```bash
docker run --rm --privileged \
  -e ENABLE_DIND=1 \
  -e DIND_STORAGE_DRIVER=overlay2 \
  -e SOURCE_WORKSPACE_DIR=/workspace-source \
  -e GH_TOKEN=*** \
  -e CLAUDE_CODE_OAUTH_TOKEN=*** \
  -v "$(pwd)":/workspace-source:ro \
  claude:go --model opus -v "run integration tests"
```

## DooD (Host Docker Socket)

Use this mode when you want nested Docker commands without starting internal `dockerd`.

Behavior:

- does not set `ENABLE_DIND`
- does not require `--privileged`
- reuses host daemon via `/var/run/docker.sock`
- avoids DinD startup latency and fallback logic

Run example:

```bash
docker run --rm \
  -e SOURCE_WORKSPACE_DIR=/workspace-source \
  -e GH_TOKEN=*** \
  -e CLAUDE_CODE_OAUTH_TOKEN=*** \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd)":/workspace-source:ro \
  claude:go --model opus -v "run integration tests"
```

## Maintenance Notes

- `entrypoint/package.json` uses `commander` and `js-yaml`.
- If `js-yaml` is unavailable, pipeline parsing falls back to `ruby -ryaml -rjson`.
- `entrypoint/node_modules` may exist in the repository, but image build uses `npm ci --omit=dev`, and `.dockerignore` excludes `node_modules/` from build context.
