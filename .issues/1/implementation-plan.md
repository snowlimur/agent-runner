# Implementation Plan

## Execution Phases

### [ ] Phase 1: Version Command

**Tasks**:
- [ ] TASK-001: Add version command with build-time injection

**Gate**: All quality gates from test-plan.md pass

---

## Technical Risk Mitigation

| Risk | Mitigation |
|------|-----------|
| ldflags symbol path mismatch | Verify binary output explicitly after build — must print injected version, not fallback |
| No git tags in repo | Version derivation script must handle missing tags by falling back to commit hash or "dev" |
| Taskfile.yml interpolation errors | Run `task cli:build` and verify exit code before marking done |

---

## Quality Gates (Non-Negotiable)

| Metric | Threshold | Command |
|--------|-----------|---------|
| Unit test coverage | >= 80% | `cd agent-cli && go test -race -cover ./...` |
| Vet | exit 0 | `cd agent-cli && go vet ./...` |
| Lint | exit 0 | `cd agent-cli && golangci-lint run .` |
| Build | exit 0 | `cd agent-cli && task build` |
| Version output | prints version string | `./agent-cli/bin/agent-cli version` |
