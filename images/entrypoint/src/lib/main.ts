import process from "node:process";

import { resolveEntrypointArgs, resolvePromptRunOptions } from "./cli.js";
import { installDinDSignalHandlers, startDinD, stopDinD } from "./dind.js";
import { executePipelinePlan, runClaudeProcess } from "./pipeline-executor.js";
import { PipelinePlanError, resolvePipelinePlan } from "./pipeline-plan.js";
import type { ClaudeProcessResult, DinDRuntime, Model } from "./types.js";
import { debugLog, isTruthyEnv } from "./utils.js";
import {
  configureGit,
  ensureGitHubAuthAndSetupGit,
  prepareWorkspaceFromReadOnlySource,
  resolveUsername,
} from "./workspace-git.js";

async function runSinglePrompt(
  model: Model,
  taskArgs: readonly string[],
  debugEnabled: boolean,
): Promise<ClaudeProcessResult> {
  const { prompt, claudeArgs } = resolvePromptRunOptions(taskArgs);
  debugLog(debugEnabled, `Running provided task with model ${model}: ${prompt}`);

  return runClaudeProcess([
    "--dangerously-skip-permissions",
    "--model",
    model,
    ...claudeArgs,
    "-p",
    prompt,
  ]);
}

async function runInteractive(debugEnabled: boolean): Promise<ClaudeProcessResult> {
  debugLog(debugEnabled, "Starting interactive session...");
  return runClaudeProcess(["--dangerously-skip-permissions"]);
}

export async function runEntrypoint(): Promise<void> {
  const args = resolveEntrypointArgs(process.argv.slice(2));
  const { debugEnabled, model, taskArgs } = args;

  debugLog(debugEnabled, "Starting Claude Code environment...");
  debugLog(debugEnabled, `User: ${resolveUsername()}`);
  debugLog(debugEnabled, `Working directory: ${process.cwd()}`);

  prepareWorkspaceFromReadOnlySource(debugEnabled);
  configureGit(debugEnabled);
  ensureGitHubAuthAndSetupGit(debugEnabled);

  let dindRuntime: DinDRuntime | null = null;
  const stopDinDRuntime = (): void => {
    if (!dindRuntime) {
      return;
    }

    stopDinD(dindRuntime, debugEnabled);
    dindRuntime = null;
  };

  if (isTruthyEnv(process.env.ENABLE_DIND)) {
    dindRuntime = startDinD(debugEnabled);
    installDinDSignalHandlers(() => {
      stopDinDRuntime();
    }, debugEnabled);
  }

  try {
    let plan = null;
    try {
      plan = resolvePipelinePlan(args, model);
    } catch (error: unknown) {
      if (error instanceof PipelinePlanError) {
        process.stderr.write(`Entrypoint failed: ${error.message}\n`);
        process.exitCode = error.exitCode;
        return;
      }
      throw error;
    }

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

    if (Object.keys(args.templateVars).length > 0) {
      throw new Error("--var can only be used together with --pipeline.");
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
  } finally {
    stopDinDRuntime();
  }
}
