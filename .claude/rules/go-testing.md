# Golang Testing

## Testing
- **T-1 (MUST)** Table-driven tests; deterministic and hermetic by default.
- **T-2 (MUST)** Run `-race` in CI; add `t.Cleanup` for teardown.
- **T-3 (SHOULD)** Add `t.Parallel()` only when the test exercises concurrent behavior (goroutines, channels, race conditions) or the suite is measurably slow. Do NOT add it by default to sequential unit tests — sequential tests are simpler and easier to debug.