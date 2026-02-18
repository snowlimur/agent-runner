import { Command, Option } from "commander";

import type { EntrypointArgs, Model, PromptRunOptions, Verbosity } from "./types.js";

const TEMPLATE_VAR_NAME_PATTERN = /^[A-Z][A-Z0-9_]*$/;

interface CliOptions {
  debug?: boolean;
  model?: string;
  pipeline?: string;
  var?: string[];
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
    .addOption(
      new Option("--var <key=value>", "Template variable for pipeline inline prompts (repeatable)")
        .argParser((value: string, previous: string[]) => [...previous, value])
        .default([]),
    )
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

function parseTemplateVars(values: readonly string[]): Record<string, string> {
  const vars: Record<string, string> = {};
  for (const raw of values) {
    const normalized = String(raw ?? "").trim();
    const equalIndex = normalized.indexOf("=");
    if (equalIndex <= 0) {
      throw new Error(`Invalid --var "${raw}": expected KEY=VALUE.`);
    }

    const key = normalized.slice(0, equalIndex).trim();
    const value = normalized.slice(equalIndex + 1);
    if (!TEMPLATE_VAR_NAME_PATTERN.test(key)) {
      throw new Error(`Invalid --var name "${key}": expected UPPER_SNAKE (^[A-Z][A-Z0-9_]*$).`);
    }
    if (Object.prototype.hasOwnProperty.call(vars, key)) {
      throw new Error(`Duplicate --var key "${key}".`);
    }

    vars[key] = value;
  }
  return vars;
}

export function resolveEntrypointArgs(rawArgs: readonly string[]): EntrypointArgs {
  const program = buildCliProgram();
  program.parse(["node", "entrypoint", ...rawArgs], { from: "node" });

  const options = program.opts<CliOptions>();
  const parsedTemplateVars = parseTemplateVars(Array.isArray(options.var) ? options.var : []);
  const taskArgs = program.args.map((arg) => String(arg));
  const baseArgs: EntrypointArgs = {
    debugEnabled: Boolean(options.debug),
    model: normalizeModel(options.model),
    taskArgs,
    templateVars: parsedTemplateVars,
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
