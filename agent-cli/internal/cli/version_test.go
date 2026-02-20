package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestVersionCommand_PrintsVersion(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	cmdErr := VersionCommand("v1.2.3", nil)

	os.Stdout = origStdout
	w.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	got := buf.String()
	want := "agent-cli version v1.2.3\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestVersionCommand_RejectsPositionalArgs(t *testing.T) {
	err := VersionCommand("v1.0.0", []string{"extra"})
	if err == nil {
		t.Fatal("expected error for positional args, got nil")
	}

	want := "does not accept positional arguments"
	if got := err.Error(); !contains(got, want) {
		t.Errorf("error = %q, want substring %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
