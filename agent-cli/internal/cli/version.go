package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// VersionCommand prints the application version. It accepts no positional arguments.
func VersionCommand(appVersion string, args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse version flags: %w", err)
	}

	if fs.NArg() > 0 {
		return errors.New("version command does not accept positional arguments")
	}

	fmt.Fprintf(os.Stdout, "agent-cli version %s\n", appVersion)
	return nil
}
