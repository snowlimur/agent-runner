# tasks.md — Example

## Task Breakdown

### TASK-001: Add version command

**Description**: Implement a `version` command that prints the application version.
The version must be injectable at build time (e.g. via ldflags) so CI can embed a
git-derived value; fall back to a sensible default when built without injection.

**Agent Type**: backend
**Depends On**: none
**Complexity**: low

**Input Artifacts**:
- Existing CLI entry point and command dispatch code
- Existing command implementations (as pattern reference)
- Existing build tooling configuration

**Subtasks**:
- [ ] Implement version storage with build-time injection support and sensible default
- [ ] Register command in CLI dispatcher, including common flag aliases
- [ ] Update build tooling to inject git-derived version at compile time
- [ ] Update help/usage output to include the new command

**Technical Notes**:
- Follow existing codebase conventions for command structure and output style
- Git version derivation must handle repos with no tags gracefully (fall back to commit hash)
- Do not add external dependencies

**Definition of Done**:
- [ ] `<binary> version` (and common aliases) prints the injected or fallback version
- [ ] Build tooling injects a git-derived version; binary built without injection shows fallback
- [ ] All tests from test-plan.md pass with `-race`
- [ ] `go vet ./... && golangci-lint run .` exits 0

**Failure Modes**:
- ldflags symbol path mismatch → binary silently shows fallback; verify binary output explicitly before marking done

---

## Risk Register

| Risk                        | Impact | Probability | Mitigation                                                  | Affects Tasks |
|-----------------------------|--------|-------------|-------------------------------------------------------------|---------------|
| ldflags symbol path mismatch| medium | medium      | Verify binary output explicitly in DoD before marking done  | TASK-001      |
| No git tags in repo         | low    | medium      | Fall back to commit hash; document expected fallback output | TASK-001      |
