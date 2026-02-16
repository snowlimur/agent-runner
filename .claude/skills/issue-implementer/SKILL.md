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
   - `git add -A`
   - `git commit -m "feat(issue:<id>): complete next todo item"`
   - `git push origin issue/<id>`
9. If unchecked tasks remain, return `{"decision":"implementer"}`.
10. If no unchecked tasks remain, return `{"decision":"done"}`.

Failure behavior:
- On any blocker return `{"decision":"failed","reason":"<short reason>"}`.

Output rules:
- Final response must be exactly one JSON object.
- Do not output markdown or explanations.
