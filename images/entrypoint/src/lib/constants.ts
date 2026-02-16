import { firstNonEmptyEnv, parsePositiveInteger } from "./utils.js";
import type { PipelineVersion } from "./types.js";

export const TARGET_WORKSPACE_DIR = "/workspace";
export const SOURCE_WORKSPACE_DIR = firstNonEmptyEnv(["SOURCE_WORKSPACE_DIR"], "/workspace-source");

export const DIND_SOCKET_PATH = "/var/run/docker.sock";
export const DIND_PID_FILE = "/var/run/dockerd.pid";
export const DIND_DATA_ROOT = "/var/lib/docker";
export const DEFAULT_DIND_STORAGE_DRIVER = "overlay2";
export const DEFAULT_DIND_STARTUP_TIMEOUT_SEC = 45;
export const DIND_LOG_TAIL_LIMIT = 32 * 1024;

export const PIPELINE_VERSION: PipelineVersion = "v2";
export const PIPELINE_DEFAULT_AGENT_IDLE_TIMEOUT_SEC = parsePositiveInteger(
  firstNonEmptyEnv(["PIPELINE_AGENT_IDLE_TIMEOUT_SEC"], "1800"),
  1800,
);
export const PIPELINE_DEFAULT_COMMAND_TIMEOUT_SEC = parsePositiveInteger(
  firstNonEmptyEnv(["PIPELINE_COMMAND_TIMEOUT_SEC"], "1800"),
  1800,
);
export const PIPELINE_DEFAULT_MAX_ITERATIONS = parsePositiveInteger(
  firstNonEmptyEnv(["PIPELINE_MAX_ITERATIONS"], "100"),
  100,
);
export const PIPELINE_DEFAULT_MAX_SAME_NODE_HITS = parsePositiveInteger(
  firstNonEmptyEnv(["PIPELINE_MAX_SAME_NODE_HITS"], "25"),
  25,
);
