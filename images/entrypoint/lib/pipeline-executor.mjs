import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { spawn } from "node:child_process";

import { PIPELINE_WORKSPACE_ROOT, TARGET_WORKSPACE_DIR } from "./constants.mjs";
import { resolveClaudeArgsForVerbosity } from "./cli.mjs";
import { debugLog, sanitizeIdentifier } from "./utils.mjs";

function emitPipelineEvent(event, payload = {}) {
  console.log(
    JSON.stringify({
      type: "pipeline_event",
      event,
      ...payload,
    }),
  );
}

function applyReadOnlyPermissions(rootDir) {
  const entries = fs.readdirSync(rootDir, { withFileTypes: true });
  for (const entry of entries) {
    const entryPath = path.join(rootDir, entry.name);
    if (entry.isDirectory()) {
      applyReadOnlyPermissions(entryPath);
      fs.chmodSync(entryPath, 0o555);
      continue;
    }
    if (entry.isFile()) {
      fs.chmodSync(entryPath, 0o444);
    }
  }
  fs.chmodSync(rootDir, 0o555);
}

function prepareTaskWorkspace(planRunID, stageID, task) {
  if (task.workspace === "shared") {
    return TARGET_WORKSPACE_DIR;
  }

  const targetDir = path.join(
    PIPELINE_WORKSPACE_ROOT,
    sanitizeIdentifier(planRunID),
    sanitizeIdentifier(stageID),
    sanitizeIdentifier(task.id),
  );

  fs.rmSync(targetDir, { recursive: true, force: true });
  fs.mkdirSync(targetDir, { recursive: true });

  for (const entry of fs.readdirSync(TARGET_WORKSPACE_DIR)) {
    fs.cpSync(path.join(TARGET_WORKSPACE_DIR, entry), path.join(targetDir, entry), {
      recursive: true,
      force: true,
      dereference: false,
      preserveTimestamps: true,
    });
  }

  if (task.workspace === "snapshot_ro") {
    applyReadOnlyPermissions(targetDir);
  }

  return targetDir;
}

export function runClaudeProcess(args, options = {}) {
  const { onStdoutLine, ...spawnOptions } = options;
  const hasStdoutHook = typeof onStdoutLine === "function";

  return new Promise((resolve, reject) => {
    const child = spawn("claude", args, {
      stdio: hasStdoutHook ? ["inherit", "pipe", "pipe"] : "inherit",
      ...spawnOptions,
    });

    let stdoutBuffer = "";

    const flushStdoutBuffer = () => {
      if (!hasStdoutHook || !stdoutBuffer) {
        return;
      }
      onStdoutLine(stdoutBuffer.replace(/\r$/, ""));
      process.stdout.write(stdoutBuffer);
      stdoutBuffer = "";
    };

    if (hasStdoutHook && child.stdout) {
      child.stdout.setEncoding("utf8");
      child.stdout.on("data", (chunk) => {
        stdoutBuffer += chunk;
        while (true) {
          const lineEndIndex = stdoutBuffer.indexOf("\n");
          if (lineEndIndex === -1) {
            break;
          }

          const lineWithNewline = stdoutBuffer.slice(0, lineEndIndex + 1);
          stdoutBuffer = stdoutBuffer.slice(lineEndIndex + 1);

          const line = lineWithNewline.slice(0, -1).replace(/\r$/, "");
          onStdoutLine(line);
          process.stdout.write(lineWithNewline);
        }
      });
    }

    if (hasStdoutHook && child.stderr) {
      child.stderr.setEncoding("utf8");
      child.stderr.on("data", (chunk) => {
        process.stderr.write(chunk);
      });
    }

    child.on("error", (error) => {
      reject(new Error(`Failed to start claude: ${error.message}`));
    });

    child.on("close", (code, signal) => {
      flushStdoutBuffer();
      resolve({
        code: code ?? 1,
        signal: signal ?? "",
      });
    });
  });
}

function extractSystemInitSessionID(line) {
  const trimmed = String(line ?? "").trim();
  if (!trimmed || !trimmed.startsWith("{")) {
    return "";
  }

  try {
    const payload = JSON.parse(trimmed);
    if (payload?.type !== "system" || payload?.subtype !== "init") {
      return "";
    }
    if (typeof payload.session_id !== "string") {
      return "";
    }
    return payload.session_id.trim();
  } catch {
    return "";
  }
}

function currentTimestamp() {
  return new Date().toISOString();
}

async function executePipelineTask(planRunID, stage, task, debugEnabled) {
  const startedAt = new Date();
  const taskWorkspaceDir = prepareTaskWorkspace(planRunID, stage.id, task);

  emitPipelineEvent("task_start", {
    stage_id: stage.id,
    task_id: task.id,
    model: task.model,
    verbosity: task.verbosity,
    workspace: task.workspace,
    prompt_source: task.promptSource,
    prompt_file: task.promptFile ? task.promptFile.normalized : "",
    started_at: startedAt.toISOString(),
  });

  debugLog(
    debugEnabled,
    `Running task ${stage.id}/${task.id} (workspace=${task.workspace}, model=${task.model}, verbosity=${task.verbosity})`,
  );

  const claudeArgs = [
    "--dangerously-skip-permissions",
    "--model",
    task.model,
    ...resolveClaudeArgsForVerbosity(task.verbosity),
    "-p",
    task.promptText,
  ];

  try {
    const boundSessionIDs = new Set();
    const result = await runClaudeProcess(claudeArgs, {
      cwd: taskWorkspaceDir,
      onStdoutLine: (line) => {
        const sessionID = extractSystemInitSessionID(line);
        if (!sessionID || boundSessionIDs.has(sessionID)) {
          return;
        }
        boundSessionIDs.add(sessionID);
        emitPipelineEvent("task_session_bind", {
          stage_id: stage.id,
          task_id: task.id,
          session_id: sessionID,
        });
      },
    });
    const finishedAt = new Date();
    const isError = Boolean(result.signal) || result.code !== 0;
    const taskResult = {
      stage_id: stage.id,
      task_id: task.id,
      status: isError ? "error" : "success",
      on_error: task.onError,
      workspace: task.workspace,
      model: task.model,
      verbosity: task.verbosity,
      prompt_source: task.promptSource,
      prompt_file: task.promptFile ? task.promptFile.normalized : "",
      exit_code: result.code,
      signal: result.signal,
      started_at: startedAt.toISOString(),
      finished_at: finishedAt.toISOString(),
      duration_ms: Math.max(0, finishedAt.getTime() - startedAt.getTime()),
      error_message: isError
        ? result.signal
          ? `Task terminated by signal ${result.signal}`
          : `Task exited with code ${result.code}`
        : "",
    };

    emitPipelineEvent("task_finish", taskResult);
    return taskResult;
  } catch (error) {
    const finishedAt = new Date();
    const message = error instanceof Error ? error.message : String(error);
    const taskResult = {
      stage_id: stage.id,
      task_id: task.id,
      status: "error",
      on_error: task.onError,
      workspace: task.workspace,
      model: task.model,
      verbosity: task.verbosity,
      prompt_source: task.promptSource,
      prompt_file: task.promptFile ? task.promptFile.normalized : "",
      exit_code: -1,
      signal: "",
      started_at: startedAt.toISOString(),
      finished_at: finishedAt.toISOString(),
      duration_ms: Math.max(0, finishedAt.getTime() - startedAt.getTime()),
      error_message: message,
    };

    emitPipelineEvent("task_finish", taskResult);
    return taskResult;
  }
}

async function executeSequentialStage(planRunID, stage, debugEnabled) {
  const taskResults = [];
  let stopAfterStage = false;

  for (const task of stage.tasks) {
    const taskResult = await executePipelineTask(planRunID, stage, task, debugEnabled);
    taskResults.push(taskResult);

    if (taskResult.status === "error" && task.onError === "fail_fast") {
      stopAfterStage = true;
      break;
    }
  }

  return {
    taskResults,
    stopAfterStage,
  };
}

async function executeParallelStage(planRunID, stage, debugEnabled) {
  const taskResults = new Array(stage.tasks.length);
  let nextIndex = 0;
  let stopAfterStage = false;

  const workerCount = Math.max(1, Math.min(stage.maxParallel, stage.tasks.length));

  const worker = async () => {
    while (true) {
      if (stopAfterStage) {
        return;
      }

      const currentIndex = nextIndex;
      nextIndex += 1;
      if (currentIndex >= stage.tasks.length) {
        return;
      }

      const task = stage.tasks[currentIndex];
      const taskResult = await executePipelineTask(planRunID, stage, task, debugEnabled);
      taskResults[currentIndex] = taskResult;

      if (taskResult.status === "error" && task.onError === "fail_fast") {
        stopAfterStage = true;
      }
    }
  };

  await Promise.all(Array.from({ length: workerCount }, () => worker()));

  return {
    taskResults: taskResults.filter(Boolean),
    stopAfterStage,
  };
}

export async function executePipelinePlan(plan, debugEnabled) {
  const planStartedAt = Date.now();
  const planRunID = `${Date.now()}-${process.pid}`;
  const stageResults = [];
  const allTaskResults = [];

  emitPipelineEvent("plan_start", {
    version: plan.version,
    started_at: currentTimestamp(),
    stage_count: plan.stages.length,
  });

  let stopPipeline = false;

  for (let stageIndex = 0; stageIndex < plan.stages.length; stageIndex += 1) {
    if (stopPipeline) {
      break;
    }

    const stage = plan.stages[stageIndex];
    const stageStartedAt = Date.now();

    emitPipelineEvent("stage_start", {
      stage_id: stage.id,
      mode: stage.mode,
      started_at: currentTimestamp(),
      task_count: stage.tasks.length,
      max_parallel: stage.mode === "parallel" ? stage.maxParallel : 1,
    });

    const outcome =
      stage.mode === "parallel"
        ? await executeParallelStage(planRunID, stage, debugEnabled)
        : await executeSequentialStage(planRunID, stage, debugEnabled);

    const stageFinishedAt = Date.now();
    const failedTasks = outcome.taskResults.filter((item) => item.status === "error");

    const stageRecord = {
      stage_id: stage.id,
      mode: stage.mode,
      status: failedTasks.length > 0 ? "error" : "success",
      task_count: stage.tasks.length,
      completed_tasks: outcome.taskResults.length,
      failed_tasks: failedTasks.length,
      duration_ms: Math.max(0, stageFinishedAt - stageStartedAt),
    };

    stageResults.push(stageRecord);
    allTaskResults.push(...outcome.taskResults);

    emitPipelineEvent("stage_finish", stageRecord);

    if (outcome.stopAfterStage) {
      stopPipeline = true;
    }
  }

  const failedTaskCount = allTaskResults.filter((item) => item.status === "error").length;
  const status = failedTaskCount > 0 ? "error" : "success";
  const planFinishedAt = Date.now();

  emitPipelineEvent("plan_finish", {
    status,
    finished_at: currentTimestamp(),
    duration_ms: Math.max(0, planFinishedAt - planStartedAt),
    stage_count: plan.stages.length,
    completed_stages: stageResults.length,
    task_count: allTaskResults.length,
    failed_task_count: failedTaskCount,
  });

  const pipelineResult = {
    type: "pipeline_result",
    version: plan.version,
    status,
    is_error: failedTaskCount > 0,
    stage_count: plan.stages.length,
    completed_stages: stageResults.length,
    task_count: allTaskResults.length,
    failed_task_count: failedTaskCount,
    tasks: allTaskResults,
  };

  console.log(JSON.stringify(pipelineResult));

  return {
    exitCode: failedTaskCount > 0 ? 1 : 0,
    signal: "",
  };
}
