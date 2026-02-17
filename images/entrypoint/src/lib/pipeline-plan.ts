import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";

import {
  PIPELINE_DEFAULT_ON_ERROR,
  PIPELINE_DEFAULT_TASK_IDLE_TIMEOUT_SEC,
  PIPELINE_DEFAULT_VERBOSITY,
  PIPELINE_DEFAULT_WORKSPACE,
  PIPELINE_VERSION,
  TARGET_WORKSPACE_DIR,
} from "./constants.js";
import type {
  EntrypointArgs,
  Model,
  OnErrorPolicy,
  PipelineDefaults,
  PipelinePlan,
  PipelineTask,
  PromptFileRef,
  StageMode,
  Verbosity,
  WorkspaceMode,
} from "./types.js";
import { isPlainObject, parsePositiveInteger, requireNonEmptyString, runSync } from "./utils.js";

interface YamlModule {
  load: (input: string) => unknown;
}

const require = createRequire(import.meta.url);
let yamlModule: YamlModule | null = null;
try {
  const loaded = require("js-yaml") as { load?: unknown };
  if (typeof loaded.load === "function") {
    yamlModule = {
      load: loaded.load as (input: string) => unknown,
    };
  }
} catch {
  yamlModule = null;
}

function normalizeModelValue(value: unknown, fieldName: string, fallback: Model): Model {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const model = String(base).trim().toLowerCase();
  if (model === "sonnet" || model === "opus") {
    return model;
  }

  throw new Error(`${fieldName} must be one of: sonnet, opus.`);
}

function normalizeVerbosityValue(value: unknown, fieldName: string, fallback: Verbosity): Verbosity {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const normalized = String(base).trim().toLowerCase();
  if (normalized === "text") {
    return "text";
  }
  if (normalized === "v" || normalized === "json") {
    return "v";
  }
  if (normalized === "vv" || normalized === "stream-json" || normalized === "stream_json") {
    return "vv";
  }

  throw new Error(`${fieldName} must be one of: text, v, vv.`);
}

function normalizeOnErrorValue(value: unknown, fieldName: string, fallback: OnErrorPolicy): OnErrorPolicy {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const normalized = String(base).trim().toLowerCase();
  if (normalized === "fail_fast" || normalized === "continue") {
    return normalized;
  }

  throw new Error(`${fieldName} must be one of: fail_fast, continue.`);
}

function normalizeWorkspaceValue(
  value: unknown,
  fieldName: string,
  fallback: WorkspaceMode,
): WorkspaceMode {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const normalized = String(base).trim().toLowerCase();
  if (normalized === "shared" || normalized === "worktree" || normalized === "snapshot_ro") {
    return normalized;
  }

  throw new Error(`${fieldName} must be one of: shared, worktree, snapshot_ro.`);
}

function normalizeOptionalBoolean(value: unknown, fieldName: string): boolean | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  if (typeof value === "boolean") {
    return value;
  }

  throw new Error(`${fieldName} must be a boolean.`);
}

function requireObject(value: unknown, fieldName: string): Record<string, unknown> {
  if (!isPlainObject(value)) {
    throw new Error(`${fieldName} must be an object.`);
  }
  return value;
}

export function resolvePromptFilePath(inputPath: unknown, fieldName: string): PromptFileRef {
  const trimmed = requireNonEmptyString(inputPath, fieldName).replaceAll("\\", "/");
  if (path.posix.isAbsolute(trimmed)) {
    throw new Error(`${fieldName} must be relative to ${TARGET_WORKSPACE_DIR}: ${trimmed}`);
  }

  const normalized = path.posix.normalize(trimmed);
  if (!normalized || normalized === "." || normalized === ".." || normalized.startsWith("../")) {
    throw new Error(`${fieldName} must stay within ${TARGET_WORKSPACE_DIR}: ${trimmed}`);
  }

  const workspaceRoot = path.resolve(TARGET_WORKSPACE_DIR);
  const resolved = path.resolve(workspaceRoot, normalized);
  if (resolved !== workspaceRoot && !resolved.startsWith(`${workspaceRoot}${path.sep}`)) {
    throw new Error(`${fieldName} must stay within ${TARGET_WORKSPACE_DIR}: ${trimmed}`);
  }

  return {
    raw: trimmed,
    normalized,
    resolved,
  };
}

function parseYamlViaRuby(rawYaml: string): unknown {
  const rubyScript = [
    "raw = STDIN.read",
    "data = YAML.safe_load(raw, aliases: true)",
    "puts JSON.generate(data)",
  ].join("; ");

  const encoded = runSync("ruby", ["-ryaml", "-rjson", "-e", rubyScript], {
    input: rawYaml,
  });
  return JSON.parse(encoded) as unknown;
}

export function parseYamlPlan(rawYaml: string): Record<string, unknown> {
  let parsed: unknown;
  try {
    if (yamlModule && typeof yamlModule.load === "function") {
      parsed = yamlModule.load(rawYaml);
    } else {
      parsed = parseYamlViaRuby(rawYaml);
    }
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`Failed to parse YAML plan: ${message}`);
  }

  if (!isPlainObject(parsed)) {
    throw new Error("Pipeline plan root must be a YAML mapping/object.");
  }

  return parsed;
}

export function resolvePipelineFilePath(pipelinePath: unknown): string {
  const trimmed = requireNonEmptyString(pipelinePath, "--pipeline");
  if (path.isAbsolute(trimmed)) {
    return path.resolve(trimmed);
  }
  return path.resolve(TARGET_WORKSPACE_DIR, trimmed);
}

function resolveDefaults(rawPlan: Record<string, unknown>, fallbackModel: Model): PipelineDefaults {
  const rawDefaults = isPlainObject(rawPlan.defaults) ? rawPlan.defaults : {};

  return {
    model: normalizeModelValue(rawDefaults.model, "defaults.model", fallbackModel),
    verbosity: normalizeVerbosityValue(rawDefaults.verbosity, "defaults.verbosity", PIPELINE_DEFAULT_VERBOSITY),
    onError: normalizeOnErrorValue(rawDefaults.on_error, "defaults.on_error", PIPELINE_DEFAULT_ON_ERROR),
    workspace: normalizeWorkspaceValue(rawDefaults.workspace, "defaults.workspace", PIPELINE_DEFAULT_WORKSPACE),
    taskIdleTimeoutSec: parsePositiveInteger(
      rawDefaults.task_idle_timeout_sec,
      PIPELINE_DEFAULT_TASK_IDLE_TIMEOUT_SEC,
    ),
  };
}

export function loadPipelinePlan(rawPlan: unknown, fallbackModel: Model): PipelinePlan {
  if (!isPlainObject(rawPlan)) {
    throw new Error("Pipeline plan root must be a YAML mapping/object.");
  }

  const version = requireNonEmptyString(rawPlan.version ?? "", "version");
  if (version !== PIPELINE_VERSION) {
    throw new Error(`Unsupported plan version: ${version}. Expected ${PIPELINE_VERSION}.`);
  }

  const defaults = resolveDefaults(rawPlan, fallbackModel);

  if (!Array.isArray(rawPlan.stages) || rawPlan.stages.length === 0) {
    throw new Error("stages must be a non-empty array.");
  }

  const stageIDs = new Set<string>();
  const stages: PipelinePlan["stages"] = rawPlan.stages.map((rawStage, stageIndex) => {
    const stage = requireObject(rawStage, `stages[${stageIndex}]`);

    const stageID = requireNonEmptyString(stage.id, `stages[${stageIndex}].id`);
    if (stageIDs.has(stageID)) {
      throw new Error(`stages[${stageIndex}].id is duplicated: ${stageID}`);
    }
    stageIDs.add(stageID);

    const modeValue = requireNonEmptyString(stage.mode, `stages[${stageIndex}].mode`).toLowerCase();
    if (modeValue !== "sequential" && modeValue !== "parallel") {
      throw new Error(`stages[${stageIndex}].mode must be one of: sequential, parallel.`);
    }

    const rawTasks = stage.tasks;
    if (!Array.isArray(rawTasks) || rawTasks.length === 0) {
      throw new Error(`stages[${stageIndex}].tasks must be a non-empty array.`);
    }

    const mode: StageMode = modeValue;
    let maxParallel = rawTasks.length;
    if (mode === "parallel") {
      maxParallel = parsePositiveInteger(stage.max_parallel, rawTasks.length);
    }

    const stageOnError = normalizeOnErrorValue(stage.on_error, `stages[${stageIndex}].on_error`, defaults.onError);
    const stageWorkspace = normalizeWorkspaceValue(
      stage.workspace,
      `stages[${stageIndex}].workspace`,
      defaults.workspace,
    );
    const stageModel = normalizeModelValue(stage.model, `stages[${stageIndex}].model`, defaults.model);
    const stageVerbosity = normalizeVerbosityValue(
      stage.verbosity,
      `stages[${stageIndex}].verbosity`,
      defaults.verbosity,
    );
    const stageTaskIdleTimeoutSec = parsePositiveInteger(
      stage.task_idle_timeout_sec,
      defaults.taskIdleTimeoutSec,
    );

    const taskIDs = new Set<string>();
    const tasks: PipelineTask[] = rawTasks.map((rawTask, taskIndex) => {
      const task = requireObject(rawTask, `stages[${stageIndex}].tasks[${taskIndex}]`);

      const taskID = requireNonEmptyString(task.id, `stages[${stageIndex}].tasks[${taskIndex}].id`);
      if (taskIDs.has(taskID)) {
        throw new Error(`Duplicate task id in stage ${stageID}: ${taskID}`);
      }
      taskIDs.add(taskID);

      const promptValue = typeof task.prompt === "string" ? task.prompt.trim() : "";
      const promptFileValue = typeof task.prompt_file === "string" ? task.prompt_file.trim() : "";

      if (!promptValue && !promptFileValue) {
        throw new Error(
          `stages[${stageIndex}].tasks[${taskIndex}] must contain exactly one of: prompt, prompt_file.`,
        );
      }
      if (promptValue && promptFileValue) {
        throw new Error(
          `stages[${stageIndex}].tasks[${taskIndex}] must contain exactly one of: prompt, prompt_file.`,
        );
      }

      let promptFile: PromptFileRef | null = null;
      let promptText = "";
      if (promptFileValue) {
        promptFile = resolvePromptFilePath(
          promptFileValue,
          `stages[${stageIndex}].tasks[${taskIndex}].prompt_file`,
        );

        let content: string;
        try {
          content = fs.readFileSync(promptFile.resolved, "utf8");
        } catch (error: unknown) {
          const message = error instanceof Error ? error.message : String(error);
          throw new Error(
            `Failed to read prompt_file for ${stageID}/${taskID} (${promptFile.normalized}): ${message}`,
          );
        }

        promptText = content.trim();
        if (!promptText) {
          throw new Error(`prompt_file is empty for ${stageID}/${taskID}: ${promptFile.normalized}`);
        }
      } else {
        promptText = requireNonEmptyString(promptValue, `task prompt for ${stageID}/${taskID}`);
      }

      const taskOnError = normalizeOnErrorValue(
        task.on_error,
        `stages[${stageIndex}].tasks[${taskIndex}].on_error`,
        stageOnError,
      );
      const taskWorkspace = normalizeWorkspaceValue(
        task.workspace,
        `stages[${stageIndex}].tasks[${taskIndex}].workspace`,
        stageWorkspace,
      );
      const taskModel = normalizeModelValue(task.model, `stages[${stageIndex}].tasks[${taskIndex}].model`, stageModel);
      const taskVerbosity = normalizeVerbosityValue(
        task.verbosity,
        `stages[${stageIndex}].tasks[${taskIndex}].verbosity`,
        stageVerbosity,
      );

      const readOnly = normalizeOptionalBoolean(task.read_only, `stages[${stageIndex}].tasks[${taskIndex}].read_only`);
      const allowSharedWrites = normalizeOptionalBoolean(
        task.allow_shared_writes,
        `stages[${stageIndex}].tasks[${taskIndex}].allow_shared_writes`,
      );
      const taskIdleTimeoutSec = parsePositiveInteger(
        task.task_idle_timeout_sec,
        stageTaskIdleTimeoutSec,
      );

      return {
        id: taskID,
        promptFile,
        onError: taskOnError,
        workspace: taskWorkspace,
        model: taskModel,
        verbosity: taskVerbosity,
        readOnly: Boolean(readOnly),
        allowSharedWrites: Boolean(allowSharedWrites),
        promptText,
        taskIdleTimeoutSec,
      };
    });

    if (mode === "parallel") {
      for (const task of tasks) {
        if (task.workspace === "shared" && !task.readOnly && !task.allowSharedWrites) {
          throw new Error(
            `Parallel task ${stageID}/${task.id} uses shared workspace with writes. Set read_only=true or allow_shared_writes=true.`,
          );
        }
      }
    }

    return {
      id: stageID,
      mode,
      maxParallel,
      onError: stageOnError,
      workspace: stageWorkspace,
      model: stageModel,
      verbosity: stageVerbosity,
      taskIdleTimeoutSec: stageTaskIdleTimeoutSec,
      tasks,
    };
  });

  return {
    version: PIPELINE_VERSION,
    defaults,
    stages,
  };
}

export function resolvePipelinePlan(args: EntrypointArgs, fallbackModel: Model): PipelinePlan | null {
  const usingPipelineFile = typeof args.pipelinePath === "string" && args.pipelinePath.trim() !== "";
  if (!usingPipelineFile) {
    return null;
  }

  const resolvedPlanPath = resolvePipelineFilePath(args.pipelinePath);
  const rawYaml = fs.readFileSync(resolvedPlanPath, "utf8");
  if (!rawYaml.trim()) {
    throw new Error(`Pipeline file is empty: ${resolvedPlanPath}`);
  }

  const rawPlan = parseYamlPlan(rawYaml);
  return loadPipelinePlan(rawPlan, fallbackModel);
}
