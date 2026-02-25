package cli_test

import (
	"bytes"
	"testing"

	"agent-cli/internal/cli"
)

func TestVersionCommand(t *testing.T) {
	t.Run("prints provided version", func(t *testing.T) {
		var buf bytes.Buffer
		err := cli.VersionCommand(&buf, "v1.2.3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := buf.String()
		want := "v1.2.3\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("prints dev fallback", func(t *testing.T) {
		var buf bytes.Buffer
		err := cli.VersionCommand(&buf, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := buf.String()
		want := "dev\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
