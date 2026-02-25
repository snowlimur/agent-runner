package cli_test

import (
	"bytes"
	"testing"

	"agent-cli/internal/cli"
)

//TODO(agent): T-1 (MUST) Refactor to table-driven test. The two subtests share identical
// structure and should be collapsed into a single loop over a slice of struct{name, version, want string}.
// Example:
//   tests := []struct{ name, version, want string }{
//       {"provided version", "v1.2.3", "v1.2.3\n"},
//       {"dev fallback",     "dev",    "dev\n"},
//   }
//   for _, tt := range tests {
//       t.Run(tt.name, func(t *testing.T) { ... })
//   }
func TestVersionCommand(t *testing.T) {
	t.Run("prints provided version", func(t *testing.T) {
		var buf bytes.Buffer
		err := cli.VersionCommand(&buf, "v1.2.3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := buf.String()
		want := "v1.2.3\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("prints dev fallback", func(t *testing.T) {
		var buf bytes.Buffer
		err := cli.VersionCommand(&buf, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := buf.String()
		want := "dev\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
