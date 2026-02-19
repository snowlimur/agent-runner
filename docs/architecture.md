# Architecture Codemap

Updated: 2026-02-19

## Overview

agent-runner is a two-component system: a **Go CLI** (`agent-cli`) on the host orchestrates **Docker containers** that run Claude Code via a **TypeScript entrypoint**. Supports single-prompt runs and graph-driven pipeline plans (v2 state machine).

## Repository Layout

```
agent-runner/
├── agent-cli/            # Go CLI binary (host-side)
│   ├── main.go
│   └── internal/
│       ├── cli/          # run + stats commands, progress TUI
│       ├── config/       # TOML parser, config validation
│       ├── result/       # stream-json protocol parser, AgentResult
│       ├── runner/       # Docker Engine API lifecycle
│       └── stats/        # RunRecord persistence + aggregation
├── images/
│   ├── Dockerfile        # Base image (claude:latest)
│   ├── golang/           # claude:go layer
│   ├── rust/             # claude:rust layer
│   └── entrypoint/       # TypeScript container entrypoint
│       └── src/
│           ├── entrypoint.ts
│           └── lib/
│               ├── main.ts           # runEntrypoint() orchestrator
│               ├── cli.ts            # arg parsing
│               ├── pipeline-plan.ts  # YAML v2 parser + validator
│               ├── pipeline-executor.ts  # state machine executor
│               ├── condition-eval.ts     # transition expression evaluator
│               ├── workspace-git.ts      # workspace copy + gh auth
│               ├── dind.ts               # Docker-in-Docker lifecycle
│               ├── types.ts              # shared TypeScript types
│               ├── constants.ts
│               └── utils.ts
├── pipelines/            # Example v2 pipeline plans + schemas + scripts
├── tests/                # Docker Compose integration tests
└── Taskfile.yml          # Root task orchestrator
```

## Execution Flow

```
Host                                     Container
─────                                    ─────────
agent-cli run [--pipeline plan.yml]
  ├─ Load .agent-cli/config.toml
  ├─ Start Docker container ──────────► entrypoint.ts → main.ts:runEntrypoint()
  ├─ Stream stdout/stderr                 ├─ prepareWorkspaceFromReadOnlySource()
  │   ├─ ParseStreamLine() per line       ├─ configureGit()
  │   ├─ Feed ProgressTUI                 ├─ ensureGitHubAuthAndSetupGit()
  │   └─ Accumulate usage metrics         ├─ (optional) startDinD()
  ├─ Extract pipeline_result              └─ mode dispatch:
  │   or AgentResult                          ├─ pipeline: parse v2 YAML → execute state machine
  ├─ Persist RunRecord + artifacts            ├─ prompt: run single Claude process
  └─ Print summary / return JSON              └─ interactive: run Claude without -p
```

## Key Interfaces

- **`PipelinePlan (v2)`** — graph DSL: `entry`, `nodes` (executable + terminal), `defaults`, `limits`
- **`pipeline_event`** — node-level lifecycle events emitted by the container (`node_start`, `node_session_bind`, `node_finish`, `transition_taken`, ...)
- **`pipeline_result`** — final JSON summary: `terminal_node`, `terminal_status`, `node_runs[]`
- **`StreamEvent`** — Go union envelope: `system | assistant | user | pipeline_event | result`
- **`stats.RunRecord`** — persisted per-run record with normalized token/cost metrics

## Build System

Taskfile v3 (hierarchical includes):
- `task install` — build CLI binary + all Docker images
- `task cli:build` / `task cli:test` — Go build/test
- `task image:entrypoint:build` / `task image:entrypoint:typecheck` — TS build/typecheck
- `task image:build:base|go|rust|all` — Docker image builds
