package cli

import (
	"fmt"
	"io"
)

// version is set at build time via ldflags; defaults to "dev".
var version = "dev" //nolint:gochecknoglobals // injected at build time

// VersionCommand writes the current version string to w.
func VersionCommand(w io.Writer) error {
	_, err := fmt.Fprintln(w, version)
	return err
}
