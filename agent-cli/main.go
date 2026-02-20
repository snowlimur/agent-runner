package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"agent-cli/internal/cli"
)

const minArgsWithCommand = 2

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}

	if len(os.Args) < minArgsWithCommand {
		printUsage()
		return nil
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "run":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return cli.RunCommand(ctx, cwd, args)
	case "stats":
		return cli.StatsCommand(cwd, args)
	case "version", "--version", "-v":
		return cli.VersionCommand(args)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func printUsage() {
	_, _ = os.Stdout.WriteString(`Usage:
  agent-cli run [--json] [--model sonnet|opus] [--debug] <prompt text>
  agent-cli run [--json] [--model sonnet|opus] [--debug] --file <path>
  agent-cli run [--json] [--model sonnet|opus] [--debug] --pipeline <path> [--var KEY=VALUE ...]
  agent-cli stats [--json]
  agent-cli version [--json]
`)
}
