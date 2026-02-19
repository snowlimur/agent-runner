#!/usr/bin/env bash
set -euo pipefail

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "${branch}" != issue/* ]]; then
  echo "Expected branch issue/<TASK>, got: ${branch}" >&2
  exit 20
fi

task="${branch#issue/}"
if [[ -z "${task}" || ! "${task}" =~ ^[0-9]+$ ]]; then
  echo "Invalid issue id parsed from branch: ${task}" >&2
  exit 20
fi

labels="$(gh issue view "${task}" --json labels --jq '.labels[].name' 2>/dev/null)" || {
  echo "Cannot read issue ${task}" >&2
  exit 20
}

has_label() {
  local name="$1"
  grep -Fxq "${name}" <<< "${labels}"
}

if ! has_label "agent"; then
  exit 11
fi

has_needs_spec=0
if has_label "needs-spec"; then
  has_needs_spec=1
fi

has_ready_for_dev=0
if has_label "ready-for-dev"; then
  has_ready_for_dev=1
fi

if [[ "${has_needs_spec}" -eq 1 && "${has_ready_for_dev}" -eq 1 ]]; then
  exit 12
fi

if [[ "${has_needs_spec}" -eq 1 ]]; then
  exit 10
fi

if [[ "${has_ready_for_dev}" -eq 1 ]]; then
  exit 0
fi

exit 13
