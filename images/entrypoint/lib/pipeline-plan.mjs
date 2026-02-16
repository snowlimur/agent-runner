import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

import {
  PIPELINE_DEFAULT_ON_ERROR,
  PIPELINE_DEFAULT_VERBOSITY,
  PIPELINE_DEFAULT_WORKSPACE,
  PIPELINE_VERSION,
  TARGET_WORKSPACE_DIR,
} from "./constants.mjs";
import {
  isPlainObject,
  parsePositiveInteger,
  requireNonEmptyString,
  runSync,
} from "./utils.mjs";

const require = createRequire(import.meta.url);
let yamlModule = null;
try {
  yamlModule = require("js-yaml");
} catch {
  yamlModule = null;
}

function normalizeModelValue(value, fieldName, fallback) {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const model = String(base).trim().toLowerCase();
  if (model === "sonnet" || model === "opus") {
    return model;
  }
  throw new Error(`${fieldName} must be one of: sonnet, opus.`);
}

function normalizeVerbosityValue(value, fieldName, fallback) {
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

function normalizeOnErrorValue(value, fieldName, fallback) {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const normalized = String(base).trim().toLowerCase();
  if (normalized === "fail_fast" || normalized === "continue") {
    return normalized;
  }
  throw new Error(`${fieldName} must be one of: fail_fast, continue.`);
}

function normalizeWorkspaceValue(value, fieldName, fallback) {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const normalized = String(base).trim().toLowerCase();
  if (normalized === "shared" || normalized === "worktree" || normalized === "snapshot_ro") {
    return normalized;
  }
  throw new Error(`${fieldName} must be one of: shared, worktree, snapshot_ro.`);
}

function normalizeOptionalBoolean(value, fieldName) {
  if (value === undefined || value === null) {
    return undefined;
  }
  if (typeof value === "boolean") {
    return value;
  }
  throw new Error(`${fieldName} must be a boolean.`);
}

export function resolvePromptFilePath(inputPath, fieldName) {
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

function parseYamlViaRuby(rawYaml) {
  const rubyScript = [
    "raw = STDIN.read",
    "data = YAML.safe_load(raw, aliases: true)",
    "puts JSON.generate(data)",
  ].join("; ");

  const encoded = runSync("ruby", ["-ryaml", "-rjson", "-e", rubyScript], {
    input: rawYaml,
  });
  return JSON.parse(encoded);
}

export function parseYamlPlan(rawYaml) {
  let parsed;
  try {
    if (yamlModule && typeof yamlModule.load === "function") {
      parsed = yamlModule.load(rawYaml);
    } else {
      parsed = parseYamlViaRuby(rawYaml);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`Failed to parse YAML plan: ${message}`);
  }

  if (!isPlainObject(parsed)) {
    throw new Error("Pipeline plan root must be a YAML mapping/object.");
  }

  return parsed;
}

export function resolvePipelineFilePath(pipelinePath) {
  const trimmed = requireNonEmptyString(pipelinePath, "--pipeline");
  if (path.isAbsolute(trimmed)) {
    return path.resolve(trimmed);
  }
  return path.resolve(TARGET_WORKSPACE_DIR, trimmed);
}

export function loadPipelinePlan(rawPlan, fallbackModel) {
  const version = requireNonEmptyString(rawPlan.version ?? "", "version");
  if (version !== PIPELINE_VERSION) {
    throw new Error(`Unsupported plan version: ${version}. Expected ${PIPELINE_VERSION}.`);
  }

  const rawDefaults = isPlainObject(rawPlan.defaults) ? rawPlan.defaults : {};
  const defaults = {
    model: normalizeModelValue(rawDefaults.model, "defaults.model", fallbackModel),
    verbosity: normalizeVerbosityValue(
      rawDefaults.verbosity,
      "defaults.verbosity",
      PIPELINE_DEFAULT_VERBOSITY,
    ),
    onError: normalizeOnErrorValue(rawDefaults.on_error, "defaults.on_error", PIPELINE_DEFAULT_ON_ERROR),
    workspace: normalizeWorkspaceValue(
      rawDefaults.workspace,
      "defaults.workspace",
      PIPELINE_DEFAULT_WORKSPACE,
    ),
  };

  if (!Array.isArray(rawPlan.stages) || rawPlan.stages.length === 0) {
    throw new Error("stages must be a non-empty array.");
  }

  const stageIDs = new Set();
  const stages = rawPlan.stages.map((rawStage, stageIndex) => {
    if (!isPlainObject(rawStage)) {
      throw new Error(`stages[${stageIndex}] must be an object.`);
    }

    const stageID = requireNonEmptyString(rawStage.id, `stages[${stageIndex}].id`);
    if (stageIDs.has(stageID)) {
      throw new Error(`stages[${stageIndex}].id is duplicated: ${stageID}`);
    }
    stageIDs.add(stageID);

    const mode = requireNonEmptyString(rawStage.mode, `stages[${stageIndex}].mode`).toLowerCase();
    if (mode !== "sequential" && mode !== "parallel") {
      throw new Error(`stages[${stageIndex}].mode must be one of: sequential, parallel.`);
    }

    if (!Array.isArray(rawStage.tasks) || rawStage.tasks.length === 0) {
      throw new Error(`stages[${stageIndex}].tasks must be a non-empty array.`);
    }

    let maxParallel = rawStage.tasks.length;
    if (mode === "parallel") {
      maxParallel = parsePositiveInteger(rawStage.max_parallel, rawStage.tasks.length);
    }

    const stageOnError = normalizeOnErrorValue(
      rawStage.on_error,
      `stages[${stageIndex}].on_error`,
      defaults.onError,
    );
    const stageWorkspace = normalizeWorkspaceValue(
      rawStage.workspace,
      `stages[${stageIndex}].workspace`,
      defaults.workspace,
    );
    const stageModel = normalizeModelValue(rawStage.model, `stages[${stageIndex}].model`, defaults.model);
    const stageVerbosity = normalizeVerbosityValue(
      rawStage.verbosity,
      `stages[${stageIndex}].verbosity`,
      defaults.verbosity,
    );

    const taskIDs = new Set();
    const tasks = rawStage.tasks.map((rawTask, taskIndex) => {
      if (!isPlainObject(rawTask)) {
        throw new Error(`stages[${stageIndex}].tasks[${taskIndex}] must be an object.`);
      }

      const taskID = requireNonEmptyString(rawTask.id, `stages[${stageIndex}].tasks[${taskIndex}].id`);
      if (taskIDs.has(taskID)) {
        throw new Error(`Duplicate task id in stage ${stageID}: ${taskID}`);
      }
      taskIDs.add(taskID);

      const promptValue = typeof rawTask.prompt === "string" ? rawTask.prompt.trim() : "";
      const promptFileValue = typeof rawTask.prompt_file === "string" ? rawTask.prompt_file.trim() : "";

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

      let promptFile = null;
      if (promptFileValue) {
        promptFile = resolvePromptFilePath(
          promptFileValue,
          `stages[${stageIndex}].tasks[${taskIndex}].prompt_file`,
        );
      }

      const taskOnError = normalizeOnErrorValue(
        rawTask.on_error,
        `stages[${stageIndex}].tasks[${taskIndex}].on_error`,
        stageOnError,
      );
      const taskWorkspace = normalizeWorkspaceValue(
        rawTask.workspace,
        `stages[${stageIndex}].tasks[${taskIndex}].workspace`,
        stageWorkspace,
      );
      const taskModel = normalizeModelValue(
        rawTask.model,
        `stages[${stageIndex}].tasks[${taskIndex}].model`,
        stageModel,
      );
      const taskVerbosity = normalizeVerbosityValue(
        rawTask.verbosity,
        `stages[${stageIndex}].tasks[${taskIndex}].verbosity`,
        stageVerbosity,
      );

      const readOnly = normalizeOptionalBoolean(
        rawTask.read_only,
        `stages[${stageIndex}].tasks[${taskIndex}].read_only`,
      );
      const allowSharedWrites = normalizeOptionalBoolean(
        rawTask.allow_shared_writes,
        `stages[${stageIndex}].tasks[${taskIndex}].allow_shared_writes`,
      );

      return {
        id: taskID,
        prompt: promptValue || null,
        promptFile,
        onError: taskOnError,
        workspace: taskWorkspace,
        model: taskModel,
        verbosity: taskVerbosity,
        readOnly: Boolean(readOnly),
        allowSharedWrites: Boolean(allowSharedWrites),
        promptSource: promptValue ? "prompt" : "prompt_file",
        promptText: null,
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
      tasks,
    };
  });

  hydrateTaskPrompts(stages);

  return {
    version,
    defaults,
    stages,
  };
}

function hydrateTaskPrompts(stages) {
  for (const stage of stages) {
    for (const task of stage.tasks) {
      if (task.promptFile) {
        let content;
        try {
          content = fs.readFileSync(task.promptFile.resolved, "utf8");
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          throw new Error(
            `Failed to read prompt_file for ${stage.id}/${task.id} (${task.promptFile.normalized}): ${message}`,
          );
        }

        const promptText = content.trim();
        if (!promptText) {
          throw new Error(`prompt_file is empty for ${stage.id}/${task.id}: ${task.promptFile.normalized}`);
        }
        task.promptText = promptText;
      } else {
        task.promptText = requireNonEmptyString(task.prompt, `task prompt for ${stage.id}/${task.id}`);
      }
    }
  }
}

export function resolvePipelinePlan(args, fallbackModel) {
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
