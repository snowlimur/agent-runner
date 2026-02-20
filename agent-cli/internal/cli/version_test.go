package cli

import (
	"bytes"
	"errors"
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

// errWriter is a writer that always returns an error.
type errWriter struct {
	err error
}

func (w *errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestVersionCommandWriteError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("disk full")
	err := VersionCommand(&errWriter{err: writeErr})
	if err == nil {
		t.Fatal("VersionCommand() expected error when writer fails, got nil")
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("VersionCommand() error = %v, want wrapped %v", err, writeErr)
	}
}
