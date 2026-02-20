# Issue #1 — Add App Version Command

[Issue link](https://github.com/snowlimur/agent-runner/issues/1)

---

## S1: Version Package & Command

[X] **T1.1**: Create version package with build-time variable `@agent-cli/internal/version/version.go`
- Define package `version` with an unexported module-level variable `version` (type `string`, default `"dev"`).
- Define `func Info() string` that returns the version string.
- The `version` variable must be settable via `-ldflags "-X agent-cli/internal/version.version=v1.2.3"`.
- No global mutable state beyond the linker-injected variable.
- **Deps**: none (stdlib only).
- **Test signature** (`@agent-cli/internal/version/version_test.go`):
  ```
  func TestInfo(t *testing.T)
  // table cases:
  //   { name: "returns default dev version", want: "dev" }
  ```
  **Finish line**: `go test ./internal/version/...` passes; `version.Info()` returns `"dev"` when no ldflags are set.

[X] **T1.2**: Create `VersionCommand` function `@agent-cli/internal/cli/version.go`
- Define `func VersionCommand(args []string) error`.
- The function prints `agent-cli version <version>` to `os.Stdout` and returns `nil`.
- Accept an `io.Writer` parameter for testability: `func VersionCommand(w io.Writer, args []string) error`.
- **Deps**: `agent-cli/internal/version`, `fmt`, `io`.
- **Test signature** (`@agent-cli/internal/cli/version_test.go`):
  ```
  func TestVersionCommand(t *testing.T)
  // table cases:
  //   { name: "prints version string", args: []string{}, wantOut: "agent-cli version dev\n" }
  //   { name: "ignores extra args", args: []string{"--json"}, wantOut: "agent-cli version dev\n" }
  ```
  **Finish line**: `go test ./internal/cli/... -run TestVersionCommand` passes; output matches expected format exactly.

[X] **T1.3**: Register `version` command in `main.go` `@agent-cli/main.go`
- Add `case "version"` to the `switch` statement in `func run()`.
- Route to `cli.VersionCommand(os.Stdout, args)`.
- Update `printUsage()` to include `agent-cli version` line.
- **Deps**: `agent-cli/internal/cli`.
- **Test**: N/A (integration verified in T1.4).
- **Finish line**: `go build .` succeeds; running `./bin/agent-cli version` prints `agent-cli version dev`.

[X] **T1.4**: Validation checkpoint — compile and run full test suite
- Run `go build ./...` to verify zero compilation errors.
- Run `go test ./...` to verify all existing and new tests pass.
- Run `go vet ./...` to verify no vet warnings.
- **Finish line**: all three commands exit 0.

---

## S2: Build-Time Version Injection

[ ] **T2.1**: Update `Taskfile.yml` to inject version via ldflags `@agent-cli/Taskfile.yml`
- Modify the `build` task `go build` command to include `-ldflags "-X agent-cli/internal/version.version={{.VERSION}}"`.
- Define `VERSION` variable using `git describe --tags --always --dirty` (with a fallback to `dev`).
- **Deps**: none.
- **Finish line**: Running `task build` produces a binary; executing `./bin/agent-cli version` prints a version string derived from the git tag or commit hash (not `dev`).

[ ] **T2.2**: Validation checkpoint — build with injection and verify output
- Run `task build`.
- Execute `./bin/agent-cli version` and assert output matches pattern `agent-cli version .+` (non-empty, non-dev).
- Run `go test ./...` to confirm no regressions.
- **Finish line**: binary prints injected version; all tests pass.
