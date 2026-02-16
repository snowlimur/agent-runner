import process from "node:process";

import { resolveEntrypointArgs, resolvePromptRunOptions } from "./cli.mjs";
import { executePipelinePlan, runClaudeProcess } from "./pipeline-executor.mjs";
import { resolvePipelinePlan } from "./pipeline-plan.mjs";
import {
  configureGit,
  ensureGitHubAuthAndSetupGit,
  prepareWorkspaceFromReadOnlySource,
  resolveUsername,
} from "./workspace-git.mjs";
import { installDinDSignalHandlers, startDinD, stopDinD } from "./dind.mjs";
import { debugLog, isTruthyEnv } from "./utils.mjs";

async function runSinglePrompt(model, taskArgs, debugEnabled) {
  const { prompt, claudeArgs } = resolvePromptRunOptions(taskArgs);
  debugLog(debugEnabled, `Running provided task with model ${model}: ${prompt}`);

  const result = await runClaudeProcess([
    "--dangerously-skip-permissions",
    "--model",
    model,
    ...claudeArgs,
    "-p",
    prompt,
  ]);

  return result;
}

async function runInteractive(debugEnabled) {
  debugLog(debugEnabled, "Starting interactive session...");
  return runClaudeProcess(["--dangerously-skip-permissions"]);
}

export async function runEntrypoint() {
  const args = resolveEntrypointArgs(process.argv.slice(2));
  const { debugEnabled, model, taskArgs } = args;

  debugLog(debugEnabled, "Starting Claude Code environment...");
  debugLog(debugEnabled, `User: ${resolveUsername()}`);
  debugLog(debugEnabled, `Working directory: ${process.cwd()}`);

  prepareWorkspaceFromReadOnlySource(debugEnabled);
  ensureGitHubAuthAndSetupGit(debugEnabled);
  configureGit();

  let dindRuntime = null;
  if (isTruthyEnv(process.env.ENABLE_DIND)) {
    dindRuntime = startDinD(debugEnabled);
    installDinDSignalHandlers(() => {
      stopDinD(dindRuntime, debugEnabled);
      dindRuntime = null;
    }, debugEnabled);
  }

  const plan = resolvePipelinePlan(args, model);
  if (plan !== null) {
    if (taskArgs.length > 0) {
      throw new Error("Prompt task arguments cannot be used together with pipeline mode.");
    }

    const pipelineResult = await executePipelinePlan(plan, debugEnabled);
    if (pipelineResult.signal) {
      process.kill(process.pid, pipelineResult.signal);
      return;
    }
    process.exitCode = pipelineResult.exitCode;
    return;
  }

  if (taskArgs.length > 0) {
    const result = await runSinglePrompt(model, taskArgs, debugEnabled);
    if (result.signal) {
      process.kill(process.pid, result.signal);
      return;
    }
    process.exitCode = result.code;
    return;
  }

  const interactiveResult = await runInteractive(debugEnabled);
  if (interactiveResult.signal) {
    process.kill(process.pid, interactiveResult.signal);
    return;
  }
  process.exitCode = interactiveResult.code;
}
