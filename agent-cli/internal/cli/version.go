package cli

import (
	"fmt"
	"io"
)

// VersionCommand prints ver to w followed by a newline.
func VersionCommand(w io.Writer, ver string) error {
	_, err := fmt.Fprintln(w, ver)
	return err
}
