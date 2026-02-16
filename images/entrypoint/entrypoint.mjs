#!/usr/bin/env node

import process from "node:process";

import { runEntrypoint } from "./lib/main.mjs";

runEntrypoint().catch((error) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`Entrypoint failed: ${message}`);
  process.exit(1);
});
