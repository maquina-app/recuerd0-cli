package commands

import "testing"

func TestIsValidCategory(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"decision", true},
		{"discovery", true},
		{"preference", true},
		{"general", true},
		{"", true},
		{"nonsense", false},
		{"Decision", false},
		{"random", false},
	}

	for _, tt := range tests {
		if got := IsValidCategory(tt.input); got != tt.want {
			t.Errorf("IsValidCategory(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
