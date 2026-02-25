package cli

import (
	"bytes"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		wantOut  string
	}{
		{
			name:    "default version",
			version: "dev",
			wantOut: "dev\n",
		},
		{
			name:    "injected version",
			version: "v1.2.3-4-gabcdef0",
			wantOut: "v1.2.3-4-gabcdef0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			original := version
			version = tt.version
			t.Cleanup(func() { version = original })

			err := writeVersion(&buf)
			if err != nil {
				t.Fatalf("writeVersion() error = %v", err)
			}

			if got := buf.String(); got != tt.wantOut {
				t.Errorf("writeVersion() output = %q, want %q", got, tt.wantOut)
			}
		})
	}
}
