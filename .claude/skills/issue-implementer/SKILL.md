---
name: issue-implementer
description: Execute one top unchecked todo item for issue <id>, mark it done, commit and push. Use only when explicitly invoked.
argument-hint: <issue-id>
disable-model-invocation: true
---

Input issue id: `$ARGUMENTS`.

Steps:
1. Validate issue id. It must be non-empty and match `^[0-9]+$`.
2. Set TODO path to `.issues/<id>/todo.md`.
3. If TODO file does not exist, return `{"decision":"todo_missing"}`.
4. Find the first unchecked line matching `^[ ]` marker format `^\[ \] `.
5. If no unchecked tasks remain, return `{"decision":"done"}`.
6. Implement only that first unchecked task.
7. Mark that same task as done in TODO (`[X] `).
8. Commit and push:
   - Capture the completed task text as `<task_text>` (without `[ ]` / `[X]` marker).
   - Build the stage list from files changed by this implementation only:
     - Include files created or modified while implementing step 6.
     - Include `.issues/<id>/todo.md` changed in step 7.
     - Exclude unrelated pre-existing repository changes.
   - Stage only those files explicitly: `git add -- <file1> <file2> ...`.
   - Create a Conventional Commit message that reflects both the completed task and actual code changes:
     - Subject format: `<type>(issue:<id>): <short task outcome>`.
     - Choose `<type>` from Conventional Commits (`feat`, `fix`, `refactor`, `docs`, `test`, `chore`) based on the implemented change.
     - Subject must mention the completed todo task.
     - Body must summarize key modifications made for that task.
   - Example:
     - Subject: `feat(issue:<id>): implement <task_text>`
     - Body lines:
       - `Task: <task_text>`
       - `Changes: <short summary of concrete file/code updates>`
   - Commit with subject and body (`git commit -m "<subject>" -m "<body>"`).
   - `git push origin issue/<id>`
9. If unchecked tasks remain, return `{"status":"todo_not_empty"}`.
10. If no unchecked tasks remain, return `{"status":"done"}`.

Failure behavior:
- On any blocker return `{"status":"failed","reason":"<short reason>"}`.

Output rules:
- Final response must be exactly one JSON object.
- Do not output markdown or explanations.
