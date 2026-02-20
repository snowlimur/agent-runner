package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
)

// Version is the application version. Override at build time via:
//
//	go build -ldflags "-X agent-cli/internal/cli.Version=1.0.0"
var Version = "dev"

func VersionCommand(args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var jsonOutput bool
	fs.BoolVar(&jsonOutput, "json", false, "print version as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("version command does not accept positional arguments")
	}

	if jsonOutput {
		encoded, err := json.MarshalIndent(map[string]string{"version": Version}, "", "  ")
		if err != nil {
			return fmt.Errorf("encode version JSON: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}

	fmt.Printf("agent-cli version %s\n", Version)
	return nil
}
