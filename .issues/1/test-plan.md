# Test Plan

## Strategy

### Testing Pyramid

- **Unit (80%)**: Version output logic, default fallback behavior, command dispatch
- **Integration (20%)**: Built binary output with and without ldflags injection

---

## Test Categories

### Unit Tests

**Coverage Target**: 80%
**Command**: `cd agent-cli && go test -race -cover ./...`
**Pass Condition**: coverage >= 80%, exit code 0, no race conditions detected

**Scope**:
- Version value returns the injected value when set via ldflags
- Version value returns the default fallback when not injected
- Version command handler produces expected output format
- Command dispatch routes `version`, `-v`, `--version` to the version handler
- Unknown commands still produce usage output and error

---

### Integration Tests

**Coverage Target**: 100% of version command surface
**Command**: `cd agent-cli && task build && ./bin/agent-cli version`
**Pass Condition**: output contains a version string, exit code 0

**Scope**:
- Binary built with ldflags prints the injected version
- Binary built without ldflags prints the default fallback version
- `agent-cli version`, `agent-cli -v`, `agent-cli --version` all produce consistent output
- Version output is a single line suitable for scripting (e.g. `agent-cli version | head -1`)

---

## Test Data

**Strategy**: No external data or fixtures required.

| Level | Data Source | Reset Strategy |
|-------|-----------|----------------|
| Unit | Inline constants, no I/O | n/a |
| Integration | Built binary | Rebuild per test scenario |

**Isolation requirement**: No test may depend on state left by another test.

---

## Verification Commands

| Check | Command | Expected |
|-------|---------|----------|
| Unit tests pass | `cd agent-cli && go test -race -cover ./...` | exit 0 |
| Vet passes | `cd agent-cli && go vet ./...` | exit 0 |
| Lint passes | `cd agent-cli && golangci-lint run .` | exit 0 |
| Build succeeds | `cd agent-cli && task build` | exit 0 |
| Version output | `./agent-cli/bin/agent-cli version` | prints version string |
