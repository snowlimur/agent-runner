package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"agent-cli/internal/cli"
)

func TestVersionCommand_PrintsVersion(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := cli.VersionCommand(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got == "" {
		t.Fatal("expected version output, got empty string")
	}

	trimmed := strings.TrimSpace(got)
	if strings.Contains(trimmed, "\n") {
		t.Errorf("expected single-line output, got %q", trimmed)
	}
}

func TestVersionCommand_DefaultFallback(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := cli.VersionCommand(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got == "" {
		t.Fatal("expected fallback version, got empty string")
	}
}

func TestSetVersion_OverridesDefault(t *testing.T) {
	t.Parallel()

	const injected = "v1.2.3"
	restore := cli.SetVersion(injected)
	defer restore()

	var buf bytes.Buffer
	err := cli.VersionCommand(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != injected {
		t.Errorf("expected %q, got %q", injected, got)
	}
}

func TestSetVersion_CommitHash(t *testing.T) {
	t.Parallel()

	const hash = "abc1234"
	restore := cli.SetVersion(hash)
	defer restore()

	var buf bytes.Buffer
	err := cli.VersionCommand(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != hash {
		t.Errorf("expected %q, got %q", hash, got)
	}
}
