#!/usr/bin/env node
"use strict";

import { spawnSync } from "node:child_process";

const EXIT_CODE_OK = 0;
const EXIT_CODE_MISSING_AGENT = 11;
const EXIT_CODE_NOT_READY = 13;
const EXIT_CODE_ERROR = 20;

function runCommand(command, args, allowFailure = false) {
  const result = spawnSync(command, args, { encoding: "utf8" });
  if (result.error) {
    throw new Error(result.error.message);
  }

  const exitCode = typeof result.status === "number" ? result.status : 1;
  const stdout = typeof result.stdout === "string" ? result.stdout.trim() : "";
  const stderr = typeof result.stderr === "string" ? result.stderr.trim() : "";

  if (exitCode !== 0 && !allowFailure) {
    throw new Error(stderr || stdout || `${command} failed`);
  }

  return { exitCode, stdout, stderr };
}

function fail(message) {
  process.stderr.write(`${String(message).trim()}\n`);
  process.exit(EXIT_CODE_ERROR);
}

function resolveIssueIDFromBranch() {
  const branchResult = runCommand(
    "git",
    ["rev-parse", "--abbrev-ref", "HEAD"],
    true,
  );
  if (branchResult.exitCode !== 0 || !branchResult.stdout) {
    fail("Cannot determine current branch");
  }

  const branch = branchResult.stdout;
  if (!branch.startsWith("issue/")) {
    fail(`Expected branch issue/<TASK>, got: ${branch}`);
  }

  const issueID = branch.slice("issue/".length);
  if (!/^[0-9]+$/.test(issueID)) {
    fail(`Invalid issue id parsed from branch: ${issueID}`);
  }

  return issueID;
}

function resolveIssueLabels(issueID) {
  const issueResult = runCommand(
    "gh",
    ["issue", "view", issueID, "--json", "labels"],
    true,
  );
  if (issueResult.exitCode !== 0) {
    fail(`Cannot read issue ${issueID}`);
  }

  let payload = {};
  try {
    payload = JSON.parse(issueResult.stdout || "{}");
  } catch {
    fail(`Cannot read issue ${issueID}`);
  }

  if (!Array.isArray(payload.labels)) {
    return new Set();
  }

  const labels = payload.labels
    .map((label) =>
      label && typeof label.name === "string" ? label.name.trim() : "",
    )
    .filter((name) => name.length > 0);

  return new Set(labels);
}

function main() {
  try {
    const issueID = resolveIssueIDFromBranch();
    const labels = resolveIssueLabels(issueID);

    if (!labels.has("agent")) {
      process.exit(EXIT_CODE_MISSING_AGENT);
    }

    const hasReadyForDev = labels.has("ready-for-dev");
    if (hasReadyForDev) {
      process.exit(EXIT_CODE_OK);
    }

    process.exit(EXIT_CODE_NOT_READY);
  } catch (error) {
    fail(error instanceof Error ? error.message : String(error));
  }
}

main();
