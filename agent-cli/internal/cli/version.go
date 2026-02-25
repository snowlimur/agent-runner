package cli

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time via ldflags. Falls back to "dev" when built
// without injection (e.g. plain `go build`).
var version = "dev"

// writeVersion writes the current version string to w.
func writeVersion(w io.Writer) error {
	_, err := fmt.Fprintln(w, version)
	return err
}

// VersionCommand prints the application version to stdout.
func VersionCommand() error {
	return writeVersion(os.Stdout)
}
