import process from "node:process";
import { execFileSync } from "node:child_process";

export function runSync(command, args, options = {}) {
  return execFileSync(command, args, {
    encoding: "utf8",
    ...options,
  });
}

export function firstNonEmptyEnv(keys, fallback) {
  for (const key of keys) {
    const value = process.env[key];
    if (typeof value === "string") {
      const trimmed = value.trim();
      if (trimmed) {
        return trimmed;
      }
    }
  }
  return fallback;
}

export function isTruthyEnv(value) {
  if (typeof value !== "string") {
    return false;
  }

  switch (value.trim().toLowerCase()) {
    case "1":
    case "true":
    case "yes":
    case "on":
      return true;
    default:
      return false;
  }
}

export function parsePositiveInteger(value, fallback) {
  const parsed = Number.parseInt(String(value ?? "").trim(), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

export function sleepMs(milliseconds) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, milliseconds);
}

export function debugLog(debugEnabled, message) {
  if (debugEnabled) {
    console.log(message);
  }
}

export function isPlainObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function requireNonEmptyString(value, fieldName) {
  if (typeof value !== "string") {
    throw new Error(`${fieldName} must be a string.`);
  }
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error(`${fieldName} must not be empty.`);
  }
  return trimmed;
}

export function sanitizeIdentifier(value) {
  const source = String(value ?? "").trim();
  if (!source) {
    return "unknown";
  }
  return source.replace(/[^a-zA-Z0-9._-]+/g, "_").replace(/_{2,}/g, "_").slice(0, 96) || "unknown";
}
