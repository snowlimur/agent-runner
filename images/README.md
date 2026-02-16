# images

The `images` directory contains runner Docker images and a Node.js entrypoint that:

1. Prepares a writable repository copy.
2. Checks GitHub authentication and configures git.
3. Runs Claude in interactive, prompt, or pipeline mode.
4. Optionally starts DinD (`dockerd` inside the container).

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
5. Starts `dockerd` if `ENABLE_DIND` is truthy.
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

## YAML Pipeline Format (v1)

Plan validation is implemented in `pipeline-plan.ts`.

Supported values:

- `version`: only `v1`
- `model`: `sonnet` | `opus`
- `verbosity`: `text` | `v` | `vv` (aliases: `json` -> `v`, `stream-json`/`stream_json` -> `vv`)
- `on_error`: `fail_fast` | `continue`
- `workspace`: `shared` | `worktree` | `snapshot_ro`
- stage `mode`: `sequential` | `parallel`

Constraints:

- `stages` must be a non-empty array.
- `stage.id` and `task.id` must be unique in their scope.
- Each task must define exactly one prompt source: `prompt` or `prompt_file`.
- `prompt_file` must be a relative path inside `/workspace` (path traversal is blocked).
- For `mode: parallel` + `workspace: shared`, one of these is required:
  - `read_only: true`
  - `allow_shared_writes: true`

Note: `read_only` is a validation guard for shared parallel mode. Actual read-only filesystem permissions are applied only for `workspace: snapshot_ro`.

### Pipeline Example

```yaml
version: v1
defaults:
  model: opus
  verbosity: vv
  on_error: fail_fast
  workspace: shared

stages:
  - id: analyze
    mode: sequential
    tasks:
      - id: inspect
        prompt: "Analyze recent changes and list risks."

  - id: implement
    mode: parallel
    max_parallel: 2
    tasks:
      - id: code
        prompt_file: prompts/implement.md
        workspace: worktree
      - id: tests
        prompt: "Run tests and summarize failing cases."
        workspace: shared
        read_only: true
```

## Pipeline Execution and Events

`pipeline-executor.ts` behavior:

- For `workspace: shared`, tasks run directly in `/workspace`.
- For `workspace: worktree` and `workspace: snapshot_ro`, a workspace copy is created at:
  - `/tmp/agent-pipeline-workspaces/<planRunId>/<stageId>/<taskId>`
- For `snapshot_ro`, permissions are applied recursively:
  - directories: `0555`
  - files: `0444`

During execution, JSON events are printed:

- `pipeline_event.plan_start`
- `pipeline_event.stage_start`
- `pipeline_event.task_start`
- `pipeline_event.task_session_bind` (when `system/init` with `session_id` is observed in stream-json output)
- `pipeline_event.task_finish`
- `pipeline_event.stage_finish`
- `pipeline_event.plan_finish`

At the end, an aggregated `pipeline_result` is printed and process exit code is:

- `0` if all tasks succeed;
- `1` if at least one task fails.

## DinD (Optional)

Enable with `ENABLE_DIND=1`.

Requirements:

- container must run with `--privileged`;
- mounting an external `/var/run/docker.sock` at the same time is not allowed (entrypoint exits with an error).

Behavior:

- `dockerd` starts via `sudo -n`.
- First attempt uses `DIND_STORAGE_DRIVER` (default `overlay2`).
- If `overlay2` fails readiness checks, it retries with `vfs`.
- Readiness timeout comes from `DIND_STARTUP_TIMEOUT_SEC` (default `45` seconds).
- On `SIGINT`/`SIGTERM`, daemon shutdown is handled gracefully.

Run example:

```bash
docker run --rm --privileged \
  -e ENABLE_DIND=1 \
  -e SOURCE_WORKSPACE_DIR=/workspace-source \
  -e GH_TOKEN=*** \
  -e CLAUDE_CODE_OAUTH_TOKEN=*** \
  -v "$(pwd)":/workspace-source:ro \
  claude:go --model opus -v "run integration tests"
```

## Maintenance Notes

- `entrypoint/package.json` uses `commander` and `js-yaml`.
- If `js-yaml` is unavailable, pipeline parsing falls back to `ruby -ryaml -rjson`.
- `entrypoint/node_modules` may exist in the repository, but image build uses `npm ci --omit=dev`, and `.dockerignore` excludes `node_modules/` from build context.
