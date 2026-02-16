# Contributing Guide

Updated: 2026-02-17

## Prerequisites

- Go 1.25.6+
- Node.js 20+
- Docker Engine
- [Task](https://taskfile.dev/) (Taskfile runner)

## Setup

```bash
git clone <repo-url>
cd agent-runner

# Build CLI + Docker images
task install
```

## Commands

| Command | Description |
|---------|-------------|
| `task install` | Build CLI and all Docker images |
| `task cli:build` | Build agent-cli binary into `agent-cli/bin/` |
| `task cli:test` | Run Go unit tests |
| `task cli:install` | Install agent-cli into GOPATH/bin |
| `task image:build:base` | Build base Docker image |
| `task image:build:go` | Build Go development image |
| `task image:build:rust` | Build Rust development image |
| `task image:build:all` | Build all images |
| `task image:entrypoint:build` | Build TypeScript entrypoint |
| `task image:entrypoint:typecheck` | Typecheck entrypoint sources |
| `task test:cli:go` | Build CLI + run Go test workspace |
| `task test:cli:go:plan` | Build CLI + run Go pipeline test |
| `task test:go` | Run Go test workspace via Docker Compose |
| `task test:rust` | Run Rust test workspace via Docker Compose |

## Project Configuration

Each project using agent-cli requires `.agent-cli/config.toml`:

```toml
[docker]
image = "claude:go"
model = "opus"             # sonnet | opus
enable_dind = false        # optional Docker-in-Docker

[auth]
github_token = "ghp_..."
claude_token = "sk-..."

[workspace]
source_workspace_dir = "/absolute/path/to/source"

[git]
user_name = "Your Name"
user_email = "you@example.com"
```

## Running

```bash
# Single prompt
agent-cli run "build and test the project"

# Prompt from file
agent-cli run --file prompt.md

# Pipeline plan
agent-cli run --pipeline plan.yml

# JSON output
agent-cli run --json "describe this project"

# Model override
agent-cli run --model sonnet "quick task"

# View run statistics
agent-cli stats
agent-cli stats --json
```

## Testing

```bash
# Go unit tests
cd agent-cli && go test ./...

# Integration tests (requires Docker + .env)
cd tests && docker compose run --rm claude-go
```

## Code Structure

- **Go CLI:** Standard `internal/` layout, no third-party frameworks except Docker SDK
- **TypeScript entrypoint:** Strict mode, ESM modules, no test framework (container-level testing)
- **Config:** Hand-written TOML parser (no external dependency)
- **Docker images:** Multi-layer (base → language-specific), Taskfile-driven builds
