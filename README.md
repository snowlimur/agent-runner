# agent-runner

Run Claude Code agents inside Docker containers from the command line. Supports single-prompt runs and graph-driven pipeline plans.

## How it works

A Go CLI (`agent-cli`) on the host creates and streams a Docker container. Inside the container, a TypeScript entrypoint copies the workspace, authenticates GitHub, and runs Claude Code.

See [docs/architecture.md](docs/architecture.md) for the full execution flow.

## Quick start

```bash
# Build CLI + Docker images
task install

# Configure a project
cat > .agent-cli/config.toml <<EOF
[docker]
image = "claude:go"

[auth]
github_token = "ghp_..."
claude_token  = "sk-..."

[workspace]
source_workspace_dir = "/absolute/path/to/project"

[git]
user_name  = "Your Name"
user_email = "you@example.com"
EOF

# Run a prompt
agent-cli run "add unit tests for the parser"

# Run a pipeline
agent-cli run --pipeline pipelines/issue-flow.yml --var TASK=42
```

Full config reference → [docs/data.md](docs/data.md#config-schema-agent-cliconfig-toml)

## Pipelines (v2)

Graph-based state machine with agent nodes and command nodes. Each agent node declares a JSON Schema for its output; Claude is invoked with `--json-schema` to enforce structured output at inference time.

See [docs/backend.md](docs/backend.md#pipeline-executor-pipeline-executorts) and [images/README.md](images/README.md) for the pipeline DSL reference.

## Documentation

| Doc | Contents |
|-----|----------|
| [docs/architecture.md](docs/architecture.md) | System overview, repo layout, execution flow |
| [docs/backend.md](docs/backend.md) | Go packages, TypeScript entrypoint, pipeline executor |
| [docs/data.md](docs/data.md) | Types, config schema, storage layout, event protocol |
| [docs/CONTRIB.md](docs/CONTRIB.md) | Setup, commands, testing, conventions |
| [docs/RUNBOOK.md](docs/RUNBOOK.md) | Build, artifacts, timeouts, troubleshooting |
