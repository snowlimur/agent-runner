import { Command, Option } from "commander";

function buildCliProgram() {
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

export function resolveEntrypointArgs(rawArgs) {
  const program = buildCliProgram();
  program.parse(["node", "entrypoint", ...rawArgs], { from: "node" });

  const options = program.opts();
  return {
    debugEnabled: Boolean(options.debug),
    model: options.model,
    taskArgs: program.args,
    pipelinePath: options.pipeline,
  };
}

export function resolvePromptRunOptions(args) {
  let verbosityLevel = 0;
  let parseFlags = true;
  const promptParts = [];

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

  if (verbosityLevel === 2) {
    return {
      prompt,
      verbosity: "vv",
      claudeArgs: ["--verbose", "--output-format", "stream-json"],
    };
  }

  if (verbosityLevel === 1) {
    return {
      prompt,
      verbosity: "v",
      claudeArgs: ["--output-format", "json"],
    };
  }

  return {
    prompt,
    verbosity: "text",
    claudeArgs: ["--output-format", "text"],
  };
}

export function resolveClaudeArgsForVerbosity(verbosity) {
  if (verbosity === "vv") {
    return ["--verbose", "--output-format", "stream-json"];
  }
  if (verbosity === "v") {
    return ["--output-format", "json"];
  }
  return ["--output-format", "text"];
}
