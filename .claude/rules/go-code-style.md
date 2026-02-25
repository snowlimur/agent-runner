# Golang Code Style

## Code Style
- **CS-1 (MUST)** Enforce `gofmt`, `go vet`.
- **CS-2 (MUST)** Avoid stutter in names: `package kv; type Store` (not `KVStore` in `kv`).
- **CS-3 (SHOULD)** Small interfaces near consumers; prefer composition over inheritance.
- **CS-4 (SHOULD)** Avoid reflection on hot paths; prefer generics when it clarifies and speeds.
- **CS-5 (MUST)** Use input structs for functions receiving more than 2 arguments.
  Input contexts should NOT go in the input struct.
- **CS-6 (SHOULD)** Declare function input structs before the function consuming them.
- **CS-7 (MUST)** Inject all dependencies explicitly via constructors. Global state is strictly forbidden.

## Errors
- **ERR-1 (MUST)** Wrap with `%w` and context: `fmt.Errorf("open %s: %w", p, err)`.
- **ERR-2 (MUST)** Use `errors.Is`/`errors.As` for control flow; no string matching.
- **ERR-3 (SHOULD)** Define sentinel errors in the package; document behavior.
- **ERR-4 (CAN)** Use `context.WithCancelCause` and `context.Cause` for propagating error causes.

## Concurrency
- **CC-1 (MUST)** The **sender** closes channels; receivers never close.
- **CC-2 (MUST)** Tie goroutine lifetime to a `context.Context`; prevent leaks.
- **CC-3 (MUST)** Protect shared state with `sync.Mutex`/`atomic`; no "probably safe" races. Conversely, do NOT add sync primitives to variables that are never written concurrently in production (e.g. build-time vars set via `-ldflags`, or test-only setters). Check two goroutines can write this simultaneously in a real binary — if not, no mutex needed.
- **CC-4 (SHOULD)** Use `errgroup` for fan-out work; cancel on first error.
- **CC-5 (CAN)** Prefer buffered channels only with rationale (throughput/back-pressure).

## Contexts
- **CTX-1 (MUST)** `ctx context.Context` is always the first parameter; never store ctx in structs.
- **CTX-2 (MUST)** Propagate non-nil `ctx`; honor `Done`/deadlines/timeouts.

## Logging & Observability
- **OBS-1 (MUST)** Structured logging (`slog`) with levels and consistent fields.
- **OBS-2 (SHOULD)** Correlate logs/metrics/traces via request IDs from context.

## Modules & Dependencies
- **MD-1 (SHOULD)** Prefer stdlib; introduce deps only with clear payoff;
  track transitive size and licenses.
- **MD-2 (CAN)** Use `govulncheck` for vulnerability scans.

## Configuration
- **CFG-1 (MUST)** Config via env/flags; validate on startup; fail fast.
- **CFG-2 (MUST)** Treat config as immutable after init; pass explicitly (not via globals).
- **CFG-3 (SHOULD)** Provide sane defaults and clear docs.

## APIs & Boundaries
- **API-1 (MUST)** Document exported items: `// Foo does …`; keep exported surface minimal.
- **API-2 (MUST)** Accept interfaces where variation is needed; **return concrete types**
  unless abstraction is required.
- **API-3 (SHOULD)** Keep functions small, orthogonal, and composable.