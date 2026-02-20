package cli

import (
	"fmt"
	"io"

	"agent-cli/internal/version"
)

// VersionCommand writes the application version to w.
func VersionCommand(w io.Writer) error {
	fmt.Fprintf(w, "agent-cli version %s\n", version.Version())

	return nil
}
