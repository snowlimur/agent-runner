import process from "node:process";

import { runEntrypoint } from "./lib/main.js";

runEntrypoint().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`Entrypoint failed: ${message}`);
  process.exit(1);
});
