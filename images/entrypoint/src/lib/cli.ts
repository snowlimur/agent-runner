import { Command, Option } from "commander";

import type { EntrypointArgs, Model, PromptRunOptions, Verbosity } from "./types.js";

interface CliOptions {
  debug?: boolean;
  model?: string;
  pipeline?: string;
}

function buildCliProgram(): Command {
  const program = new Command();

  program
    .name("entrypoint")
    .description("Prepare workspace and run Claude Code")
    .showHelpAfterError()
    .version("1.0.0")
    .option("--debug", "Enable debug logs")
    .addOption(
      new Option("--model <model>", "Model for prompt mode (used as fallback in pipeline mode)")
        .choices(["sonnet", "opus"])
        .default("opus"),
    )
    .option("--pipeline <path>", "Path to YAML pipeline plan file")
    .allowUnknownOption(true)
    .allowExcessArguments(true)
    .argument("[taskArgs...]", "Prompt text (supports -v/-vv before prompt)");

  return program;
}

function normalizeModel(value: unknown): Model {
  const normalized = typeof value === "string" ? value.trim().toLowerCase() : "";
  if (normalized === "sonnet" || normalized === "opus") {
    return normalized;
  }
  return "opus";
}

export function resolveEntrypointArgs(rawArgs: readonly string[]): EntrypointArgs {
  const program = buildCliProgram();
  program.parse(["node", "entrypoint", ...rawArgs], { from: "node" });

  const options = program.opts<CliOptions>();
  const taskArgs = program.args.map((arg) => String(arg));
  const baseArgs: EntrypointArgs = {
    debugEnabled: Boolean(options.debug),
    model: normalizeModel(options.model),
    taskArgs,
  };

  return typeof options.pipeline === "string" ? { ...baseArgs, pipelinePath: options.pipeline } : baseArgs;
}

export function resolvePromptRunOptions(args: readonly string[]): PromptRunOptions {
  let verbosityLevel = 0;
  let parseFlags = true;
  const promptParts: string[] = [];

  for (const arg of args) {
    if (parseFlags && arg === "--") {
      parseFlags = false;
      continue;
    }

    if (parseFlags && arg === "-vv") {
      verbosityLevel = 2;
      continue;
    }

    if (parseFlags && arg === "-v") {
      verbosityLevel = Math.max(verbosityLevel, 1);
      continue;
    }

    promptParts.push(arg);
  }

  const prompt = promptParts.join(" ").trim();
  if (!prompt) {
    throw new Error("Prompt is empty. Pass prompt text after -v/-vv.");
  }

  const verbosity: Verbosity = verbosityLevel === 2 ? "vv" : verbosityLevel === 1 ? "v" : "text";
  return {
    prompt,
    verbosity,
    claudeArgs: resolveClaudeArgsForVerbosity(verbosity),
  };
}

export function resolveClaudeArgsForVerbosity(verbosity: Verbosity): readonly string[] {
  if (verbosity === "vv") {
    return ["--verbose", "--output-format", "stream-json"];
  }
  if (verbosity === "v") {
    return ["--output-format", "json"];
  }
  return ["--output-format", "text"];
}
