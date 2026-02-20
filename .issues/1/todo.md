# Issue #1 — Add app version command

> **Goal:** Add a `version` command to `agent-cli` that prints the current application version, injected at build time via `-ldflags`.

---

## S1 — Version infrastructure and command

[X] **T1.1**: Define version variable and create `VersionCommand` handler
- Add a **package-level variable** `var version = "dev"` in `@agent-cli/main.go`. The variable must be overridable via `go build -ldflags "-X main.version=<value>"`.
- Create `@agent-cli/internal/cli/version.go` exporting `func VersionCommand(appVersion string, args []string) error`.
  - The function must accept a `*flag.FlagSet` internally with no registered flags. If any positional arguments are provided (`fs.NArg() > 0`), return an error `"version command does not accept positional arguments"`.
  - On success, print **exactly** `agent-cli version <version>\n` to `os.Stdout` and return `nil`.
- **Packages:** `flag`, `fmt`, `errors`, `os`.
- **Test** `@agent-cli/internal/cli/version_test.go`:
  - `TestVersionCommand_PrintsVersion` — call `VersionCommand("v1.2.3", nil)`, capture stdout, assert output equals `"agent-cli version v1.2.3\n"`.
  - `TestVersionCommand_RejectsPositionalArgs` — call `VersionCommand("v1.0.0", []string{"extra"})`, assert non-nil error containing `"does not accept positional arguments"`.
- **Finish line:** `go test ./internal/cli/ -run TestVersionCommand` passes (2/2).

[X] **T1.2**: Wire `version` command into CLI dispatcher and update help text
- In `@agent-cli/main.go`, add `case "version"` to the command switch that calls `cli.VersionCommand(version, args)`.
- Update `printUsage()` to include `version` in the list of available commands with description `"Print application version"`.
- **Test** `@agent-cli/main_test.go` (or verify manually via build):
  - `TestMainDispatch_Version` — build the binary, run `./bin/agent-cli version`, assert exit code `0` and stdout contains `"agent-cli version "`.
  - `TestMainDispatch_VersionRejectsArgs` — run `./bin/agent-cli version foo`, assert exit code `1` and stderr contains `"does not accept positional arguments"`.
- **Finish line:** `go build -o bin/agent-cli . && ./bin/agent-cli version` prints `agent-cli version dev` with exit code 0.

[X] **T1.3**: Update `Taskfile.yml` to inject version from git tag at build time
- In `@agent-cli/Taskfile.yml`, update the `build` task `cmds` to: `go build -ldflags "-X main.version={{.VERSION}}" -o {{.BUILD_DIR}}/{{.BINARY_NAME}} .`
- Add a dynamic variable `VERSION` that resolves via `git describe --tags --always --dirty 2>/dev/null || echo dev`.
- **Test:** Run `task build && ./bin/agent-cli version`. Output must match pattern `agent-cli version <non-empty-string>\n` (not empty, not blank).
- **Finish line:** `task build` succeeds, and `./bin/agent-cli version` prints a version string derived from git metadata (or `dev` fallback).

[X] **T1.4**: Validate full build and all tests pass
- Run `task test` (or `go test ./...`) from `@agent-cli/` directory — **all** tests must pass, including the new version tests.
- Run `task build` — binary compiles without errors.
- Run `./bin/agent-cli version` — outputs version string.
- Run `./bin/agent-cli help` — output includes `version` command.
- **Finish line:** Zero test failures, zero build errors, version command visible in help output.
