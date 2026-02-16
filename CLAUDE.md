# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build everything (CLI + Docker images)
task install

# Go CLI
task cli:build                # Build binary to agent-cli/bin/agent-cli
task cli:test                 # Run all Go unit tests
task cli:install              # Install to GOPATH/bin
cd agent-cli && go test ./internal/cli/...   # Run tests for a single package

# TypeScript entrypoint
task image:entrypoint:build       # npm install + tsc
task image:entrypoint:typecheck   # tsc --noEmit

# Docker images
task image:build:base         # claude:latest
task image:build:go           # claude:go (depends on base)
task image:build:rust         # claude:rust (depends on base)
task image:build:all          # All images

# Integration tests (require Docker + tests/.env with tokens)
task test:cli:go              # Build CLI then run Go test workspace
task test:cli:go:plan         # Build CLI then run pipeline test
```

## Architecture

Two-component system: a **Go CLI** on the host orchestrates **Docker containers** that run Claude Code via a **TypeScript entrypoint**.

### Host: agent-cli (Go)

`agent-cli run <prompt>` loads `.agent-cli/config.toml`, creates a Docker container, streams its stdout/stderr, parses JSON events in real-time, and persists run records to `.agent-cli/runs/`.

Key packages in `agent-cli/internal/`:
- **runner** — Docker Engine API lifecycle (create → start → stream logs → wait → cleanup). Uses `dockerAPI` interface for testability. Labels containers with CWD hash for stale cleanup.
- **result** — Parses Claude Code's streaming JSON protocol. `StreamEvent` is the union envelope dispatched by `Type` field: `system`, `assistant`, `user`, `pipeline_event`, `result`.
- **cli** — `RunCommand` wires runner + result parsing + progress printing + stats persistence. `ProgressPrinter` formats real-time tool lifecycle lines, prefixed with `[stage_id/task_id]` in pipeline mode.
- **config** — Hand-written TOML parser (no external deps). Sections: `[docker]`, `[auth]`, `[workspace]`, `[git]`.
- **stats** — Persists `RunRecord` as JSON per run, aggregates across runs for `agent-cli stats`.

### Container: entrypoint (TypeScript)

The container's `ENTRYPOINT` is `node /opt/entrypoint/dist/entrypoint.js`. Startup sequence: copy read-only source mount → `/workspace`, authenticate GitHub, configure git, optionally start DinD, then run Claude in one of three modes: pipeline, single prompt, or interactive.

Key modules in `images/entrypoint/src/lib/`:
- **pipeline-plan** — Parses and validates YAML pipeline plans (v1). Cascading defaults: plan → stage → task for model, verbosity, on_error, workspace.
- **pipeline-executor** — Runs stages sequentially; within a stage, tasks run sequentially or in parallel (worker pool with `maxParallel`). Emits `pipeline_event` JSON on stdout that the Go CLI parses. Prepares per-task workspace copies for `worktree`/`snapshot_ro` modes.
- **dind** — Optional Docker-in-Docker daemon lifecycle with overlay2 → vfs fallback.

### Data Flow

The CLI streams container stdout line-by-line. Each line is either a JSON event or plain text. The `result.ParseStreamLine()` function classifies and parses each line. Events flow through: stream hooks → progress printer (human output) + metrics collector → final result extraction → run record persistence.

Pipeline mode adds a layer: the entrypoint emits `pipeline_event` JSON (plan/stage/task lifecycle) alongside Claude's own stream-json events. The CLI's `extractPipelineResultFromStream()` finds the final `pipeline_result` JSON object.

## Conventions

- Go CLI uses zero external frameworks beyond the Docker SDK. Config parsing, CLI flags, and TOML are all hand-written.
- TypeScript uses strict mode with `exactOptionalPropertyTypes`, `noUncheckedIndexedAccess`, and ESM modules (`NodeNext`).
- Docker images are layered: base (`claude:latest`) → language-specific (`claude:go`, `claude:rust`).
- The TOML config lives at `<project>/.agent-cli/config.toml`; run artifacts at `<project>/.agent-cli/runs/<timestamp>-<id>/`.
- Pipeline workspace isolation: `shared` reuses `/workspace`; `worktree` copies it; `snapshot_ro` copies and chmod's to read-only. Parallel tasks on `shared` workspace must declare `read_only: true` or `allow_shared_writes: true`.
- Go tests use the standard `testing` package. The runner package uses a `dockerAPI` interface and `newDockerAPIFn` variable for test doubles.
