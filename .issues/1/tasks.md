# Implementation Tasks

## Task Breakdown

### TASK-001: Add version command with build-time injection

**Description**: Implement a `version` command that prints the application version.
The version must be injectable at build time via ldflags so CI and the Taskfile can
embed a git-derived value; fall back to a sensible default (e.g. `"dev"`) when built
without injection. The command should also support common flag aliases (`--version`, `-v`).

**Agent Type**: backend
**Depends On**: none
**Complexity**: low

**Input Artifacts**:
- `agent-cli/main.go` — CLI entry point with switch-based command dispatch
- `agent-cli/internal/cli/` — existing command implementations (pattern reference for `run`, `stats`)
- `agent-cli/Taskfile.yml` — build tooling configuration

**Subtasks**:
- [ ] Implement version storage with build-time injection support and a sensible default
- [ ] Register `version` command (and aliases `--version`, `-v`) in CLI dispatcher
- [ ] Update `printUsage()` to include the new `version` command
- [ ] Update `Taskfile.yml` build task to inject git-derived version via ldflags
- [ ] Ensure git version derivation handles repos with no tags gracefully (fall back to commit hash)

**Technical Notes**:
- Follow existing codebase conventions: switch-case dispatch in `main.go`, plain stdout output, no external CLI framework
- Use `ldflags -X` for build-time injection; the linker symbol path must match the actual package + variable
- Git version derivation in the Taskfile should handle: tagged commits, untagged commits, dirty worktrees
- Do not add any external dependencies
- Go 1.26 features are available (see project rules)

**Definition of Done**:
- [ ] `agent-cli version` prints the injected or fallback version string
- [ ] `agent-cli --version` and `agent-cli -v` produce the same output
- [ ] `task build` injects a git-derived version; binary built with plain `go build` shows fallback
- [ ] `agent-cli help` output includes the `version` command
- [ ] All tests pass with `-race` flag
- [ ] `go vet ./...` exits 0
- [ ] `golangci-lint run .` exits 0

**Failure Modes**:
- ldflags symbol path mismatch → binary silently shows fallback; verify actual binary output explicitly before marking done
- No git tags in repo → version derivation must fall back to commit hash or `"dev"`; test this scenario

---

## Dependency Matrix

```
TASK-001 → (none)
```

Single-task plan; no inter-task dependencies.

---

## Execution Graph

```
[TASK-001: version command] ── (start) ── (done)
```

---

## Risk Register

| Risk                         | Impact | Probability | Mitigation                                                   | Affects Tasks |
|------------------------------|--------|-------------|--------------------------------------------------------------|---------------|
| ldflags symbol path mismatch | medium | medium      | Verify binary output explicitly in DoD before marking done   | TASK-001      |
| No git tags in repo          | low    | medium      | Fall back to commit hash; document expected fallback output  | TASK-001      |
| Taskfile shell portability   | low    | low         | Use POSIX-compatible shell commands for git version derivation| TASK-001      |
