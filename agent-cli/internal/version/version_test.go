package version

import "testing"

func TestInfo(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "returns default dev version", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Info()
			if got != tt.want {
				t.Errorf("Info() = %q, want %q", got, tt.want)
			}
		})
	}
}
