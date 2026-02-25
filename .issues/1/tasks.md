# Implementation Tasks

## Task Breakdown

### TASK-001: Add version command with build-time injection

**Description**: Implement a `version` command that prints the application version.
The version must be injectable at build time via ldflags so CI and the Taskfile can embed a
git-derived value; fall back to a sensible default (e.g. `dev`) when built without injection.

**Agent Type**: backend
**Depends On**: none
**Complexity**: low

**Input Artifacts**:
- `agent-cli/main.go` — CLI entry point with command dispatch switch
- `agent-cli/Taskfile.yml` — build configuration (currently has no ldflags)
- `agent-cli/internal/cli/` — existing command implementations (pattern reference)

**Subtasks**:
- [ ] Implement version storage with build-time injection support and a sensible default
- [ ] Register `version` command (and `--version` flag) in the CLI dispatcher
- [ ] Update `printUsage()` to include the version command
- [ ] Update `Taskfile.yml` build task to inject git-derived version via `-ldflags`
- [ ] Handle repos with no tags gracefully (fall back to commit hash or `dev`)

**Technical Notes**:
- The CLI uses a hand-rolled command dispatcher (switch on `os.Args[1]`), not a framework
- Build tooling is Task (Taskfile.yml), not Make — update the `build` task accordingly
- Git version derivation must handle repos with no tags gracefully
- Do not add external dependencies
- Follow existing output conventions (plain text to stdout)

**Definition of Done**:
- [ ] `agent-cli version` and `agent-cli --version` print the injected or fallback version
- [ ] `task build` injects a git-derived version; binary built with plain `go build` shows fallback
- [ ] All tests pass with `-race` flag
- [ ] `go vet ./...` exits 0

**Failure Modes**:
- ldflags symbol path mismatch → binary silently shows fallback; verify binary output explicitly before marking done
- No git tags in repo → fall back to commit hash; verify fallback output in test

---

## Risk Register

| Risk                         | Impact | Probability | Mitigation                                                  | Affects Tasks |
|------------------------------|--------|-------------|-------------------------------------------------------------|---------------|
| ldflags symbol path mismatch | medium | medium      | Verify binary output explicitly in DoD before marking done  | TASK-001      |
| No git tags in repo          | low    | medium      | Fall back to commit hash; document expected fallback output | TASK-001      |
