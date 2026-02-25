package cli_test

import (
	"bytes"
	"testing"

	"agent-cli/internal/cli"
)

func TestVersionCommand(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"provided version", "v1.2.3", "v1.2.3\n"},
		{"dev fallback", "dev", "dev\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := cli.VersionCommand(&buf, tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := buf.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
