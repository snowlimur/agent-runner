package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	t.Parallel()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	if err := VersionCommand(nil); err != nil {
		os.Stdout = old
		t.Fatalf("VersionCommand returned error: %v", err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	got := strings.TrimSpace(buf.String())
	if got != Version {
		t.Fatalf("expected %q, got %q", Version, got)
	}
}

func TestVersionCommandRejectsArgs(t *testing.T) {
	t.Parallel()

	err := VersionCommand([]string{"extra"})
	if err == nil {
		t.Fatal("expected error when passing arguments")
	}
}
