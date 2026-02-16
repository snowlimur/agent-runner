import { firstNonEmptyEnv } from "./utils.js";
import type { OnErrorPolicy, PipelineVersion, Verbosity, WorkspaceMode } from "./types.js";

export const TARGET_WORKSPACE_DIR = "/workspace";
export const SOURCE_WORKSPACE_DIR = firstNonEmptyEnv(["SOURCE_WORKSPACE_DIR"], "/workspace-source");

export const DIND_SOCKET_PATH = "/var/run/docker.sock";
export const DIND_PID_FILE = "/var/run/dockerd.pid";
export const DIND_DATA_ROOT = "/var/lib/docker";
export const DEFAULT_DIND_STORAGE_DRIVER = "overlay2";
export const DEFAULT_DIND_STARTUP_TIMEOUT_SEC = 45;
export const DIND_LOG_TAIL_LIMIT = 32 * 1024;

export const PIPELINE_VERSION: PipelineVersion = "v1";
export const PIPELINE_DEFAULT_ON_ERROR: OnErrorPolicy = "fail_fast";
export const PIPELINE_DEFAULT_WORKSPACE: WorkspaceMode = "shared";
export const PIPELINE_DEFAULT_VERBOSITY: Verbosity = "vv";
export const PIPELINE_WORKSPACE_ROOT = "/tmp/agent-pipeline-workspaces";
