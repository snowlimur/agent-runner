# Test Plan

## Strategy

### Testing Pyramid

- **Unit (80%)**: Version output logic, fallback behavior, flag/alias parsing
- **Integration (20%)**: CLI dispatch for version command and aliases

---

## Test Categories

### Unit Tests

**Coverage Target**: 80%
**Command**: `cd agent-cli && go test -race -count=1 ./...`
**Pass Condition**: all tests pass with `-race`, exit code 0

**Scope**:
- Version command outputs the current version string to stdout
- Default/fallback version is returned when no build-time injection occurs
- Injected version (simulated via test setup) is returned correctly
- Version output format is consistent (plain text, single line, no trailing decoration)

---

### Integration Tests (CLI Dispatch)

**Command**: `cd agent-cli && go test -race -count=1 ./...`
**Pass Condition**: all tests pass, exit code 0

**Scope**:
- `version` command is dispatched correctly from the CLI entry point
- `-v` and `--version` aliases produce the same output as `version`
- Help/usage text includes the version command
- Unknown commands still produce the expected error

---

### Build Verification (Manual / CI)

**Command**: `cd agent-cli && task build && ./bin/agent-cli version`
**Pass Condition**: output contains a git-derived version (not the fallback default)

**Scope**:
- `task build` succeeds and injects version via ldflags
- The built binary prints the injected version
- A plain `go build .` (without ldflags) produces a binary that prints the fallback default

---

### Lint Verification

**Command**: `cd agent-cli && golangci-lint run .`
**Pass Condition**: exit code 0, zero findings

---

## Test Data

| Level       | Data Source                          | Reset Strategy |
|-------------|--------------------------------------|----------------|
| Unit        | Inline string constants, no I/O      | n/a            |
| Integration | CLI argument arrays, captured stdout | n/a            |
