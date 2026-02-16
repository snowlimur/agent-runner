import fs from "node:fs";
import process from "node:process";
import { spawn } from "node:child_process";

import {
  DEFAULT_DIND_STARTUP_TIMEOUT_SEC,
  DEFAULT_DIND_STORAGE_DRIVER,
  DIND_DATA_ROOT,
  DIND_LOG_TAIL_LIMIT,
  DIND_PID_FILE,
  DIND_SOCKET_PATH,
} from "./constants.js";
import type { DinDRuntime } from "./types.js";
import {
  debugLog,
  firstNonEmptyEnv,
  isTruthyEnv,
  parsePositiveInteger,
  runSync,
  sleepMs,
} from "./utils.js";

const OVERLAY2_FAST_FALLBACK_GRACE_MS = 5_000;
const OVERLAY2_FATAL_PATTERNS = ["failed to mount overlay", "driver not supported: overlay2"];

function appendLogTail(current: string, chunk: string): string {
  const next = `${current}${chunk}`;
  if (next.length <= DIND_LOG_TAIL_LIMIT) {
    return next;
  }
  return next.slice(next.length - DIND_LOG_TAIL_LIMIT);
}

function readDockerdPid(): number | null {
  try {
    const raw = fs.readFileSync(DIND_PID_FILE, "utf8").trim();
    if (!raw) {
      return null;
    }

    const pid = Number.parseInt(raw, 10);
    if (!Number.isFinite(pid) || pid <= 0) {
      return null;
    }
    return pid;
  } catch {
    return null;
  }
}

function processExists(pid: number): boolean {
  if (!Number.isFinite(pid) || pid <= 0) {
    return false;
  }

  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function canTalkToDocker(): boolean {
  try {
    runSync("docker", ["info"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

function launchDockerd(debugEnabled: boolean, storageDriver: string): DinDRuntime {
  let logTail = "";
  const dockerdArgs = [
    "-n",
    "dockerd",
    `--host=unix://${DIND_SOCKET_PATH}`,
    `--pidfile=${DIND_PID_FILE}`,
    `--data-root=${DIND_DATA_ROOT}`,
    `--storage-driver=${storageDriver}`,
  ];

  const child = spawn("sudo", dockerdArgs, { stdio: ["ignore", "pipe", "pipe"] });
  child.stdout.setEncoding("utf8");
  child.stderr.setEncoding("utf8");

  child.stdout.on("data", (chunk: string) => {
    logTail = appendLogTail(logTail, chunk);
    if (debugEnabled) {
      process.stdout.write(`[dockerd] ${chunk}`);
    }
  });

  child.stderr.on("data", (chunk: string) => {
    logTail = appendLogTail(logTail, chunk);
    if (debugEnabled) {
      process.stderr.write(`[dockerd] ${chunk}`);
    }
  });

  child.on("error", (error: Error) => {
    logTail = appendLogTail(logTail, `\nspawn error: ${error.message}\n`);
  });

  return {
    child,
    storageDriver,
    getLogTail: () => logTail,
  };
}

function hasOverlay2FatalError(logTail: string): boolean {
  const normalized = logTail.toLowerCase();
  return OVERLAY2_FATAL_PATTERNS.some((pattern) => normalized.includes(pattern));
}

function waitForDockerReady(
  runtime: DinDRuntime,
  timeoutSeconds: number,
  useFastOverlayFallback: boolean,
): boolean {
  const deadline = Date.now() + timeoutSeconds * 1000;
  const fastFallbackDeadline = useFastOverlayFallback
    ? Date.now() + OVERLAY2_FAST_FALLBACK_GRACE_MS
    : Number.POSITIVE_INFINITY;

  while (Date.now() < deadline) {
    if (runtime.child.exitCode !== null) {
      return false;
    }
    if (canTalkToDocker()) {
      return true;
    }

    if (useFastOverlayFallback) {
      if (hasOverlay2FatalError(runtime.getLogTail())) {
        return false;
      }
      if (Date.now() >= fastFallbackDeadline) {
        return false;
      }
    }

    sleepMs(250);
  }

  return false;
}

function terminateDockerdProcess(runtime: DinDRuntime | null, debugEnabled: boolean): void {
  if (!runtime) {
    return;
  }

  const pid = readDockerdPid();
  if (pid !== null) {
    try {
      runSync("sudo", ["-n", "kill", "-TERM", String(pid)], { stdio: "ignore" });
    } catch (error: unknown) {
      debugLog(debugEnabled, `dockerd TERM failed: ${String(error)}`);
    }
  }

  if (runtime.child.exitCode === null) {
    try {
      runtime.child.kill("SIGTERM");
    } catch (error: unknown) {
      debugLog(debugEnabled, `dockerd child SIGTERM failed: ${String(error)}`);
    }
  }

  const waitDeadline = Date.now() + 10_000;
  while (Date.now() < waitDeadline) {
    const daemonStopped = !canTalkToDocker();
    const childStopped = runtime.child.exitCode !== null;
    if (daemonStopped && childStopped) {
      break;
    }
    sleepMs(250);
  }

  if (pid !== null && processExists(pid)) {
    try {
      runSync("sudo", ["-n", "kill", "-KILL", String(pid)], { stdio: "ignore" });
    } catch (error: unknown) {
      debugLog(debugEnabled, `dockerd KILL failed: ${String(error)}`);
    }
  }

  if (runtime.child.exitCode === null) {
    try {
      runtime.child.kill("SIGKILL");
    } catch (error: unknown) {
      debugLog(debugEnabled, `dockerd child SIGKILL failed: ${String(error)}`);
    }
  }
}

export function startDinD(debugEnabled: boolean): DinDRuntime | null {
  if (!isTruthyEnv(process.env.ENABLE_DIND)) {
    return null;
  }

  if (fs.existsSync(DIND_SOCKET_PATH)) {
    throw new Error(
      `${DIND_SOCKET_PATH} already exists. ENABLE_DIND=1 cannot be used with a pre-mounted docker socket.`,
    );
  }

  const timeoutSeconds = parsePositiveInteger(
    process.env.DIND_STARTUP_TIMEOUT_SEC,
    DEFAULT_DIND_STARTUP_TIMEOUT_SEC,
  );
  const preferredDriver = firstNonEmptyEnv(["DIND_STORAGE_DRIVER"], DEFAULT_DIND_STORAGE_DRIVER);
  const drivers = preferredDriver === "overlay2" ? ["overlay2", "vfs"] : [preferredDriver];

  let lastFailure: { driver: string; logTail: string } | null = null;

  for (const driver of drivers) {
    debugLog(debugEnabled, `Starting dockerd with storage driver: ${driver}`);
    const runtime = launchDockerd(debugEnabled, driver);
    const useFastOverlayFallback = driver === "overlay2" && drivers.includes("vfs");

    if (waitForDockerReady(runtime, timeoutSeconds, useFastOverlayFallback)) {
      debugLog(debugEnabled, `DinD is ready (driver=${driver}).`);
      return runtime;
    }

    const logTail = runtime.getLogTail();
    terminateDockerdProcess(runtime, debugEnabled);
    lastFailure = {
      driver,
      logTail,
    };

    if (driver === "overlay2" && drivers.includes("vfs")) {
      debugLog(debugEnabled, "DinD with overlay2 failed readiness check, retrying with vfs...");
    }
  }

  if (lastFailure !== null) {
    throw new Error(
      `Failed to start dockerd (driver=${lastFailure.driver}). Tail logs:\n${lastFailure.logTail || "<empty>"}`,
    );
  }

  throw new Error("Failed to start dockerd.");
}

export function stopDinD(runtime: DinDRuntime | null, debugEnabled: boolean): void {
  if (!runtime) {
    return;
  }

  debugLog(debugEnabled, "Stopping DinD daemon...");
  terminateDockerdProcess(runtime, debugEnabled);
}

export function installDinDSignalHandlers(stopRuntime: () => void, _debugEnabled: boolean): void {
  const forwardSignal = (signalName: NodeJS.Signals): void => {
    stopRuntime();
    process.removeListener("SIGINT", onSigInt);
    process.removeListener("SIGTERM", onSigTerm);
    process.kill(process.pid, signalName);
  };

  const onSigInt = (): void => forwardSignal("SIGINT");
  const onSigTerm = (): void => forwardSignal("SIGTERM");

  process.on("SIGINT", onSigInt);
  process.on("SIGTERM", onSigTerm);
  process.on("exit", () => {
    stopRuntime();
  });
}
