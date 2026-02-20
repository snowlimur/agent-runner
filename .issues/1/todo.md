# Issue #1 — Add app version command

GitHub: https://github.com/snowlimur/agent-runner/issues/1

## Summary

Add a `version` command to `agent-cli` that displays the current application version.
The Go CLI (`agent-cli/`) uses a switch-based command dispatcher in `main.go` with
command handlers defined as functions in `internal/cli/`. There is currently no version
constant or version command.

## Implementation Checklist

[ ] Define a `Version` variable in `agent-cli/internal/cli/version.go` that can be
    overridden at build time via `-ldflags` (default to `"dev"`).

[ ] Implement `VersionCommand()` in `agent-cli/internal/cli/version.go` that prints
    the version string to stdout (format: `agent-cli version <version>`).
    Support `--json` flag following the `StatsCommand` pattern.

[ ] Register the `version` command in the switch statement in `agent-cli/main.go`
    (handle `"version"`, `"--version"`, and `"-v"` cases).

[ ] Update `printUsage()` in `agent-cli/main.go` to include `agent-cli version [--json]`
    in the usage text.

[ ] Update `agent-cli/Taskfile.yml` build task to inject the version via
    `-ldflags "-X agent-cli/internal/cli.Version=<version>"` using a git tag or
    a hardcoded value.

[ ] Add unit tests in `agent-cli/internal/cli/version_test.go` to verify the output
    format of `VersionCommand()`.

[ ] Run `go build` and `go test ./...` inside `agent-cli/` to verify everything
    compiles and passes.
