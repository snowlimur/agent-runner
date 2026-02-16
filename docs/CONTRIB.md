# Contributing Guide

Updated: 2026-02-19

## Prerequisites

- Go 1.26+
- Node.js 20+
- Docker Engine
- [Task](https://taskfile.dev/) (Taskfile runner)
- `gh` CLI (for integration tests)

## Setup

```bash
git clone <repo-url>
cd agent-runner

# Build CLI binary + all Docker images
task install
```

Each project that uses agent-cli needs `.agent-cli/config.toml` — see `docs/data.md` for the schema.

## Commands

| Command | Description |
|---------|-------------|
| `task install` | Build CLI binary and all Docker images |
| `task cli:build` | Build agent-cli binary into `agent-cli/bin/` |
| `task cli:test` | Run Go unit tests |
| `task cli:install` | Install agent-cli into GOPATH/bin |
| `task cli:mod` | Download and tidy Go modules |
| `task cli:lint` | Run golangci-lint + deadcode |
| `task cli:fmt` | Format Go code with gofumpt |
| `task image:build:base` | Build base Docker image (`claude:latest`) |
| `task image:build:go` | Build Go image (`claude:go`, depends on base) |
| `task image:build:rust` | Build Rust image (`claude:rust`, depends on base) |
| `task image:build:all` | Build all images |
| `task image:entrypoint:build` | Compile TypeScript entrypoint (`npm install + tsc`) |
| `task image:entrypoint:typecheck` | Typecheck entrypoint without emitting output |
| `task test:cli:go` | Build CLI then run Go integration workspace |
| `task test:cli:go:plan` | Build CLI then run pipeline integration test |

## Running agent-cli

```bash
# Single prompt
agent-cli run "build and test the project"

# Prompt from file
agent-cli run --file prompt.txt

# Pipeline plan
agent-cli run --pipeline pipelines/issue-flow.yml

# Pipeline with template variables ({{UPPER_SNAKE}} placeholders)
agent-cli run --pipeline plan.yml --var TASK=42 --var ENV=staging

# Raw JSON output
agent-cli run --json "describe this project"

# Model override
agent-cli run --model sonnet "quick task"

# View run statistics
agent-cli stats
agent-cli stats --json
```

Notes on `--var`:
- Only valid with `--pipeline`
- Repeatable; key must match `^[A-Z][A-Z0-9_]*$`
- Applies to inline `nodes.<id>.run.prompt` only (not `prompt_file`)
- Run fails fast for missing, unused, or duplicate vars

## Testing

```bash
# Go unit tests
cd agent-cli && go test ./...

# Integration tests (require Docker + tests/.env with tokens)
task test:cli:go        # Go workspace test
task test:cli:go:plan   # Pipeline test
```

## Code Conventions

- **Go CLI:** Standard `internal/` layout, no third-party frameworks beyond Docker SDK. Config and TOML parsing are hand-written.
- **TypeScript entrypoint:** Strict mode with `exactOptionalPropertyTypes`, `noUncheckedIndexedAccess`, `verbatimModuleSyntax`. ESM modules (`NodeNext`). No test framework (tested at integration level).
- **Docker images:** Layered: `claude:latest` → `claude:go` / `claude:rust`.
- **Pipeline v2 plans:** YAML files with `version: v2`. Agent nodes require a `decision.schema_file` (JSON Schema). Transitions use expression syntax evaluated against `PipelineConditionScope`.
