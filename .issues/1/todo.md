# Issue #1 — Add app version command

> **Goal:** Add a `version` command to `agent-cli` that displays the current application version.
> **Repo pattern:** Custom switch-based command dispatcher in `main.go`; commands are functions in `internal/cli/`; flags via stdlib `flag`; tests use `t.Parallel()` and `t.Fatalf`.

---

## S1 — Version infrastructure and command implementation

[X] **T1.1**: Create version variable with build-time injection support
- Create `@agent-cli/internal/version/version.go`.
- Define package `version` with an unexported module-level `var version string` and a public `func Version() string` that returns it (defaulting to `"dev"` when unset).
- The variable is populated at build time via `-ldflags "-X <module>/internal/version.version=<semver>"`.
- **Deps:** none (stdlib only).
- **Test signature (`@agent-cli/internal/version/version_test.go`):**
  ```go
  func TestVersionDefault(t *testing.T)
  // Asserts Version() == "dev" when no ldflags are provided.
  ```
- **Pass criteria:** `go test ./internal/version/...` passes; `Version()` returns `"dev"`.

[X] **T1.2**: Implement `VersionCommand` function
- Create `@agent-cli/internal/cli/version.go`.
- Signature: `func VersionCommand(w io.Writer) error`.
- Accepts an `io.Writer` to enable testable output capture.
- Prints `agent-cli version <version>` (newline-terminated) using `version.Version()`.
- Returns `nil` (no error paths).
- **Deps:** `internal/version`, `io`, `fmt`.
- **Test signatures (`@agent-cli/internal/cli/version_test.go`):**
  ```go
  func TestVersionCommandOutput(t *testing.T)
  // Calls VersionCommand with a bytes.Buffer, asserts output matches "agent-cli version dev\n".
  ```
- **Pass criteria:** `go test ./internal/cli/...` passes; output format verified.

[X] **T1.3**: Compilation and test checkpoint for S1
- Run `go build ./...` from `agent-cli/` — must compile with zero errors.
- Run `go test ./internal/version/... ./internal/cli/...` — all tests green.
- **Pass criteria:** exit code 0 for both commands.

---

## S2 — CLI integration and help text

[X] **T2.1**: Register `version` command in the main dispatcher
- Edit `@agent-cli/main.go`.
- Add `case "version":` to the existing command switch, calling `cli.VersionCommand(os.Stdout)`.
- Add `case "-v", "--version":` mapping to the same handler so top-level version flag works.
- **Deps:** `os` (already imported), `internal/cli` (already imported).
- **Test signatures (`@agent-cli/main_test.go` — new file):**
  ```go
  func TestMainDispatchVersion(t *testing.T)
  // Executes the built binary with "version" arg via os/exec, asserts stdout contains "agent-cli version".

  func TestMainDispatchVersionFlag(t *testing.T)
  // Executes the built binary with "--version" flag via os/exec, asserts stdout contains "agent-cli version".
  ```
- **Pass criteria:** `go test ./...` passes; `agent-cli version` and `agent-cli --version` both print version string.

[X] **T2.2**: Update usage/help text
- Edit `@agent-cli/main.go` `printUsage()` function.
- Add `version` command entry to the usage output (e.g., `  version     Show the application version`).
- **Test signature (extend `@agent-cli/main_test.go`):**
  ```go
  func TestHelpIncludesVersion(t *testing.T)
  // Executes the built binary with "help" arg via os/exec, asserts stdout contains "version".
  ```
- **Pass criteria:** `agent-cli help` output includes the `version` command description.

[X] **T2.3**: Final validation checkpoint
- Run `go build ./...` — zero errors.
- Run `go vet ./...` — zero warnings.
- Run `go test ./...` — all tests green.
- **Pass criteria:** exit code 0 for all three commands.
