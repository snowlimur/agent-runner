# Implementation Tasks

## Task Breakdown

### TASK-001: Add version command with build-time injection

**Description**: Implement a `version` command that prints the application version.
The version must be injectable at build time via ldflags so that CI and the build
system can embed a git-derived value. When built without injection, the binary must
fall back to a sensible default (e.g. `dev`). The command must integrate into the
existing manual command dispatch and follow existing codebase conventions.

**Agent Type**: backend
**Depends On**: none
**Complexity**: low

**Input Artifacts**:
- `main.go` — entry point with switch-based command dispatch and `printUsage()`
- `internal/cli/` — existing command implementations (pattern reference)
- `Taskfile.yml` — current build configuration (no ldflags yet)

**Subtasks**:
- [ ] Implement version storage with a package-level variable that supports build-time injection via ldflags, with a sensible default for uninjected builds
- [ ] Implement the version command handler following existing conventions in `internal/cli/`
- [ ] Register `version` (and common aliases like `-v`, `--version`) in the CLI dispatcher
- [ ] Update `printUsage()` to include the version command
- [ ] Update `Taskfile.yml` build task to inject a git-derived version string via ldflags
- [ ] Git version derivation must handle repos with no tags (fall back to short commit hash)

**Technical Notes**:
- The CLI uses a manual switch statement in `main.go` for command dispatch — no external CLI framework
- Existing commands (`run`, `stats`) are implemented as exported functions in `internal/cli/`
- Follow the same output style: plain text to stdout, no decorations
- Do not add external dependencies
- Use Go 1.26 features where appropriate per project guidelines
- The linter configuration is strict (50+ linters); ensure code passes `golangci-lint run .`

**Definition of Done**:
- [ ] `agent-cli version` prints the injected or fallback version string
- [ ] `agent-cli -v` and `agent-cli --version` produce the same output
- [ ] `task build` injects a git-derived version; the resulting binary shows the real version
- [ ] A plain `go build .` (without ldflags) produces a binary that shows the fallback default
- [ ] All tests pass with `-race` flag
- [ ] `go vet ./...` and `golangci-lint run .` exit 0
- [ ] Help/usage output includes the version command

**Failure Modes**:
- ldflags symbol path mismatch → binary silently shows fallback; verify binary output explicitly before marking done
- `git describe` fails in shallow clones or repos with no tags → ensure fallback to short commit hash

---

## Dependency Matrix

```
TASK-001 → (no dependencies)
```

## Execution Graph

```
TASK-001 [version command]
   └── (independent, can start immediately)
```

## Risk Register

| Risk                         | Impact | Probability | Mitigation                                                        | Affects Tasks |
|------------------------------|--------|-------------|-------------------------------------------------------------------|---------------|
| ldflags symbol path mismatch | medium | medium      | Verify binary output explicitly in DoD before marking done        | TASK-001      |
| No git tags in repo          | low    | medium      | Fall back to commit hash; test both tagged and untagged scenarios  | TASK-001      |
| Strict linter rejects code   | low    | low         | Run `golangci-lint run .` before marking done; fix all findings   | TASK-001      |
