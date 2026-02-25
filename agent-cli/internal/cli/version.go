package cli

import (
	"fmt"
	"io"
	"sync"
)

var version = "dev" //nolint:gochecknoglobals // injected at build time via ldflags

var versionMu sync.Mutex //nolint:gochecknoglobals // guards version for concurrent test access

// VersionCommand writes the current version string to w.
func VersionCommand(w io.Writer) error {
	versionMu.Lock()
	v := version
	versionMu.Unlock()

	_, err := fmt.Fprintln(w, v)

	return err
}

// SetVersion overrides the version string and returns a restore function.
func SetVersion(v string) func() {
	versionMu.Lock()
	old := version
	version = v //nolint:reassign // test helper for version override
	versionMu.Unlock()

	return func() {
		versionMu.Lock()
		version = old //nolint:reassign // restore original version
		versionMu.Unlock()
	}
}
