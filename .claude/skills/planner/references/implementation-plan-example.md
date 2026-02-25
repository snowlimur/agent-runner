# implementation-plan.md — Example

## Execution Phases

### [ ] Phase 1: Core Feature

**Tasks**:
- [ ] TASK-001

**Gate**: quality gates from test-plan.md

---

## Technical Risk Mitigation

| Risk                          | Mitigation                                                          |
|-------------------------------|---------------------------------------------------------------------|
| ldflags symbol path mismatch  | Verify binary output explicitly before marking done                 |
| Flaky tests blocking pipeline | Mark as flaky, quarantine immediately, fix before phase completes   |

---

## Quality Gates (Non-Negotiable)

| Metric                  | Threshold       | Command                        |
|-------------------------|-----------------|--------------------------------|
| Unit test coverage      | ≥ 80%           | `make test-unit`               |
| Integration tests       | 100% pass       | `make test-integration`        |
| Build                   | exit 0          | `make build`                   |
| Lint                    | exit 0          | `make lint`                    |
