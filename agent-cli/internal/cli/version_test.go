package cli

import (
	"bytes"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantOut string
	}{
		{name: "prints version string", args: []string{}, wantOut: "agent-cli version dev\n"},
		{name: "ignores extra args", args: []string{"--json"}, wantOut: "agent-cli version dev\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			err := VersionCommand(&buf, tt.args)
			if err != nil {
				t.Fatalf("VersionCommand() error = %v, want nil", err)
			}

			if got := buf.String(); got != tt.wantOut {
				t.Errorf("VersionCommand() output = %q, want %q", got, tt.wantOut)
			}
		})
	}
}
