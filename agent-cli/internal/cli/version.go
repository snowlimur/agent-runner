package cli

import (
	"fmt"
	"io"

	"agent-cli/internal/version"
)

// VersionCommand prints the application version to w.
func VersionCommand(w io.Writer, _ []string) error {
	_, err := fmt.Fprintf(w, "agent-cli version %s\n", version.Info())
	return err
}
