import type { ChildProcessByStdio, SpawnOptions } from "node:child_process";
import type { Readable } from "node:stream";

export type Model = "sonnet" | "opus";
export type Verbosity = "text" | "v" | "vv";
export type OnErrorPolicy = "fail_fast" | "continue";
export type WorkspaceMode = "shared" | "worktree" | "snapshot_ro";
export type StageMode = "sequential" | "parallel";
export type TaskStatus = "success" | "error";
export type PromptSource = "prompt" | "prompt_file";
export type PipelineVersion = "v1";

export interface EntrypointArgs {
  debugEnabled: boolean;
  model: Model;
  taskArgs: string[];
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
  verbosity: Verbosity;
  onError: OnErrorPolicy;
  workspace: WorkspaceMode;
}

export interface PipelineTask {
  id: string;
  prompt: string | null;
  promptFile: PromptFileRef | null;
  onError: OnErrorPolicy;
  workspace: WorkspaceMode;
  model: Model;
  verbosity: Verbosity;
  readOnly: boolean;
  allowSharedWrites: boolean;
  promptSource: PromptSource;
  promptText: string;
}

export interface PipelineStage {
  id: string;
  mode: StageMode;
  maxParallel: number;
  onError: OnErrorPolicy;
  workspace: WorkspaceMode;
  model: Model;
  verbosity: Verbosity;
  tasks: PipelineTask[];
}

export interface PipelinePlan {
  version: PipelineVersion;
  defaults: PipelineDefaults;
  stages: PipelineStage[];
}

export interface ClaudeProcessResult {
  code: number;
  signal: NodeJS.Signals | "";
}

export interface PipelineExecutionResult {
  exitCode: number;
  signal: NodeJS.Signals | "";
}

export interface RunClaudeProcessOptions extends Omit<SpawnOptions, "stdio"> {
  onStdoutLine?: (line: string) => void;
}

export interface DinDRuntime {
  child: ChildProcessByStdio<null, Readable, Readable>;
  storageDriver: string;
  getLogTail: () => string;
}

export interface PipelineTaskResult {
  stage_id: string;
  task_id: string;
  status: TaskStatus;
  on_error: OnErrorPolicy;
  workspace: WorkspaceMode;
  model: Model;
  verbosity: Verbosity;
  prompt_source: PromptSource;
  prompt_file: string;
  exit_code: number;
  signal: NodeJS.Signals | "";
  started_at: string;
  finished_at: string;
  duration_ms: number;
  error_message: string;
}

export interface PipelineStageResult {
  stage_id: string;
  mode: StageMode;
  status: TaskStatus;
  task_count: number;
  completed_tasks: number;
  failed_tasks: number;
  duration_ms: number;
}

export interface PipelineResult {
  type: "pipeline_result";
  version: PipelineVersion;
  status: TaskStatus;
  is_error: boolean;
  stage_count: number;
  completed_stages: number;
  task_count: number;
  failed_task_count: number;
  tasks: PipelineTaskResult[];
}

export interface StageExecutionOutcome {
  taskResults: PipelineTaskResult[];
  stopAfterStage: boolean;
}

export interface PipelineEventPayloadMap {
  plan_start: {
    version: PipelineVersion;
    started_at: string;
    stage_count: number;
  };
  stage_start: {
    stage_id: string;
    mode: StageMode;
    started_at: string;
    task_count: number;
    max_parallel: number;
  };
  task_start: {
    stage_id: string;
    task_id: string;
    model: Model;
    verbosity: Verbosity;
    workspace: WorkspaceMode;
    prompt_source: PromptSource;
    prompt_file: string;
    started_at: string;
  };
  task_session_bind: {
    stage_id: string;
    task_id: string;
    session_id: string;
  };
  task_finish: PipelineTaskResult;
  stage_finish: PipelineStageResult;
  plan_finish: {
    status: TaskStatus;
    finished_at: string;
    duration_ms: number;
    stage_count: number;
    completed_stages: number;
    task_count: number;
    failed_task_count: number;
  };
}

export type PipelineEventName = keyof PipelineEventPayloadMap;
