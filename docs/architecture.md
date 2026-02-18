# Architecture Codemap

Updated: 2026-02-18

## Overview

agent-runner is a CLI + container system for running Claude Code agents inside Docker containers. It supports single-prompt runs and graph-driven pipeline plans (`pipeline v2`).

## Repository Layout

```
agent-runner/
├── agent-cli/
│   ├── main.go
│   └── internal/
│       ├── cli/
│       ├── config/
│       ├── result/
│       ├── runner/
│       └── stats/
├── images/
│   ├── Dockerfile
│   ├── entrypoint/
│   │   └── src/
│   │       ├── entrypoint.ts
│   │       └── lib/
│   │           ├── main.ts
│   │           ├── cli.ts
│   │           ├── pipeline-plan.ts
│   │           ├── condition-eval.ts
│   │           ├── pipeline-executor.ts
│   │           ├── workspace-git.ts
│   │           ├── dind.ts
│   │           ├── types.ts
│   │           ├── constants.ts
│   │           └── utils.ts
│   ├── golang/
│   └── rust/
├── tests/
└── Taskfile.yml
```

## Execution Flow

```
Host                                Container
─────                               ─────────
agent-cli run --pipeline plan.yml
  │
  ├─ Load config
  ├─ Start Docker runner container ──────► entrypoint.ts
  ├─ Stream stdout/stderr                ├─ Prepare /workspace
  │   ├─ Parse stream JSON               ├─ Setup gh/git
  │   ├─ Update TUI                      ├─ Parse pipeline v2 graph
  │   └─ Attribute usage                 ├─ Execute state machine (nodes/transitions)
  ├─ Extract pipeline_result             └─ Emit pipeline_event + pipeline_result
  ├─ Persist run artifacts
  └─ Print summary
```

## Key Interfaces

- `PipelinePlan (v2)` - graph DSL with executable and terminal nodes.
- `pipeline_event` - node-level lifecycle events (`node_start`, `node_finish`, `transition_taken`, ...).
- `pipeline_result` - final summary with terminal outcome and `node_runs[]`.
- `stats.RunRecord` - persisted record enriched with normalized usage metrics.

## Build System

Taskfile v3 (hierarchical):
- `task install` builds CLI + images
- `task cli:test` runs Go unit tests
- `task image:entrypoint:typecheck` validates TS entrypoint
