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
} from "./constants.mjs";
import {
  debugLog,
  firstNonEmptyEnv,
  isTruthyEnv,
  parsePositiveInteger,
  runSync,
  sleepMs,
} from "./utils.mjs";

function appendLogTail(current, chunk) {
  const next = `${current}${chunk}`;
  if (next.length <= DIND_LOG_TAIL_LIMIT) {
    return next;
  }
  return next.slice(next.length - DIND_LOG_TAIL_LIMIT);
}

function readDockerdPid() {
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

function processExists(pid) {
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

function canTalkToDocker() {
  try {
    runSync("docker", ["info"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

function launchDockerd(debugEnabled, storageDriver) {
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

  child.stdout.on("data", (chunk) => {
    logTail = appendLogTail(logTail, chunk);
    if (debugEnabled) {
      process.stdout.write(`[dockerd] ${chunk}`);
    }
  });

  child.stderr.on("data", (chunk) => {
    logTail = appendLogTail(logTail, chunk);
    if (debugEnabled) {
      process.stderr.write(`[dockerd] ${chunk}`);
    }
  });

  child.on("error", (error) => {
    logTail = appendLogTail(logTail, `\nspawn error: ${error.message}\n`);
  });

  return {
    child,
    storageDriver,
    getLogTail: () => logTail,
  };
}

function waitForDockerReady(runtime, timeoutSeconds) {
  const deadline = Date.now() + timeoutSeconds * 1000;

  while (Date.now() < deadline) {
    if (runtime.child.exitCode !== null) {
      return false;
    }
    if (canTalkToDocker()) {
      return true;
    }
    sleepMs(1000);
  }

  return false;
}

function terminateDockerdProcess(runtime, debugEnabled) {
  if (!runtime) {
    return;
  }

  const pid = readDockerdPid();
  if (pid !== null) {
    try {
      runSync("sudo", ["-n", "kill", "-TERM", String(pid)], { stdio: "ignore" });
    } catch (error) {
      debugLog(debugEnabled, `dockerd TERM failed: ${String(error)}`);
    }
  }

  if (runtime.child.exitCode === null) {
    try {
      runtime.child.kill("SIGTERM");
    } catch (error) {
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
    } catch (error) {
      debugLog(debugEnabled, `dockerd KILL failed: ${String(error)}`);
    }
  }

  if (runtime.child.exitCode === null) {
    try {
      runtime.child.kill("SIGKILL");
    } catch (error) {
      debugLog(debugEnabled, `dockerd child SIGKILL failed: ${String(error)}`);
    }
  }
}

export function startDinD(debugEnabled) {
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

  let lastFailure = null;

  for (const driver of drivers) {
    debugLog(debugEnabled, `Starting dockerd with storage driver: ${driver}`);
    const runtime = launchDockerd(debugEnabled, driver);

    if (waitForDockerReady(runtime, timeoutSeconds)) {
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

export function stopDinD(runtime, debugEnabled) {
  if (!runtime) {
    return;
  }

  debugLog(debugEnabled, "Stopping DinD daemon...");
  terminateDockerdProcess(runtime, debugEnabled);
}

export function installDinDSignalHandlers(stopRuntime, debugEnabled) {
  const forwardSignal = (signalName) => {
    stopRuntime();
    process.removeListener("SIGINT", onSigInt);
    process.removeListener("SIGTERM", onSigTerm);
    process.kill(process.pid, signalName);
  };

  const onSigInt = () => forwardSignal("SIGINT");
  const onSigTerm = () => forwardSignal("SIGTERM");

  process.on("SIGINT", onSigInt);
  process.on("SIGTERM", onSigTerm);
  process.on("exit", () => {
    stopRuntime();
  });
}
