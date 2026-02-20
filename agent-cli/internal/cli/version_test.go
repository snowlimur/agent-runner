package cli

import (
	"bytes"
	"testing"
)

func TestVersionCommandOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := VersionCommand(&buf); err != nil {
		t.Fatalf("VersionCommand() returned unexpected error: %v", err)
	}

	want := "agent-cli version dev\n"
	if got := buf.String(); got != want {
		t.Fatalf("VersionCommand() output = %q, want %q", got, want)
	}
}
