# Test Plan

## Strategy

### Testing Pyramid

- **Unit (80%)**: Version output logic, default value behavior, command dispatch
- **Integration (20%)**: Build with ldflags and verify binary output

---

## Test Categories

### Unit Tests

**Coverage Target**: 80%
**Command**: `task test` (or `go test -race ./...` from `agent-cli/`)
**Pass Condition**: coverage ≥ 80% for version-related code, exit code 0

**Scope**:
- Version command prints correct output to stdout
- Default version value is used when no build-time injection occurs
- `version` and `--version` both route to the version handler
- Output format is consistent and parseable

---

### Integration Tests

**Command**: `task build && ./bin/agent-cli version`
**Pass Condition**: binary outputs a git-derived version string, exit code 0

**Scope**:
- Build with ldflags injects the expected version string
- Binary built without ldflags shows fallback default
- Version output is a single line, suitable for scripting (`agent-cli version | head -1`)

---

## Test Data

| Level       | Data Source             | Reset Strategy |
|-------------|------------------------|----------------|
| Unit        | Inline fixtures, no IO | n/a            |
| Integration | Built binary           | Rebuild        |

**Isolation requirement**: No test may depend on state left by another test.
