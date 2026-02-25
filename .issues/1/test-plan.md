# Test Plan

## Strategy

### Testing Pyramid

- **Unit (80%)**: Version output logic, command dispatch routing, flag alias handling
- **Integration (20%)**: Built binary output verification with and without ldflags injection

---

## Test Categories

### Unit Tests

**Coverage Target**: 80%
**Command**: `cd agent-cli && go test -race ./...`
**Pass Condition**: coverage >= 80% for changed packages, exit code 0

**Scope**:
- Version command returns correct output when version variable has default value
- Version command returns correct output when version variable is injected
- Command dispatch routes `version`, `--version`, `-v` to version handler
- Help/usage output includes the `version` command
- Unknown commands still produce correct error behavior (regression)

---

### Integration Tests

**Command**: `cd agent-cli && task build && ./bin/agent-cli version`
**Pass Condition**: binary prints version string, exit code 0

**Scope**:
- Binary built via `task build` prints a git-derived version (not the default fallback)
- Binary built via plain `go build` prints the default fallback version
- `--version` and `-v` flags produce same output as `version` subcommand
- Exit code is 0 for all version output paths

---

## Test Data

**Strategy**: No external data needed. All tests use inline values.

| Level       | Data Source                         | Reset Strategy |
|-------------|-------------------------------------|----------------|
| Unit        | Inline string constants, no I/O     | n/a            |
| Integration | Built binary, git repo state        | n/a            |

**Isolation requirement**: No test may depend on state left by another test. Version tests must not modify global state or require specific git tags to be present.
