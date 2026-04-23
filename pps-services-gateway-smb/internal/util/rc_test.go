package util

import "testing"

func TestResolveRCPPS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"success", "00", 0},
		{"pending 28", "28", 9},
		{"pending 68", "68", 9},
		{"empty string", "", 9},
		{"failed 93", "93", 1},
		{"failed 94", "94", 1},
		{"failed 97", "97", 1},
		{"failed 98", "98", 1},
		{"failed 99", "99", 1},
		{"failed random", "XYZ", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveRCPPS(tt.input)
			if got != tt.expected {
				t.Errorf("ResolveRCPPS(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestStatusToBeFromRC(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"success", 0, "F"},
		{"failed", 1, "C"},
		{"pending", 9, "S"},
		{"unknown", 99, "C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusToBeFromRC(tt.input)
			if got != tt.expected {
				t.Errorf("StatusToBeFromRC(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
