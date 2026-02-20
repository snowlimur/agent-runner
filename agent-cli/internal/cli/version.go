package cli

import (
	"fmt"
	"io"

	"agent-cli/internal/version"
)

// VersionCommand writes the application version to w.
func VersionCommand(w io.Writer) error {
	_, err := fmt.Fprintf(w, "agent-cli version %s\n", version.Version())
	if err != nil {
		return fmt.Errorf("write version: %w", err)
	}

	return nil
}
