package cli

import (
	"fmt"
	"io"

	"agent-cli/internal/version"
)

// VersionCommand writes the application version to w.
func VersionCommand(w io.Writer) error {
	// #TODO(agent): fmt.Fprintf error is silently discarded. Propagate the write error
	// instead of unconditionally returning nil. Use fmt.Errorf("write version: %w", err)
	// to wrap the error if non-nil.
	fmt.Fprintf(w, "agent-cli version %s\n", version.Version())

	return nil
}
