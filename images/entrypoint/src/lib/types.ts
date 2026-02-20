import type { ChildProcessByStdio, SpawnOptions } from "node:child_process";
import type { Readable } from "node:stream";

export type Model = "sonnet" | "opus";
export type Verbosity = "text" | "v" | "vv";
export type PipelineVersion = "v2";

export type PipelineNodeKind = "agent" | "command";
export type PipelineNodeRunStatus = "success" | "error";
export type PipelineTerminalStatus = "success" | "blocked" | "failed" | "canceled";

export type JSONPrimitive = string | number | boolean | null;
export type JSONValue = JSONPrimitive | JSONArray | JSONObject;
export interface JSONObject {
  [key: string]: JSONValue;
}
export type JSONArray = JSONValue[];

export interface EntrypointArgs {
  debugEnabled: boolean;
  model: Model;
  taskArgs: string[];
  templateVars: Record<string, string>;
  pipelinePath?: string;
}

export interface PromptRunOptions {
  prompt: string;
  verbosity: Verbosity;
  claudeArgs: readonly string[];
}

export interface PromptFileRef {
  raw: string;
  normalized: string;
  resolved: string;
}

export interface PipelineDefaults {
  model: Model;
  agentIdleTimeoutSec: number;
  commandTimeoutSec: number;
}

export interface PipelineLimits {
  maxIterations: number;
  maxSameNodeHits: number;
}

export interface PipelineTransition {
  when: string;
  to: string;
}

export interface PipelineDecisionRule {
  schemaFile: PromptFileRef;
  schema: JSONObject;
}

export interface PipelineAgentRun {
  kind: "agent";
  model: Model;
  promptFile: PromptFileRef | null;
  promptText: string;
  idleTimeoutSec: number;
  decision: PipelineDecisionRule;
}

export interface PipelineCommandRun {
  kind: "command";
  cmd: string;
  cwd: string;
  timeoutSec: number;
}

export type PipelineNodeRun = PipelineAgentRun | PipelineCommandRun;

export interface PipelineTerminalNode {
  id: string;
  terminal: true;
  terminalStatus: PipelineTerminalStatus;
  exitCode: number;
  message: string;
}

export interface PipelineExecutableNode {
  id: string;
  run: PipelineNodeRun;
  transitions: PipelineTransition[];
}

export type PipelineNode = PipelineTerminalNode | PipelineExecutableNode;

export interface PipelinePlan {
  version: PipelineVersion;
  entryNode: string;
  defaults: PipelineDefaults;
  limits: PipelineLimits;
  nodeOrder: string[];
  nodes: Record<string, PipelineNode>;
}

export interface ClaudeProcessResult {
  code: number;
  signal: NodeJS.Signals | "";
  timedOut: boolean;
  timeoutMessage: string;
  fatalErrorCode: string;
  fatalErrorMessage: string;
}

export interface CommandProcessResult {
  code: number;
  signal: NodeJS.Signals | "";
  timedOut: boolean;
  timeoutMessage: string;
}

export interface PipelineExecutionResult {
  exitCode: number;
  signal: NodeJS.Signals | "";
}

export interface RunClaudeProcessOptions extends Omit<SpawnOptions, "stdio"> {
  onStdoutLine?: (line: string) => void;
  timeoutMs?: number;
  onIdleTimeout?: (timeoutMs: number) => void;
}

export interface RunCommandProcessOptions extends Omit<SpawnOptions, "stdio"> {
  timeoutMs?: number;
  onTimeout?: (timeoutMs: number) => void;
}

export interface DinDRuntime {
  child: ChildProcessByStdio<null, Readable, Readable>;
  storageDriver: string;
  getLogTail: () => string;
}

export interface PipelineNodeRunRecord {
  node_id: string;
  node_run_id: string;
  kind: PipelineNodeKind;
  status: PipelineNodeRunStatus;
  model: Model | "";
  prompt_source: "prompt" | "prompt_file" | "";
  prompt_file: string;
  cmd: string;
  cwd: string;
  exit_code: number;
  signal: NodeJS.Signals | "";
  timed_out: boolean;
  started_at: string;
  finished_at: string;
  duration_ms: number;
  error_message: string;
}

export interface PipelineResult {
  type: "pipeline_result";
  version: PipelineVersion;
  status: PipelineNodeRunStatus;
  is_error: boolean;
  entry_node: string;
  terminal_node: string;
  terminal_status: PipelineTerminalStatus | "";
  exit_code: number;
  iterations: number;
  node_run_count: number;
  failed_node_count: number;
  node_runs: PipelineNodeRunRecord[];
}

export interface PipelineRuntimeState {
  iteration: number;
  totalNodeRuns: number;
  currentNodeID: string;
  currentNodeRunID: string;
}

export interface PipelineConditionScope {
  decision: JSONObject;
  run: {
    exit_code: number;
    signal: string;
    timed_out: boolean;
    status: PipelineNodeRunStatus;
  };
  node: {
    id: string;
    kind: PipelineNodeKind;
    attempt: number;
    run_id: string;
  };
  pipeline: {
    iteration: number;
    total_node_runs: number;
  };
}

export interface PipelineEventPayloadMap {
  plan_start: {
    version: PipelineVersion;
    started_at: string;
    entry_node: string;
    node_count: number;
  };
  node_start: {
    node_id: string;
    node_run_id: string;
    kind: PipelineNodeKind;
    model: Model | "";
    prompt_source: "prompt" | "prompt_file" | "";
    prompt_file: string;
    cmd: string;
    cwd: string;
    iteration: number;
    attempt: number;
    idle_timeout_sec: number;
    timeout_sec: number;
    started_at: string;
  };
  node_session_bind: {
    node_id: string;
    node_run_id: string;
    session_id: string;
  };
  node_timeout: {
    node_id: string;
    node_run_id: string;
    idle_timeout_sec: number;
    reason: string;
  };
  node_finish: PipelineNodeRunRecord;
  transition_taken: {
    node_id: string;
    node_run_id: string;
    from_node: string;
    to_node: string;
    when: string;
    iteration: number;
  };
  plan_finish: {
    status: PipelineNodeRunStatus;
    finished_at: string;
    duration_ms: number;
    iterations: number;
    node_run_count: number;
    failed_node_count: number;
    terminal_node: string;
    terminal_status: PipelineTerminalStatus | "";
    exit_code: number;
  };
}

export type PipelineEventName = keyof PipelineEventPayloadMap;
