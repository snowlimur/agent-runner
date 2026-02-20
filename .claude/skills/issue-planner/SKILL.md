---
name: issue-planner
description: Build .issues/<id>/todo.md from GitHub issue details, then commit and push. Use only when explicitly invoked.
argument-hint: <issue-id>
disable-model-invocation: true
---

Input issue id: `$ARGUMENTS`.

Steps:
1. Validate issue id. It must be non-empty and match `^[0-9]+$`.
2. Read issue details with gh:
   - `gh issue view <id> --json number,title,body,labels,url`
3. Set TODO path to `.issues/<id>/todo.md`.
4. If TODO file already exists, return `{"status":"done"}`.
5. If TODO file does not exist:
   - create `.issues/<id>/`
   - analyze the issue and generate a detailed implementation checklist
   - write checklist into `.issues/<id>/todo.md`
   - each task line must start with `[ ] `
6. Commit and push:
   - `git add .issues/<id>/todo.md`
   - `git commit -m "plan(issue:<id>): create todo checklist"`
   - `git push -u origin issue/<id>` (or `git push origin issue/<id>` if upstream already configured)
7. Return `{"status":"created"}`.

Failure behavior:
- On any blocker return `{"status":"failed","reason":"<short reason>"}`.

Output rules:
- Final response must be exactly one JSON object.
- Do not output markdown or explanations.
