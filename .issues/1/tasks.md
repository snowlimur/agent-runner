# Implementation Tasks

## Task Breakdown

### TASK-001: Add version command with build-time injection

**Description**: Implement a `version` command for agent-cli that prints the application version.
The version must be injectable at build time via ldflags so CI/CD can embed a git-derived value;
it must fall back to a sensible default (e.g. `"dev"`) when built without injection.
The command must integrate with the existing custom switch-based command dispatcher in `main.go`
and follow the established patterns for command implementation in `internal/cli/`.

**Agent Type**: backend
**Depends On**: none
**Complexity**: low

**Input Artifacts**:
- `agent-cli/main.go` — CLI entry point with switch-based command dispatch and `printUsage()` function
- `agent-cli/internal/cli/` — existing command implementations (`run.go`, `stats.go`) as pattern reference
- `agent-cli/Taskfile.yml` — build tasks (currently `go build` with no ldflags)
- `agent-cli/.golangci.yml` — linter configuration

**Subtasks**:
- [ ] Implement version storage with build-time injection support and a sensible default fallback
- [ ] Implement version command handler following existing CLI patterns
- [ ] Register the version command in the main dispatcher, including common aliases (`version`, `-v`, `--version`)
- [ ] Update `printUsage()` to include the version command
- [ ] Update build task in `Taskfile.yml` to inject git-derived version via ldflags
- [ ] Ensure git version derivation handles repos with no tags gracefully (fall back to commit hash or "dev")

**Technical Notes**:
- The CLI uses a custom `switch` dispatcher on `os.Args[1]`, not a framework like Cobra — follow this pattern
- Existing commands use the standard `flag` package for flag parsing — maintain consistency
- Error handling uses `fmt.Errorf()` with `%w` wrapping — follow this convention
- The version command is output-only (no flags required beyond what the dispatcher provides)
- Do not add external dependencies
- The build currently uses `go build -o bin/agent-cli .` — extend with `-ldflags` for version injection

**Definition of Done**:
- [ ] `agent-cli version` prints the injected or fallback version string
- [ ] `-v` and `--version` aliases also print the version
- [ ] Build task injects a git-derived version; binary built without injection shows the default fallback
- [ ] `printUsage()` output includes the version command
- [ ] All tests pass with `-race` flag
- [ ] `go vet ./...` exits 0
- [ ] `golangci-lint run .` exits 0

**Failure Modes**:
- ldflags symbol path mismatch -> binary silently shows fallback instead of injected version; verify binary output explicitly before marking done
- Taskfile.yml syntax error in ldflags interpolation -> build task fails; test `task cli:build` explicitly

---

## Dependency Matrix

```
TASK-001: (no dependencies)
```

## Execution Graph

```
TASK-001 [version command]
   |
   v
  DONE
```

## Risk Register

| Risk | Impact | Probability | Mitigation | Affects Tasks |
|------|--------|-------------|------------|---------------|
| ldflags symbol path mismatch | medium | medium | Verify binary output explicitly in DoD before marking done | TASK-001 |
| No git tags in repo | low | medium | Fall back to commit hash; if no commit, use "dev" default | TASK-001 |
| Taskfile.yml ldflags interpolation | low | low | Test `task cli:build` explicitly and verify output | TASK-001 |
