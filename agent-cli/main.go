package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"agent-cli/internal/cli"
)

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

	if len(os.Args) < 2 {
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
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  agent-cli run [--json] [--model sonnet|opus] <prompt text>")
	fmt.Println("  agent-cli run [--json] [--model sonnet|opus] --file <path>")
	fmt.Println("  agent-cli run [--json] [--model sonnet|opus] --pipeline <path>")
	fmt.Println("  agent-cli stats [--json]")
}
