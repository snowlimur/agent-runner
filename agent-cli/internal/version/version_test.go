package version

import "testing"

func TestVersionDefault(t *testing.T) {
	t.Parallel()

	got := Version()
	if got != "dev" {
		t.Fatalf("Version() = %q, want %q", got, "dev")
	}
}
