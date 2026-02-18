import process from "node:process";

import { runEntrypoint } from "./lib/main.js";

runEntrypoint().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`Entrypoint failed: ${message}`);

  const exitCode =
    error !== null &&
    typeof error === "object" &&
    typeof (error as { exitCode?: unknown }).exitCode === "number" &&
    Number.isFinite((error as { exitCode: number }).exitCode)
      ? Math.trunc((error as { exitCode: number }).exitCode)
      : 1;

  process.exit(exitCode);
});
