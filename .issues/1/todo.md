# Issue #1 — Add app version command

Reference: https://github.com/snowlimur/agent-runner/issues/1

## Context

The Go CLI (`agent-cli`) dispatches commands via a switch statement in `agent-cli/main.go`.
Existing commands (`run`, `stats`, `help`) live in `agent-cli/internal/cli/`.
The `StatsCommand` in `stats.go` is the simplest pattern to follow for a new command.
Currently there is no version tracking in the Go CLI.

## Tasks

[ ] Define a version variable in `agent-cli/internal/cli/version.go` (new file) with a `var Version = "dev"` that can be overridden via `-ldflags` at build time
[ ] Implement `VersionCommand(args []string) error` in `agent-cli/internal/cli/version.go` following the `StatsCommand` pattern — support `--json` flag, print `agent-cli version <ver>` by default
[ ] Register the `version` command in the switch statement in `agent-cli/main.go` (`case "version"` → `cli.VersionCommand(args)`)
[ ] Update `printUsage()` in `agent-cli/main.go` to include `agent-cli version [--json]`
[ ] Add build-time version injection to `agent-cli/Taskfile.yml` via `-ldflags "-X agent-cli/internal/cli.Version={{.VERSION}}"`
[ ] Verify the build compiles: `cd agent-cli && go build ./...`
