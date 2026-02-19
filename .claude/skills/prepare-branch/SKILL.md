---
name: prepare-branch
description: Prepare local git state for issue work by syncing main and checking out issue/<id>. Use only when explicitly invoked.
argument-hint: <issue-id>
disable-model-invocation: true
---

Input issue id: `$ARGUMENTS`.

Steps:
1. Validate issue id. It must be non-empty and match `^[0-9]+$`.
2. Synchronize local `main` with `origin/main`:
   - `git fetch --prune origin`
   - if local `main` exists: `git checkout main` then `git pull --ff-only origin main`
   - else: `git checkout -B main origin/main`
3. Switch to `issue/<id>`:
   - if `origin/issue/<id>` exists: `git checkout -B issue/<id> origin/issue/<id>`
   - else if local branch exists: `git checkout issue/<id>`
   - else: `git checkout -b issue/<id>`
4. Return JSON only:
   - success: `{"status":"branch_ready"}`
   - failure: `{"status":"failed","reason":"<short reason>"}`

Output rules:
- Final response must be exactly one JSON object.
- Do not output markdown or explanations.
