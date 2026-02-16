import fs from "node:fs";
import os from "node:os";
import path from "node:path";

import { SOURCE_WORKSPACE_DIR, TARGET_WORKSPACE_DIR } from "./constants.mjs";
import { debugLog, firstNonEmptyEnv, runSync } from "./utils.mjs";

function clearDirectoryContents(directoryPath) {
  const entries = fs.readdirSync(directoryPath);
  for (const entry of entries) {
    fs.rmSync(path.join(directoryPath, entry), { recursive: true, force: true });
  }
}

export function prepareWorkspaceFromReadOnlySource(debugEnabled) {
  const sourceDir = path.resolve(SOURCE_WORKSPACE_DIR);
  const targetDir = path.resolve(TARGET_WORKSPACE_DIR);

  const sourceInsideTarget = sourceDir.startsWith(`${targetDir}${path.sep}`);
  const targetInsideSource = targetDir.startsWith(`${sourceDir}${path.sep}`);
  if (sourceDir === targetDir || sourceInsideTarget || targetInsideSource) {
    throw new Error(
      `SOURCE_WORKSPACE_DIR must not overlap with ${TARGET_WORKSPACE_DIR}: ${sourceDir}`,
    );
  }

  if (!fs.existsSync(sourceDir)) {
    throw new Error(`Project source directory not found: ${sourceDir}`);
  }

  if (!fs.statSync(sourceDir).isDirectory()) {
    throw new Error(`Project source path is not a directory: ${sourceDir}`);
  }

  debugLog(debugEnabled, `Copying project to writable workspace: ${sourceDir} -> ${targetDir}`);
  fs.mkdirSync(targetDir, { recursive: true });
  clearDirectoryContents(targetDir);

  for (const entry of fs.readdirSync(sourceDir)) {
    fs.cpSync(path.join(sourceDir, entry), path.join(targetDir, entry), {
      recursive: true,
      force: true,
      dereference: false,
      preserveTimestamps: true,
    });
  }

  debugLog(debugEnabled, "Workspace is ready in /workspace.");
}

export function configureGit() {
  const gitUserName = firstNonEmptyEnv(
    ["GIT_USER_NAME", "GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"],
    "Claude Code Agent",
  );
  const gitUserEmail = firstNonEmptyEnv(
    ["GIT_USER_EMAIL", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"],
    "claude-bot@local.docker",
  );

  runSync("git", ["config", "--global", "user.name", gitUserName]);
  runSync("git", ["config", "--global", "user.email", gitUserEmail]);
  runSync("git", ["config", "--global", "--add", "safe.directory", TARGET_WORKSPACE_DIR]);
}

export function ensureGitHubAuthAndSetupGit(debugEnabled) {
  const commandOptions = debugEnabled ? { stdio: "inherit" } : {};

  debugLog(debugEnabled, "Checking GitHub CLI authentication...");
  runSync("gh", ["auth", "status"], commandOptions);
  debugLog(debugEnabled, "Configuring git credential helper via gh...");
  runSync("gh", ["auth", "setup-git"], commandOptions);
}

export function resolveUsername() {
  try {
    return os.userInfo().username;
  } catch {
    return process.env.USER || "unknown";
  }
}
