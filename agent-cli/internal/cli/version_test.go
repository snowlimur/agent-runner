package cli

import (
	"encoding/json"
	"bytes"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn while capturing os.Stdout and returns the output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return buf.String()
}

func TestVersionCommandPlainOutput(t *testing.T) {
	orig := Version
	Version = "1.2.3"
	defer func() { Version = orig }()

	output := captureStdout(t, func() {
		if err := VersionCommand(nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	expected := "agent-cli version 1.2.3\n"
	if output != expected {
		t.Fatalf("expected %q, got %q", expected, output)
	}
}

func TestVersionCommandDefaultVersion(t *testing.T) {
	orig := Version
	Version = "dev"
	defer func() { Version = orig }()

	output := captureStdout(t, func() {
		if err := VersionCommand(nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	expected := "agent-cli version dev\n"
	if output != expected {
		t.Fatalf("expected %q, got %q", expected, output)
	}
}

func TestVersionCommandJSONOutput(t *testing.T) {
	orig := Version
	Version = "2.0.0"
	defer func() { Version = orig }()

	output := captureStdout(t, func() {
		if err := VersionCommand([]string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result["version"] != "2.0.0" {
		t.Fatalf("expected version %q, got %q", "2.0.0", result["version"])
	}
}

func TestVersionCommandRejectsPositionalArgs(t *testing.T) {
	err := VersionCommand([]string{"extra"})
	if err == nil {
		t.Fatal("expected error for positional arguments")
	}
	if !strings.Contains(err.Error(), "positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}
