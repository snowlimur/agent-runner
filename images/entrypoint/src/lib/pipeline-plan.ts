import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";

import {
  PIPELINE_DEFAULT_AGENT_IDLE_TIMEOUT_SEC,
  PIPELINE_DEFAULT_COMMAND_TIMEOUT_SEC,
  PIPELINE_DEFAULT_MAX_ITERATIONS,
  PIPELINE_DEFAULT_MAX_SAME_NODE_HITS,
  PIPELINE_VERSION,
  TARGET_WORKSPACE_DIR,
} from "./constants.js";
import { compileCondition } from "./condition-eval.js";
import type {
  EntrypointArgs,
  JSONValue,
  Model,
  PipelineAgentRun,
  PipelineCommandRun,
  PipelineDefaults,
  PipelineExecutableNode,
  PipelineLimits,
  PipelineNode,
  PipelinePlan,
  PipelineTerminalNode,
  PipelineTerminalStatus,
  PromptFileRef,
  JSONObject,
} from "./types.js";
import { isPlainObject, parsePositiveInteger, requireNonEmptyString, runSync } from "./utils.js";

interface YamlModule {
  load: (input: string) => unknown;
}

const TEMPLATE_PLACEHOLDER_PATTERN = /\{\{([A-Z][A-Z0-9_]*)\}\}/g;

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

export class PipelinePlanError extends Error {
  readonly exitCode: number;

  constructor(message: string, exitCode = 2) {
    super(message);
    this.name = "PipelinePlanError";
    this.exitCode = exitCode;
  }
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
    throw new PipelinePlanError(`Failed to parse YAML plan: ${message}`);
  }

  if (!isPlainObject(parsed)) {
    throw new PipelinePlanError("Pipeline plan root must be a YAML mapping/object.");
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

function requireObject(value: unknown, fieldName: string): Record<string, unknown> {
  if (!isPlainObject(value)) {
    throw new PipelinePlanError(`${fieldName} must be an object.`);
  }
  return value;
}

function normalizeModelValue(value: unknown, fieldName: string, fallback: Model): Model {
  const base = value === undefined || value === null || String(value).trim() === "" ? fallback : value;
  const model = String(base).trim().toLowerCase();
  if (model === "sonnet" || model === "opus") {
    return model;
  }

  throw new PipelinePlanError(`${fieldName} must be one of: sonnet, opus.`);
}

function normalizeTerminalStatus(value: unknown, fieldName: string): PipelineTerminalStatus {
  const status = requireNonEmptyString(value, fieldName).toLowerCase();
  if (status === "success" || status === "blocked" || status === "failed" || status === "canceled") {
    return status;
  }

  throw new PipelinePlanError(`${fieldName} must be one of: success, blocked, failed, canceled.`);
}

function parseExitCode(value: unknown, fieldName: string): number {
  const raw = String(value ?? "").trim();
  if (!raw) {
    throw new PipelinePlanError(`${fieldName} must be an integer in range 0..255.`);
  }

  const parsed = Number.parseInt(raw, 10);
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 255) {
    throw new PipelinePlanError(`${fieldName} must be an integer in range 0..255.`);
  }

  return parsed;
}

function normalizePromptFilePath(inputPath: unknown, fieldName: string): PromptFileRef {
  const trimmed = requireNonEmptyString(inputPath, fieldName).replaceAll("\\", "/");
  if (path.posix.isAbsolute(trimmed)) {
    throw new PipelinePlanError(`${fieldName} must be relative to ${TARGET_WORKSPACE_DIR}: ${trimmed}`);
  }

  const normalized = path.posix.normalize(trimmed);
  if (!normalized || normalized === "." || normalized === ".." || normalized.startsWith("../")) {
    throw new PipelinePlanError(`${fieldName} must stay within ${TARGET_WORKSPACE_DIR}: ${trimmed}`);
  }

  const workspaceRoot = path.resolve(TARGET_WORKSPACE_DIR);
  const resolved = path.resolve(workspaceRoot, normalized);
  if (resolved !== workspaceRoot && !resolved.startsWith(`${workspaceRoot}${path.sep}`)) {
    throw new PipelinePlanError(`${fieldName} must stay within ${TARGET_WORKSPACE_DIR}: ${trimmed}`);
  }

  return {
    raw: trimmed,
    normalized,
    resolved,
  };
}

function resolveCommandCWD(inputPath: unknown, fieldName: string): string {
  if (inputPath === undefined || inputPath === null || String(inputPath).trim() === "") {
    return TARGET_WORKSPACE_DIR;
  }

  const raw = requireNonEmptyString(inputPath, fieldName).replaceAll("\\", "/");
  const workspaceRoot = path.resolve(TARGET_WORKSPACE_DIR);

  const resolved = path.isAbsolute(raw)
    ? path.resolve(raw)
    : path.resolve(workspaceRoot, path.posix.normalize(raw));
  if (resolved !== workspaceRoot && !resolved.startsWith(`${workspaceRoot}${path.sep}`)) {
    throw new PipelinePlanError(`${fieldName} must stay within ${TARGET_WORKSPACE_DIR}: ${raw}`);
  }

  return resolved;
}

function resolveDefaults(rawPlan: Record<string, unknown>, fallbackModel: Model): PipelineDefaults {
  const rawDefaults = isPlainObject(rawPlan.defaults) ? rawPlan.defaults : {};

  return {
    model: normalizeModelValue(rawDefaults.model, "defaults.model", fallbackModel),
    agentIdleTimeoutSec: parsePositiveInteger(
      rawDefaults.agent_idle_timeout_sec,
      PIPELINE_DEFAULT_AGENT_IDLE_TIMEOUT_SEC,
    ),
    commandTimeoutSec: parsePositiveInteger(
      rawDefaults.command_timeout_sec,
      PIPELINE_DEFAULT_COMMAND_TIMEOUT_SEC,
    ),
  };
}

function resolveLimits(rawPlan: Record<string, unknown>): PipelineLimits {
  const rawLimits = isPlainObject(rawPlan.limits) ? rawPlan.limits : {};

  return {
    maxIterations: parsePositiveInteger(rawLimits.max_iterations, PIPELINE_DEFAULT_MAX_ITERATIONS),
    maxSameNodeHits: parsePositiveInteger(rawLimits.max_same_node_hits, PIPELINE_DEFAULT_MAX_SAME_NODE_HITS),
  };
}

function applyInlinePromptTemplate(
  prompt: string,
  nodeID: string,
  templateVars: Readonly<Record<string, string>>,
  usedTemplateVars: Set<string>,
): string {
  const missingVars = new Set<string>();
  const replaced = prompt.replace(TEMPLATE_PLACEHOLDER_PATTERN, (_match, rawName: unknown) => {
    const name = String(rawName ?? "");
    usedTemplateVars.add(name);
    if (!Object.prototype.hasOwnProperty.call(templateVars, name)) {
      missingVars.add(name);
      return `{{${name}}}`;
    }
    return templateVars[name] ?? "";
  });

  if (missingVars.size > 0) {
    const missing = [...missingVars].sort().join(", ");
    throw new PipelinePlanError(`Missing template vars for ${nodeID}: ${missing}`);
  }

  return replaced;
}

function parseDecisionSchema(schemaPath: PromptFileRef): JSONObject {
  let rawSchema = "";
  try {
    rawSchema = fs.readFileSync(schemaPath.resolved, "utf8");
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    throw new PipelinePlanError(`Failed to read schema file ${schemaPath.normalized}: ${message}`);
  }

  if (!rawSchema.trim()) {
    throw new PipelinePlanError(`Schema file is empty: ${schemaPath.normalized}`);
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(rawSchema);
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    throw new PipelinePlanError(`Invalid JSON in schema file ${schemaPath.normalized}: ${message}`);
  }

  if (!isPlainObject(parsed)) {
    throw new PipelinePlanError(`Schema file must contain a JSON object: ${schemaPath.normalized}`);
  }

  return parsed as JSONObject;
}

function parseTransitions(rawTransitions: unknown, fieldName: string): PipelineExecutableNode["transitions"] {
  if (!Array.isArray(rawTransitions) || rawTransitions.length === 0) {
    throw new PipelinePlanError(`${fieldName} must be a non-empty array.`);
  }

  return rawTransitions.map((rawTransition, index) => {
    const transition = requireObject(rawTransition, `${fieldName}[${index}]`);
    const when = requireNonEmptyString(transition.when, `${fieldName}[${index}].when`);
    const to = requireNonEmptyString(transition.to, `${fieldName}[${index}].to`);

    try {
      compileCondition(when);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : String(error);
      throw new PipelinePlanError(`${fieldName}[${index}].when is invalid: ${message}`);
    }

    return {
      when,
      to,
    };
  });
}

function resolveAgentRun(
  nodeID: string,
  rawRun: Record<string, unknown>,
  defaults: PipelineDefaults,
  templateVars: Readonly<Record<string, string>>,
  usedTemplateVars: Set<string>,
): PipelineAgentRun {
  const promptValue = typeof rawRun.prompt === "string" ? rawRun.prompt.trim() : "";
  const promptFileValue = typeof rawRun.prompt_file === "string" ? rawRun.prompt_file.trim() : "";

  if (!promptValue && !promptFileValue) {
    throw new PipelinePlanError(`nodes.${nodeID}.run must contain exactly one of: prompt, prompt_file.`);
  }
  if (promptValue && promptFileValue) {
    throw new PipelinePlanError(`nodes.${nodeID}.run must contain exactly one of: prompt, prompt_file.`);
  }

  let promptText = "";
  let promptFile: PromptFileRef | null = null;
  if (promptFileValue) {
    promptFile = normalizePromptFilePath(promptFileValue, `nodes.${nodeID}.run.prompt_file`);
    let content = "";
    try {
      content = fs.readFileSync(promptFile.resolved, "utf8");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : String(error);
      throw new PipelinePlanError(`Failed to read prompt_file for ${nodeID} (${promptFile.normalized}): ${message}`);
    }
    promptText = content.trim();
  } else {
    promptText = applyInlinePromptTemplate(promptValue, nodeID, templateVars, usedTemplateVars);
  }

  if (!promptText.trim()) {
    throw new PipelinePlanError(`Prompt is empty for node ${nodeID}.`);
  }

  const decisionRaw = requireObject(rawRun.decision, `nodes.${nodeID}.run.decision`);
  const schemaFile = normalizePromptFilePath(decisionRaw.schema_file, `nodes.${nodeID}.run.decision.schema_file`);
  const schema = parseDecisionSchema(schemaFile);

  return {
    kind: "agent",
    model: normalizeModelValue(rawRun.model, `nodes.${nodeID}.run.model`, defaults.model),
    promptFile,
    promptText,
    idleTimeoutSec: parsePositiveInteger(rawRun.idle_timeout_sec, defaults.agentIdleTimeoutSec),
    decision: {
      schemaFile,
      schema,
    },
  };
}

function resolveCommandRun(nodeID: string, rawRun: Record<string, unknown>, defaults: PipelineDefaults): PipelineCommandRun {
  return {
    kind: "command",
    cmd: requireNonEmptyString(rawRun.cmd, `nodes.${nodeID}.run.cmd`),
    cwd: resolveCommandCWD(rawRun.cwd, `nodes.${nodeID}.run.cwd`),
    timeoutSec: parsePositiveInteger(rawRun.timeout_sec, defaults.commandTimeoutSec),
  };
}

function resolveNode(
  nodeID: string,
  rawNode: unknown,
  defaults: PipelineDefaults,
  templateVars: Readonly<Record<string, string>>,
  usedTemplateVars: Set<string>,
): PipelineNode {
  const node = requireObject(rawNode, `nodes.${nodeID}`);

  if (node.terminal === true) {
    if (node.run !== undefined) {
      throw new PipelinePlanError(`nodes.${nodeID}.run is forbidden for terminal node.`);
    }
    if (node.transitions !== undefined) {
      throw new PipelinePlanError(`nodes.${nodeID}.transitions is forbidden for terminal node.`);
    }

    const terminalNode: PipelineTerminalNode = {
      id: nodeID,
      terminal: true,
      terminalStatus: normalizeTerminalStatus(node.terminal_status, `nodes.${nodeID}.terminal_status`),
      exitCode: parseExitCode(node.exit_code, `nodes.${nodeID}.exit_code`),
      message: typeof node.message === "string" ? node.message.trim() : "",
    };

    return terminalNode;
  }

  if (node.terminal !== undefined && node.terminal !== false) {
    throw new PipelinePlanError(`nodes.${nodeID}.terminal must be boolean.`);
  }

  if (node.exit_code !== undefined || node.terminal_status !== undefined || node.message !== undefined) {
    throw new PipelinePlanError(`nodes.${nodeID} terminal fields are allowed only when terminal=true.`);
  }

  const rawRun = requireObject(node.run, `nodes.${nodeID}.run`);
  const kind = requireNonEmptyString(rawRun.kind, `nodes.${nodeID}.run.kind`).toLowerCase();

  const run =
    kind === "agent"
      ? resolveAgentRun(nodeID, rawRun, defaults, templateVars, usedTemplateVars)
      : kind === "command"
      ? resolveCommandRun(nodeID, rawRun, defaults)
      : (() => {
          throw new PipelinePlanError(`nodes.${nodeID}.run.kind must be one of: agent, command.`);
        })();

  return {
    id: nodeID,
    run,
    transitions: parseTransitions(node.transitions, `nodes.${nodeID}.transitions`),
  };
}

export function loadPipelinePlan(
  rawPlan: unknown,
  fallbackModel: Model,
  templateVars: Readonly<Record<string, string>>,
): PipelinePlan {
  if (!isPlainObject(rawPlan)) {
    throw new PipelinePlanError("Pipeline plan root must be a YAML mapping/object.");
  }

  const version = requireNonEmptyString(rawPlan.version ?? "", "version");
  if (version !== PIPELINE_VERSION) {
    throw new PipelinePlanError(`Unsupported plan version: ${version}. Expected ${PIPELINE_VERSION}.`);
  }

  const entryNode = requireNonEmptyString(rawPlan.entry, "entry");
  const defaults = resolveDefaults(rawPlan, fallbackModel);
  const limits = resolveLimits(rawPlan);

  const rawNodes = requireObject(rawPlan.nodes, "nodes");
  const nodeEntries = Object.entries(rawNodes);
  if (nodeEntries.length === 0) {
    throw new PipelinePlanError("nodes must define at least one node.");
  }

  const usedTemplateVars = new Set<string>();
  const nodeOrder: string[] = [];
  const nodes: Record<string, PipelineNode> = {};

  for (const [nodeID, rawNode] of nodeEntries) {
    const normalizedID = requireNonEmptyString(nodeID, "node id");
    if (nodes[normalizedID]) {
      throw new PipelinePlanError(`Duplicate node id: ${normalizedID}`);
    }
    nodeOrder.push(normalizedID);
    nodes[normalizedID] = resolveNode(normalizedID, rawNode, defaults, templateVars, usedTemplateVars);
  }

  if (!nodes[entryNode]) {
    throw new PipelinePlanError(`entry node is not defined in nodes: ${entryNode}`);
  }

  for (const node of Object.values(nodes)) {
    if ("terminal" in node && node.terminal) {
      continue;
    }
    const executableNode = node as PipelineExecutableNode;

    for (const transition of executableNode.transitions) {
      if (!nodes[transition.to]) {
        throw new PipelinePlanError(
          `nodes.${executableNode.id}.transitions has unknown target node: ${transition.to}`,
        );
      }
    }
  }

  const unusedTemplateVars = Object.keys(templateVars)
    .filter((name) => !usedTemplateVars.has(name))
    .sort();
  if (unusedTemplateVars.length > 0) {
    throw new PipelinePlanError(`Unused template vars: ${unusedTemplateVars.join(", ")}`);
  }

  return {
    version: PIPELINE_VERSION,
    entryNode,
    defaults,
    limits,
    nodeOrder,
    nodes,
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
    throw new PipelinePlanError(`Pipeline file is empty: ${resolvedPlanPath}`);
  }

  const rawPlan = parseYamlPlan(rawYaml);
  return loadPipelinePlan(rawPlan, fallbackModel, args.templateVars);
}

export function validateDecisionJSONSchema(
  schema: JSONObject,
  value: JSONValue,
  fieldName = "decision",
): string[] {
  const errors: string[] = [];

  function validate(schemaNode: JSONValue, currentValue: JSONValue, pathName: string): void {
    if (!isPlainObject(schemaNode)) {
      errors.push(`${pathName}: schema node must be an object`);
      return;
    }

    const node = schemaNode as Record<string, JSONValue>;
    const schemaType = typeof node.type === "string" ? node.type : "";
    if (schemaType) {
      if (schemaType === "object") {
        if (currentValue === null || typeof currentValue !== "object" || Array.isArray(currentValue)) {
          errors.push(`${pathName}: expected object`);
          return;
        }
      } else if (schemaType === "array") {
        if (!Array.isArray(currentValue)) {
          errors.push(`${pathName}: expected array`);
          return;
        }
      } else if (schemaType === "string") {
        if (typeof currentValue !== "string") {
          errors.push(`${pathName}: expected string`);
          return;
        }
      } else if (schemaType === "number") {
        if (typeof currentValue !== "number") {
          errors.push(`${pathName}: expected number`);
          return;
        }
      } else if (schemaType === "integer") {
        if (typeof currentValue !== "number" || !Number.isInteger(currentValue)) {
          errors.push(`${pathName}: expected integer`);
          return;
        }
      } else if (schemaType === "boolean") {
        if (typeof currentValue !== "boolean") {
          errors.push(`${pathName}: expected boolean`);
          return;
        }
      } else if (schemaType === "null") {
        if (currentValue !== null) {
          errors.push(`${pathName}: expected null`);
          return;
        }
      } else {
        errors.push(`${pathName}: unsupported schema type ${schemaType}`);
        return;
      }
    }

    if (Array.isArray(node.enum)) {
      const matched = node.enum.some((candidate) => JSON.stringify(candidate) === JSON.stringify(currentValue));
      if (!matched) {
        errors.push(`${pathName}: value is not in enum`);
        return;
      }
    }

    if (schemaType === "object") {
      const objectValue = currentValue as Record<string, JSONValue>;
      const required = Array.isArray(node.required) ? node.required : [];
      for (const requiredField of required) {
        if (typeof requiredField !== "string") {
          continue;
        }
        if (!Object.prototype.hasOwnProperty.call(objectValue, requiredField)) {
          errors.push(`${pathName}.${requiredField}: missing required field`);
        }
      }

      const properties = isPlainObject(node.properties) ? (node.properties as Record<string, JSONValue>) : {};
      for (const [propertyName, propertySchema] of Object.entries(properties)) {
        if (!Object.prototype.hasOwnProperty.call(objectValue, propertyName)) {
          continue;
        }
        validate(propertySchema, objectValue[propertyName] ?? null, `${pathName}.${propertyName}`);
      }

      if (node.additionalProperties === false) {
        const allowedFields = new Set(Object.keys(properties));
        for (const key of Object.keys(objectValue)) {
          if (!allowedFields.has(key)) {
            errors.push(`${pathName}.${key}: additional property is not allowed`);
          }
        }
      }
    }

    if (schemaType === "array") {
      const arrayValue = currentValue as JSONValue[];
      if (isPlainObject(node.items)) {
        for (let index = 0; index < arrayValue.length; index += 1) {
          validate(node.items as JSONValue, arrayValue[index] ?? null, `${pathName}[${index}]`);
        }
      }
    }
  }

  validate(schema, value, fieldName);
  return errors;
}
