# Issue #1: Add app version command

> Add a `version` command to agent-cli for displaying the current version of the application.
> https://github.com/snowlimur/agent-runner/issues/1

## Implementation checklist

[ ] Create `agent-cli/internal/cli/version.go` with `VersionCommand(args []string) error` function that prints version info to stdout (follow the `StatsCommand` pattern in `stats.go`)
[ ] Define a `Version` variable (`var Version = "dev"`) in `version.go` that can be overridden via `-ldflags` at build time
[ ] Register the `version` command in `agent-cli/main.go` by adding a `case "version", "-v", "--version":` branch to the switch statement (~line 36)
[ ] Update `printUsage()` in `agent-cli/main.go` to include `agent-cli version` in the help text
[ ] Update the build task in `agent-cli/Taskfile.yml` to inject version via ldflags: `-ldflags="-X agent-cli/internal/cli.Version=$(git describe --tags --always --dirty)"`
[ ] Add unit test `agent-cli/internal/cli/version_test.go` verifying `VersionCommand` outputs the version string
[ ] Run `task cli:build` and verify `agent-cli version` prints the expected output
[ ] Run `task cli:test` and confirm all tests pass
