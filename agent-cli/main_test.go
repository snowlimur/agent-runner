package main_test

import (
	"os/exec"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()

	binary := t.TempDir() + "/agent-cli"

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	return binary
}

func TestMainDispatchVersion(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)

	cmd := exec.Command(binary, "version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(out), "agent-cli version") {
		t.Fatalf("expected stdout to contain %q, got %q", "agent-cli version", string(out))
	}
}

func TestHelpIncludesVersion(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)

	cmd := exec.Command(binary, "help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(out), "version") {
		t.Fatalf("expected help output to contain %q, got %q", "version", string(out))
	}
}

func TestMainDispatchVersionFlag(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)

	cmd := exec.Command(binary, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(out), "agent-cli version") {
		t.Fatalf("expected stdout to contain %q, got %q", "agent-cli version", string(out))
	}
}
