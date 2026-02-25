package cli

import (
	"bytes"
	"testing"
)

func TestVersionCommand_DefaultVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := VersionCommand(&buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got != "dev\n" {
		t.Errorf("expected %q, got %q", "dev\n", got)
	}
}

func TestVersionCommand_InjectedVersion(t *testing.T) {
	prev := version
	t.Cleanup(func() { version = prev })

	version = "v1.2.3"

	var buf bytes.Buffer
	if err := VersionCommand(&buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got != "v1.2.3\n" {
		t.Errorf("expected %q, got %q", "v1.2.3\n", got)
	}
}

func TestVersionCommand_OutputFormat(t *testing.T) {
	prev := version
	t.Cleanup(func() { version = prev })

	version = "abc123"

	var buf bytes.Buffer
	if err := VersionCommand(&buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	// Should be plain text, single line, with trailing newline
	if got != "abc123\n" {
		t.Errorf("expected plain single-line output with newline, got %q", got)
	}
}
