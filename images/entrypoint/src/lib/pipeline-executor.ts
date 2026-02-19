import process from "node:process";
import { spawn } from "node:child_process";

import { evaluateCondition, compileCondition } from "./condition-eval.js";
import { validateDecisionJSONSchema } from "./pipeline-plan.js";
import type {
  ClaudeProcessResult,
  CommandProcessResult,
  JSONValue,
  JSONObject,
  PipelineAgentRun,
  PipelineConditionScope,
  PipelineEventName,
  PipelineEventPayloadMap,
  PipelineExecutionResult,
  PipelineExecutableNode,
  PipelineNode,
  PipelineNodeRunRecord,
  PipelineNodeRunStatus,
  PipelinePlan,
  PipelineResult,
  RunClaudeProcessOptions,
  RunCommandProcessOptions,
} from "./types.js";
import { isPlainObject } from "./utils.js";

const SYSTEM_ERROR_INVALID_PLAN = 2;
const SYSTEM_ERROR_NO_TRANSITION = 3;
const SYSTEM_ERROR_LIMIT_REACHED = 4;
const SYSTEM_ERROR_NODE_EXECUTION = 5;

function emitPipelineEvent<TEvent extends PipelineEventName>(
  event: TEvent,
  payload: PipelineEventPayloadMap[TEvent],
): void {
  console.log(
    JSON.stringify({
      type: "pipeline_event",
      event,
      ...payload,
    }),
  );
}

function currentTimestamp(): string {
  return new Date().toISOString();
}

function extractSystemInitSessionID(line: string): string {
  const trimmed = String(line ?? "").trim();
  if (!trimmed || !trimmed.startsWith("{")) {
    return "";
  }

  try {
    const payload = JSON.parse(trimmed) as unknown;
    if (!isPlainObject(payload)) {
      return "";
    }
    if (payload.type !== "system" || payload.subtype !== "init") {
      return "";
    }

    const sessionID = payload.session_id;
    if (typeof sessionID !== "string") {
      return "";
    }

    return sessionID.trim();
  } catch {
    return "";
  }
}

interface ClaudeResultEvent {
  type: string;
  session_id?: string;
  result?: unknown;
}

function extractFinalResultEvent(line: string): ClaudeResultEvent | null {
  const trimmed = String(line ?? "").trim();
  if (!trimmed || !trimmed.startsWith("{")) {
    return null;
  }

  try {
    const payload = JSON.parse(trimmed) as unknown;
    if (!isPlainObject(payload)) {
      return null;
    }
    if (payload.type !== "result") {
      return null;
    }

    return {
      type: "result",
      session_id: typeof payload.session_id === "string" ? payload.session_id : "",
      result: payload.result,
    };
  } catch {
    return null;
  }
}

export function runClaudeProcess(
  args: readonly string[],
  options: RunClaudeProcessOptions = {},
): Promise<ClaudeProcessResult> {
  const { onStdoutLine, timeoutMs, onIdleTimeout, ...spawnOptions } = options;
  const stdoutHook = typeof onStdoutLine === "function" ? onStdoutLine : null;
  const hasStdoutHook = stdoutHook !== null;
  const idleTimeoutMs = Number.isFinite(timeoutMs) && Number(timeoutMs) > 0 ? Math.trunc(Number(timeoutMs)) : 0;

  return new Promise<ClaudeProcessResult>((resolve, reject) => {
    const child = spawn("claude", [...args], {
      stdio: ["inherit", "pipe", "pipe"],
      ...spawnOptions,
    });

    let stdoutBuffer = "";
    let timedOut = false;
    let closed = false;
    let idleTimer: NodeJS.Timeout | null = null;
    let forceKillTimer: NodeJS.Timeout | null = null;

    const clearTimers = (): void => {
      if (idleTimer !== null) {
        clearTimeout(idleTimer);
        idleTimer = null;
      }
      if (forceKillTimer !== null) {
        clearTimeout(forceKillTimer);
        forceKillTimer = null;
      }
    };

    const scheduleIdleTimeout = (): void => {
      if (idleTimeoutMs <= 0 || closed) {
        return;
      }

      if (idleTimer !== null) {
        clearTimeout(idleTimer);
      }

      idleTimer = setTimeout(() => {
        if (closed) {
          return;
        }

        timedOut = true;
        onIdleTimeout?.(idleTimeoutMs);
        child.kill("SIGTERM");

        forceKillTimer = setTimeout(() => {
          if (closed) {
            return;
          }
          child.kill("SIGKILL");
        }, 10_000);
      }, idleTimeoutMs);
    };

    const markActivity = (): void => {
      scheduleIdleTimeout();
    };

    const flushStdoutBuffer = (): void => {
      if (!stdoutBuffer) {
        return;
      }
      const flushed = stdoutBuffer;
      stdoutBuffer = "";
      stdoutHook?.(flushed.replace(/\r$/, ""));
      process.stdout.write(flushed);
    };

    if (child.stdout) {
      child.stdout.setEncoding("utf8");
      child.stdout.on("data", (chunk: string) => {
        markActivity();
        if (!hasStdoutHook) {
          process.stdout.write(chunk);
          return;
        }

        stdoutBuffer += chunk;
        while (true) {
          const lineEndIndex = stdoutBuffer.indexOf("\n");
          if (lineEndIndex === -1) {
            break;
          }

          const lineWithNewline = stdoutBuffer.slice(0, lineEndIndex + 1);
          stdoutBuffer = stdoutBuffer.slice(lineEndIndex + 1);
          const line = lineWithNewline.slice(0, -1).replace(/\r$/, "");
          stdoutHook(line);
          process.stdout.write(lineWithNewline);
        }
      });
    }

    if (child.stderr) {
      child.stderr.setEncoding("utf8");
      child.stderr.on("data", (chunk: string) => {
        markActivity();
        process.stderr.write(chunk);
      });
    }

    scheduleIdleTimeout();

    child.on("error", (error: Error) => {
      closed = true;
      clearTimers();
      reject(new Error(`Failed to start claude: ${error.message}`));
    });

    child.on("close", (code, signal) => {
      closed = true;
      clearTimers();
      flushStdoutBuffer();
      const timeoutMessage = timedOut
        ? `idle timeout after ${Math.max(1, Math.floor(idleTimeoutMs / 1000))} seconds without task output`
        : "";
      resolve({
        code: code ?? (timedOut ? 124 : 1),
        signal: signal ?? "",
        timedOut,
        timeoutMessage,
      });
    });
  });
}

function runCommandProcess(
  cmd: string,
  options: RunCommandProcessOptions = {},
): Promise<CommandProcessResult> {
  const { timeoutMs, onTimeout, ...spawnOptions } = options;
  const limitMs = Number.isFinite(timeoutMs) && Number(timeoutMs) > 0 ? Math.trunc(Number(timeoutMs)) : 0;

  return new Promise<CommandProcessResult>((resolve, reject) => {
    const child = spawn("sh", ["-lc", cmd], {
      stdio: ["inherit", "pipe", "pipe"],
      ...spawnOptions,
    });

    let timedOut = false;
    let closed = false;
    let timeoutHandle: NodeJS.Timeout | null = null;
    let forceKillHandle: NodeJS.Timeout | null = null;

    const clearTimers = (): void => {
      if (timeoutHandle !== null) {
        clearTimeout(timeoutHandle);
        timeoutHandle = null;
      }
      if (forceKillHandle !== null) {
        clearTimeout(forceKillHandle);
        forceKillHandle = null;
      }
    };

    if (child.stdout) {
      child.stdout.setEncoding("utf8");
      child.stdout.on("data", (chunk: string) => {
        process.stdout.write(chunk);
      });
    }

    if (child.stderr) {
      child.stderr.setEncoding("utf8");
      child.stderr.on("data", (chunk: string) => {
        process.stderr.write(chunk);
      });
    }

    if (limitMs > 0) {
      timeoutHandle = setTimeout(() => {
        if (closed) {
          return;
        }

        timedOut = true;
        onTimeout?.(limitMs);
        child.kill("SIGTERM");
        forceKillHandle = setTimeout(() => {
          if (closed) {
            return;
          }
          child.kill("SIGKILL");
        }, 10_000);
      }, limitMs);
    }

    child.on("error", (error: Error) => {
      closed = true;
      clearTimers();
      reject(new Error(`Failed to start command: ${error.message}`));
    });

    child.on("close", (code, signal) => {
      closed = true;
      clearTimers();
      const timeoutMessage = timedOut
        ? `timeout after ${Math.max(1, Math.floor(limitMs / 1000))} seconds`
        : "";
      resolve({
        code: code ?? (timedOut ? 124 : 1),
        signal: signal ?? "",
        timedOut,
        timeoutMessage,
      });
    });
  });
}

function makeNodeRunID(nodeID: string, sequence: number): string {
  return `${nodeID}-${sequence}`;
}

interface ExecutedNodeRun {
  record: PipelineNodeRunRecord;
  decision: JSONObject;
}

function toDecisionScope(record: PipelineNodeRunRecord, decision: JSONObject, attempt: number, iteration: number, totalNodeRuns: number): PipelineConditionScope {
  return {
    decision,
    run: {
      exit_code: record.exit_code,
      signal: record.signal,
      timed_out: record.timed_out,
      status: record.status,
    },
    node: {
      id: record.node_id,
      kind: record.kind,
      attempt,
      run_id: record.node_run_id,
    },
    pipeline: {
      iteration,
      total_node_runs: totalNodeRuns,
    },
  };
}

function executeTerminalResult(
  plan: PipelinePlan,
  nodeRuns: PipelineNodeRunRecord[],
  startedAtMs: number,
  node: PipelineNode,
  iterations: number,
): PipelineExecutionResult {
  if (!("terminal" in node) || !node.terminal) {
    return {
      exitCode: SYSTEM_ERROR_NODE_EXECUTION,
      signal: "",
    };
  }

  const failedNodeCount = nodeRuns.filter((item) => item.status === "error").length;
  const status: PipelineNodeRunStatus = node.exitCode === 0 ? "success" : "error";
  const result: PipelineResult = {
    type: "pipeline_result",
    version: plan.version,
    status,
    is_error: node.exitCode !== 0,
    entry_node: plan.entryNode,
    terminal_node: node.id,
    terminal_status: node.terminalStatus,
    exit_code: node.exitCode,
    iterations,
    node_run_count: nodeRuns.length,
    failed_node_count: failedNodeCount,
    node_runs: nodeRuns,
  };

  emitPipelineEvent("plan_finish", {
    status,
    finished_at: currentTimestamp(),
    duration_ms: Math.max(0, Date.now() - startedAtMs),
    iterations,
    node_run_count: nodeRuns.length,
    failed_node_count: failedNodeCount,
    terminal_node: node.id,
    terminal_status: node.terminalStatus,
    exit_code: node.exitCode,
  });
  console.log(JSON.stringify(result));

  return {
    exitCode: node.exitCode,
    signal: "",
  };
}

function executeSystemErrorResult(
  plan: PipelinePlan,
  nodeRuns: PipelineNodeRunRecord[],
  startedAtMs: number,
  iterations: number,
  exitCode: number,
  message: string,
): PipelineExecutionResult {
  const failedNodeCount = nodeRuns.filter((item) => item.status === "error").length;
  const result: PipelineResult = {
    type: "pipeline_result",
    version: plan.version,
    status: "error",
    is_error: true,
    entry_node: plan.entryNode,
    terminal_node: "",
    terminal_status: "",
    exit_code: exitCode,
    iterations,
    node_run_count: nodeRuns.length,
    failed_node_count: failedNodeCount,
    node_runs: nodeRuns,
  };

  emitPipelineEvent("plan_finish", {
    status: "error",
    finished_at: currentTimestamp(),
    duration_ms: Math.max(0, Date.now() - startedAtMs),
    iterations,
    node_run_count: nodeRuns.length,
    failed_node_count: failedNodeCount,
    terminal_node: "",
    terminal_status: "",
    exit_code: exitCode,
  });
  console.log(JSON.stringify(result));
  if (message.trim()) {
    process.stderr.write(`${message.trim()}\n`);
  }

  return {
    exitCode,
    signal: "",
  };
}

function buildAgentClaudeArgs(run: PipelineAgentRun): string[] {
  const schemaJSON = JSON.stringify(run.decision.schema);
  return [
    "--dangerously-skip-permissions",
    "--model",
    run.model,
    "--verbose",
    "--output-format",
    "stream-json",
    "--json-schema",
    schemaJSON,
    "-p",
    run.promptText,
  ];
}

async function executeAgentNode(
  node: PipelineExecutableNode,
  nodeRunID: string,
  attempt: number,
  iteration: number,
): Promise<ExecutedNodeRun> {
  if (node.run.kind !== "agent") {
    throw new Error(`executeAgentNode called for non-agent node ${node.id}`);
  }
  const run = node.run;

  const startedAt = new Date();
  emitPipelineEvent("node_start", {
    node_id: node.id,
    node_run_id: nodeRunID,
    kind: "agent",
    model: run.model,
    prompt_source: run.promptFile ? "prompt_file" : "prompt",
    prompt_file: run.promptFile ? run.promptFile.normalized : "",
    cmd: "",
    cwd: "",
    iteration,
    attempt,
    idle_timeout_sec: run.idleTimeoutSec,
    timeout_sec: run.idleTimeoutSec,
    started_at: startedAt.toISOString(),
  });

  const args = buildAgentClaudeArgs(run);

  const boundSessionIDs = new Set<string>();
  let finalResultValue: unknown = undefined;
  let hasFinalResult = false;
  let processResult: ClaudeProcessResult;
  let executionError = "";

  try {
    processResult = await runClaudeProcess(args, {
      timeoutMs: run.idleTimeoutSec * 1000,
      onIdleTimeout: () => {
        emitPipelineEvent("node_timeout", {
          node_id: node.id,
          node_run_id: nodeRunID,
          idle_timeout_sec: run.idleTimeoutSec,
          reason: `idle timeout after ${run.idleTimeoutSec} seconds without task output`,
        });
      },
      onStdoutLine: (line) => {
        const sessionID = extractSystemInitSessionID(line);
        if (sessionID && !boundSessionIDs.has(sessionID)) {
          boundSessionIDs.add(sessionID);
          emitPipelineEvent("node_session_bind", {
            node_id: node.id,
            node_run_id: nodeRunID,
            session_id: sessionID,
          });
        }

        const maybeResult = extractFinalResultEvent(line);
        if (maybeResult) {
          hasFinalResult = true;
          finalResultValue = maybeResult.result;
        }
      },
    });
  } catch (error: unknown) {
    processResult = {
      code: -1,
      signal: "",
      timedOut: false,
      timeoutMessage: "",
    };
    executionError = error instanceof Error ? error.message : String(error);
  }

  let decision: JSONObject = {};
  let errorMessage = executionError;

  if (!errorMessage) {
    if (!hasFinalResult) {
      errorMessage = "final result event not found in agent stream";
    } else {
      if (isPlainObject(finalResultValue)) {
        decision = finalResultValue as JSONObject;
      } else if (typeof finalResultValue === "string") {
        try {
          const parsedDecision = JSON.parse(finalResultValue) as JSONValue;
          if (!isPlainObject(parsedDecision)) {
            errorMessage = "decision payload must be a JSON object";
          } else {
            decision = parsedDecision as JSONObject;
          }
        } catch (error: unknown) {
          const message = error instanceof Error ? error.message : String(error);
          errorMessage = `failed to parse decision JSON: ${message}`;
        }
      } else {
        errorMessage = "decision payload must be a JSON object";
      }

      if (!errorMessage) {
        const schemaErrors = validateDecisionJSONSchema(run.decision.schema, decision, "decision");
        if (schemaErrors.length > 0) {
          errorMessage = `decision schema validation failed: ${schemaErrors.join("; ")}`;
        }
      }
    }
  }

  const finishedAt = new Date();
  const runFailed =
    Boolean(errorMessage) || processResult.timedOut || Boolean(processResult.signal) || processResult.code !== 0;
  const timeoutMessage = processResult.timeoutMessage.trim();

  const record: PipelineNodeRunRecord = {
    node_id: node.id,
    node_run_id: nodeRunID,
    kind: "agent",
    status: runFailed ? "error" : "success",
    model: run.model,
    prompt_source: run.promptFile ? "prompt_file" : "prompt",
    prompt_file: run.promptFile ? run.promptFile.normalized : "",
    cmd: "",
    cwd: "",
    exit_code: processResult.code,
    signal: processResult.signal,
    timed_out: processResult.timedOut,
    started_at: startedAt.toISOString(),
    finished_at: finishedAt.toISOString(),
    duration_ms: Math.max(0, finishedAt.getTime() - startedAt.getTime()),
    error_message: runFailed
      ? errorMessage || timeoutMessage || (processResult.signal ? `terminated by ${processResult.signal}` : `exit code ${processResult.code}`)
      : "",
  };

  emitPipelineEvent("node_finish", record);

  return {
    record,
    decision,
  };
}

async function executeCommandNode(
  node: PipelineExecutableNode,
  nodeRunID: string,
  attempt: number,
  iteration: number,
): Promise<ExecutedNodeRun> {
  if (node.run.kind !== "command") {
    throw new Error(`executeCommandNode called for non-command node ${node.id}`);
  }
  const run = node.run;

  const startedAt = new Date();
  emitPipelineEvent("node_start", {
    node_id: node.id,
    node_run_id: nodeRunID,
    kind: "command",
    model: "",
    prompt_source: "",
    prompt_file: "",
    cmd: run.cmd,
    cwd: run.cwd,
    iteration,
    attempt,
    idle_timeout_sec: run.timeoutSec,
    timeout_sec: run.timeoutSec,
    started_at: startedAt.toISOString(),
  });

  let processResult: CommandProcessResult;
  let executionError = "";

  try {
    processResult = await runCommandProcess(run.cmd, {
      cwd: run.cwd,
      timeoutMs: run.timeoutSec * 1000,
      onTimeout: () => {
        emitPipelineEvent("node_timeout", {
          node_id: node.id,
          node_run_id: nodeRunID,
          idle_timeout_sec: run.timeoutSec,
          reason: `timeout after ${run.timeoutSec} seconds`,
        });
      },
    });
  } catch (error: unknown) {
    processResult = {
      code: -1,
      signal: "",
      timedOut: false,
      timeoutMessage: "",
    };
    executionError = error instanceof Error ? error.message : String(error);
  }

  const finishedAt = new Date();
  const runFailed = Boolean(executionError) || processResult.timedOut || Boolean(processResult.signal) || processResult.code != 0;

  const record: PipelineNodeRunRecord = {
    node_id: node.id,
    node_run_id: nodeRunID,
    kind: "command",
    status: runFailed ? "error" : "success",
    model: "",
    prompt_source: "",
    prompt_file: "",
    cmd: run.cmd,
    cwd: run.cwd,
    exit_code: processResult.code,
    signal: processResult.signal,
    timed_out: processResult.timedOut,
    started_at: startedAt.toISOString(),
    finished_at: finishedAt.toISOString(),
    duration_ms: Math.max(0, finishedAt.getTime() - startedAt.getTime()),
    error_message: runFailed
      ? executionError || processResult.timeoutMessage || (processResult.signal ? `terminated by ${processResult.signal}` : `exit code ${processResult.code}`)
      : "",
  };

  emitPipelineEvent("node_finish", record);

  return {
    record,
    decision: {},
  };
}

async function executeNodeRun(
  node: PipelineExecutableNode,
  nodeRunID: string,
  attempt: number,
  iteration: number,
): Promise<ExecutedNodeRun> {
  if (node.run.kind === "agent") {
    return executeAgentNode(node, nodeRunID, attempt, iteration);
  }
  return executeCommandNode(node, nodeRunID, attempt, iteration);
}

export async function executePipelinePlan(
  plan: PipelinePlan,
  _debugEnabled: boolean,
): Promise<PipelineExecutionResult> {
  const startedAtMs = Date.now();
  const nodeRuns: PipelineNodeRunRecord[] = [];
  const nodeRunHits = new Map<string, number>();

  emitPipelineEvent("plan_start", {
    version: plan.version,
    started_at: currentTimestamp(),
    entry_node: plan.entryNode,
    node_count: plan.nodeOrder.length,
  });

  let currentNodeID = plan.entryNode;
  let iterations = 0;

  while (true) {
    const node = plan.nodes[currentNodeID];
    if (!node) {
      return executeSystemErrorResult(
        plan,
        nodeRuns,
        startedAtMs,
        iterations,
        SYSTEM_ERROR_INVALID_PLAN,
        `node not found: ${currentNodeID}`,
      );
    }

    if ("terminal" in node && node.terminal) {
      return executeTerminalResult(plan, nodeRuns, startedAtMs, node, iterations);
    }

    if (iterations >= plan.limits.maxIterations) {
      return executeSystemErrorResult(
        plan,
        nodeRuns,
        startedAtMs,
        iterations,
        SYSTEM_ERROR_LIMIT_REACHED,
        `max_iterations exceeded: ${plan.limits.maxIterations}`,
      );
    }
    const executableNode = node as PipelineExecutableNode;

    iterations += 1;

    const nodeHitCount = (nodeRunHits.get(executableNode.id) ?? 0) + 1;
    nodeRunHits.set(executableNode.id, nodeHitCount);
    if (nodeHitCount > plan.limits.maxSameNodeHits) {
      return executeSystemErrorResult(
        plan,
        nodeRuns,
        startedAtMs,
        iterations,
        SYSTEM_ERROR_LIMIT_REACHED,
        `max_same_node_hits exceeded for ${executableNode.id}: ${plan.limits.maxSameNodeHits}`,
      );
    }

    const nodeRunID = makeNodeRunID(executableNode.id, nodeRuns.length + 1);
    const executed = await executeNodeRun(executableNode, nodeRunID, nodeHitCount, iterations);
    nodeRuns.push(executed.record);

    const scope = toDecisionScope(executed.record, executed.decision, nodeHitCount, iterations, nodeRuns.length);

    let matchedTransition: PipelineExecutableNode["transitions"][number] | null = null;
    for (const transition of executableNode.transitions) {
      const compiled = compileCondition(transition.when);
      const matched = evaluateCondition(compiled, scope);
      if (!matched) {
        continue;
      }

      matchedTransition = transition;
      break;
    }

    if (!matchedTransition) {
      const noTransitionErrorCode = executed.record.status === "error" ? SYSTEM_ERROR_NODE_EXECUTION : SYSTEM_ERROR_NO_TRANSITION;
      return executeSystemErrorResult(
        plan,
        nodeRuns,
        startedAtMs,
        iterations,
        noTransitionErrorCode,
        `no transition matched for node ${executableNode.id}`,
      );
    }

    emitPipelineEvent("transition_taken", {
      node_id: executableNode.id,
      node_run_id: nodeRunID,
      from_node: executableNode.id,
      to_node: matchedTransition.to,
      when: matchedTransition.when,
      iteration: iterations,
    });

    currentNodeID = matchedTransition.to;
  }
}
