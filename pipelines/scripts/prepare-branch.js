#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");

const MAX_REASON_LENGTH = 180;

function emitSuccess() {
  process.stdout.write('{"status":"branch_ready"}\n');
}

function sanitizeReason(reason) {
  const normalized = String(reason ?? "")
    .replace(/\s+/g, " ")
    .trim();
  if (!normalized) {
    return "command failed";
  }
  return normalized.slice(0, MAX_REASON_LENGTH);
}

function emitFailure(reason) {
  process.stdout.write(
    JSON.stringify({ status: "failed", reason: sanitizeReason(reason) }) + "\n",
  );
}

function fail(reason) {
  emitFailure(reason);
  process.exit(1);
}

function runGit(args, allowFailure = false) {
  const result = spawnSync("git", args, {
    encoding: "utf8",
  });

  if (result.error) {
    throw new Error(result.error.message);
  }

  const exitCode = typeof result.status === "number" ? result.status : 1;
  const stdout = typeof result.stdout === "string" ? result.stdout.trim() : "";
  const stderr = typeof result.stderr === "string" ? result.stderr.trim() : "";

  if (exitCode !== 0 && !allowFailure) {
    const command = `git ${args.join(" ")}`;
    const details = stderr || stdout || "unknown error";
    throw new Error(`${command}: ${details}`);
  }

  return {
    exitCode,
    stdout,
    stderr,
  };
}

function hasLocalBranch(branchName) {
  return (
    runGit(
      ["show-ref", "--verify", "--quiet", `refs/heads/${branchName}`],
      true,
    ).exitCode === 0
  );
}

function hasUpstream() {
  return (
    runGit(["rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"], true)
      .exitCode === 0
  );
}

function hasRemoteBranch(branchName) {
  return (
    runGit(
      ["show-ref", "--verify", "--quiet", `refs/remotes/origin/${branchName}`],
      true,
    ).exitCode === 0
  );
}

function hasWorktreeChanges() {
  return runGit(["status", "--porcelain"]).stdout.length > 0;
}

function convertGitHubSSHToHTTPS(remoteURL) {
  const normalized = String(remoteURL ?? "").trim();
  if (!normalized) {
    return "";
  }

  const scpLikeMatch = normalized.match(/^git@github\.com:(.+)$/);
  if (scpLikeMatch) {
    return `https://github.com/${scpLikeMatch[1]}`;
  }

  const sshLikeMatch = normalized.match(
    /^ssh:\/\/git@github\.com[:/]([^/].*)$/,
  );
  if (sshLikeMatch) {
    return `https://github.com/${sshLikeMatch[1]}`;
  }

  return normalized;
}

function ensureHTTPSOrigin() {
  const originURLResult = runGit(["remote", "get-url", "origin"], true);
  if (originURLResult.exitCode !== 0 || !originURLResult.stdout) {
    return;
  }

  const currentURL = originURLResult.stdout;
  const httpsURL = convertGitHubSSHToHTTPS(currentURL);
  if (httpsURL && httpsURL !== currentURL) {
    runGit(["remote", "set-url", "origin", httpsURL]);
  }
}

function main() {
  const issueId = String(process.argv[2] ?? "").trim();
  if (!/^[0-9]+$/.test(issueId)) {
    fail("invalid issue id");
  }

  const issueBranch = `issue/${issueId}`;

  try {
    ensureHTTPSOrigin();
    runGit(["fetch", "--prune", "origin"]);

    if (hasWorktreeChanges()) {
      runGit(["stash", "push", "-m", `prepare-branch:${issueId}:auto-stash`]);
    }

    if (hasLocalBranch(issueBranch)) {
      runGit(["checkout", issueBranch]);
      if (hasRemoteBranch(issueBranch)) {
        runGit(["pull", "--ff-only", "origin", issueBranch]);
      } else if (hasUpstream()) {
        runGit(["pull", "--ff-only"]);
      }
      emitSuccess();
      process.exit(0);
    }

    if (hasRemoteBranch(issueBranch)) {
      runGit(["checkout", "-B", issueBranch, `origin/${issueBranch}`]);
      emitSuccess();
      process.exit(0);
    }

    if (hasLocalBranch("main")) {
      runGit(["checkout", "main"]);
      runGit(["pull", "--ff-only", "origin", "main"]);
    } else {
      fail("main branch does't exist");
    }

    runGit(["checkout", "-b", issueBranch]);
    emitSuccess();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    fail(message);
  }
}

main();
