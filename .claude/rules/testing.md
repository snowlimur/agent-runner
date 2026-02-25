---
paths:
  - "src/api/**/*_test.go"
---

## Testing
- **T-1 (MUST)** Table-driven tests; deterministic and hermetic by default.
- **T-2 (MUST)** Run `-race` in CI; add `t.Cleanup` for teardown.
- **T-3 (SHOULD)** Mark safe tests with `t.Parallel()`.