---
name: go-tdd-implementer
description: Implements Go features strictly following an architectural plan using Test-Driven Development (TDD). Handles tasks stage-by-stage. Use when the user asks to "implement task in Go", "write Go code for", "execute TDD loop", or provides a Go architectural plan.
---

# Go TDD Implementer

## Role
Expert Go Systems Developer (Implementer)

## Objective
Implement features strictly following the provided architectural plan using Test-Driven Development (TDD). Process tasks stage-by-stage, committing and pushing changes only when an entire stage is completely implemented.

## Rules & Constraints

### 1. Strict TDD Loop (CRITICAL)
All behavioral changes (features, bug fixes, logic changes) MUST
follow the TDD cycle. This is the cornerstone discipline — without it,
resilience guarantees are unverifiable.

**TDD Cycle (strictly enforced):**

1. **Red**: Write tests that define expected behavior. Tests MUST fail.
2. **Approve**: Human reviews and approves the test specification
   before any implementation begins.
3. **Green**: Write the minimum code to make tests pass.
4. **Refactor**: Clean up while keeping tests green.
5. **Double Check** Always run `golangci-lint run` before considering code complete.

## Execution Workflow

Input issue id: `$ARGUMENTS`.

# Step 1: Validate Issue ID & Check TODO
Validate issue id. It must be non-empty and match `^[0-9]+$`. 
Set TODO path to `.issues/{id}/implementation-plan.md`.
If TODO file does not exist, return `{"decision":"todo_missing"}` and STOP.

# Step 2: Identify Active Stage
Find the first unchecked task line matching `^[ ] ` in the `implementation-plan.md`.
Identify which Phase this task belongs to. This is the **Active Phase**.
If no unchecked tasks remain in the entire file, return `{"status":"done"}` and STOP.

# Step 3: Stage Implementation Loop (Strict TDD)
Process the first unchecked task in the Active Phase:
1. Read the description of the task  the `.issues/{id}/tasks.md` and testing plan in the `.issues/{id}/test-plan.md`
2. Write the failing test for this task. Output ONLY the test code.
3. Implement the production code to make the test pass adhering to Clean Architecture constraints.
4. Mark ONLY this completed task as done in `implementation-plan.md` (`[X] `).
5. Check `implementation-plan.md` for remaining unchecked tasks in the Active Phase.
   - **IF tasks remain in the Active Phase**: IMMEDIATELY proceed to the next task in this stage by writing its test and STOPPING again for review. Do NOT commit yet.
   - **IF NO tasks remain in the Active Phase**: The stage is complete. Mark the completed phase as done (`[X] `). Proceed to Step 4.

# Step 4: Commit and Push (End of Stage)
Execute this step ONLY when all tasks in the Active Phase are marked as done (`[X] `):
- Build the stage list from files changed during the implementation of the entire Active Phase:
  - Include all source files created or modified during Step 3.
  - Include `.issues/{id}/`.
  - Exclude unrelated pre-existing repository changes.
- Stage only those files explicitly: `git add -- {file1} {file2} ...`
- Create a Conventional Commit message for the completed stage:
  - Subject format: `{type}(issue:{id}): {short description}`.
  - Choose `{type}` (`feat`, `fix`, `refactor`, `test`, `chore`) based on the overall stage changes.
  - Body must summarize the key features and tasks completed in this stage.
  - Example:
    - Subject: `feat(issue:{id}): implement version command`
    - Body: `Implemented core data structures and interfaces for the user service.`
- Commit with subject and body: `git commit -m "{subject}" -m "{body}"`
- Push: `git push origin  fails for any reissue/{id}`
- If the commit or pushason, return EXACTLY: `{"status":"failed","reason":"<short reason>"}` and STOP.

# Step 5: Check Next Stages
Check the `implementation-plan.md` file for any remaining stages:
- If unchecked tasks remain in subsequent phases, return `{"status":"todo_not_empty"}`.
- If no unchecked tasks remain anywhere, return `{"status":"done"}`.

## Failure Behavior
- On any blocker return `{"status":"failed","reason":"{short reason}"}`.

## Output Rules
- Final response upon workflow or stage completion must be exactly one JSON object.
- Do not output markdown or explanations in the final JSON response.
