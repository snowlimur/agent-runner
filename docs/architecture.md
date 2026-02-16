# Architecture Codemap

Updated: 2026-02-17

## Overview

agent-runner is a CLI + container system for running Claude Code agents inside Docker containers. It supports single-prompt runs and multi-step YAML pipeline plans with parallel/sequential stage execution.

## Repository Layout

```
agent-runner/
├── agent-cli/              # Go CLI (host-side orchestrator)
│   ├── main.go             # Entry point: run | stats | help
│   └── internal/
│       ├── cli/            # Command implementations + progress printer
│       ├── config/         # TOML config loader (.agent-cli/config.toml)
│       ├── result/         # Stream JSON parser + result types
│       ├── runner/         # Docker container lifecycle (create/start/logs/wait/cleanup)
│       └── stats/          # Run record persistence + aggregation
├── images/                 # Docker image definitions
│   ├── Dockerfile          # Base image (Node 20, Claude Code, Docker Engine, gh, task)
│   ├── entrypoint/         # TypeScript entrypoint (runs inside container)
│   │   └── src/
│   │       ├── entrypoint.ts       # Process entry
│   │       └── lib/
│   │           ├── main.ts         # Workspace prep -> git auth -> DinD -> run
│   │           ├── cli.ts          # Argument parsing (commander)
│   │           ├── pipeline-plan.ts    # YAML plan loader + validator
│   │           ├── pipeline-executor.ts # Stage/task execution engine
│   │           ├── workspace-git.ts    # Source copy + git config
│   │           ├── dind.ts         # Docker-in-Docker lifecycle
│   │           ├── types.ts        # Shared type definitions
│   │           ├── constants.ts    # Paths, defaults
│   │           └── utils.ts        # Shell helpers, env readers
│   ├── golang/             # Go language layer (Dockerfile + Claude config)
│   └── rust/               # Rust language layer (Dockerfile + Claude config)
├── tests/                  # Integration test workspaces
│   ├── golang/             # Go test project with .agent-cli config
│   └── rust/               # Rust test project
└── Taskfile.yml            # Root task runner (includes cli, image, test)
```

## Execution Flow

```
Host                                Container
─────                               ─────────
agent-cli run <prompt>
  │
  ├─ Load .agent-cli/config.toml
  ├─ Build Docker run spec
  ├─ Cleanup stale containers
  ├─ Pull image (best-effort)
  ├─ Create + Start container ──────► entrypoint.ts
  ├─ Stream stdout/stderr             ├─ Copy source -> /workspace
  │   ├─ Parse JSON events            ├─ Setup git + GitHub auth
  │   ├─ Update progress printer      ├─ Start DinD (optional)
  │   └─ Collect metrics              ├─ Resolve pipeline or prompt
  ├─ Wait for container exit          ├─ Run claude process(es)
  ├─ Save run record + artifacts      └─ Exit
  └─ Print summary
```

## Key Interfaces

- `runner.dockerAPI` — Docker Engine client abstraction (create/start/logs/wait/stop/remove)
- `result.StreamEvent` — Parsed stream event envelope (system | assistant | user | pipeline | result)
- `stats.RunRecord` — Persisted run record with metrics, errors, and agent result
- `PipelinePlan` — Validated pipeline plan (version, defaults, stages with tasks)

## External Dependencies

### Go CLI (agent-cli)
- `github.com/docker/docker` — Docker Engine API client
- `github.com/opencontainers/image-spec` — OCI image spec types

### TypeScript Entrypoint (images/entrypoint)
- `commander` — CLI argument parsing
- `js-yaml` — YAML plan file parsing

## Build System

Taskfile v3 (hierarchical):
- Root `Taskfile.yml` includes `cli:`, `image:`, `test:`
- `task install` builds CLI + all images
- `task cli:test` runs Go unit tests
- `task image:build:all` builds base, go, (rust) images
