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
- You MUST write the failing test FIRST for the active task. 
- Implement the production code to make the test pass.

### 2. Clean Architecture & State
- Inject all dependencies explicitly via constructors. Global state is strictly forbidden.
- Pass `context.Context` explicitly as the first parameter to all blocking, I/O, or asynchronous functions. NEVER embed or store `context.Context` inside structs.
- Prefer composition over inheritance.

### 3. Concurrency Safety
- Use `golang.org/x/sync/errgroup` for managing multiple goroutines instead of naked `sync.WaitGroup`.
- Ensure explicit error handling and context cancellation to prevent goroutine leaks.

### 4. Error Handling & Observability
- Treat errors as values. Use `errors.Is` and `errors.As`. Never use the `err ==` anti-pattern for equality checks.
- Use `log/slog` for all structured logging. Never use `fmt.Printf` or `log.Fatal` in production application logic.

### 5. Memory & Performance
- Pre-allocate slices and maps with known capacity to avoid memory churn: `make(T, 0, capacity)`.

## Execution Workflow

Input issue id: `$ARGUMENTS`.

-# Step 1: Validate Issue ID & Check TODO
Validate issue id. It must be non-empty and match `^[0-9]+$`. 
Set TODO path to `.issues/{id}/todo.md`.
If TODO file does not exist, return `{"decision":"todo_missing"}` and STOP.

-# Step 2: Identify Active Stage
Find the first unchecked task line matching `^[ ] ` in the `todo.md`.
Identify which Stage (e.g., S1, S2) this task belongs to. This is the **Active Stage**.
If no unchecked tasks remain in the entire file, return `{"status":"done"}` and STOP.

-# Step 3: Stage Implementation Loop (Strict TDD)
Process the first unchecked task in the Active Stage:
1. Write the failing test for this task. Output ONLY the test code.
2. Implement the production code to make the test pass adhering to Clean Architecture constraints.
3. Mark ONLY this completed task as done in `todo.md` (`[X] `).
4. Check `todo.md` for remaining unchecked tasks in the Active Stage.
   - **IF tasks remain in the Active Stage**: IMMEDIATELY proceed to the next task in this stage by writing its test and STOPPING again for review. Do NOT commit yet.
   - **IF NO tasks remain in the Active Stage**: The stage is complete. Proceed to Step 4.

-# Step 4: Commit and Push (End of Stage)
Execute this step ONLY when all tasks in the Active Stage are marked as done (`[X] `):
- Build the stage list from files changed during the implementation of the entire Active Stage:
  - Include all source files created or modified during Step 3.
  - Include `.issues/{id}/todo.md`.
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
- Push: `git push origin issue/{id}`
- If the commit or push fails for any reason, return EXACTLY: `{"status":"failed","reason":"<short reason>"}` and STOP.

-# Step 5: Check Next Stages
Check the TODO file for any remaining stages:
- If unchecked tasks remain in subsequent stages, return `{"status":"todo_not_empty"}`.
- If no unchecked tasks remain anywhere, return `{"status":"done"}`.

## Failure Behavior
- On any blocker return `{"status":"failed","reason":"{short reason}"}`.

## Output Rules
- Final response upon workflow or stage completion must be exactly one JSON object.
- Do not output markdown or explanations in the final JSON response.
