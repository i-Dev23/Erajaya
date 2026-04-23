package postgres

import (
	"encoding/json"
	"testing"
)

func TestNullIfEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  any
	}{
		{name: "empty string returns nil", input: "", want: nil},
		{name: "non-empty string returns string", input: "hello", want: "hello"},
		{name: "whitespace-only string returns string", input: " ", want: " "},
		{name: "single char returns string", input: "x", want: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullIfEmpty(tt.input)
			if got != tt.want {
				t.Errorf("nullIfEmpty(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNullIfZero(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  any
	}{
		{name: "zero returns nil", input: 0, want: nil},
		{name: "positive returns integer", input: 42, want: 42},
		{name: "negative returns integer", input: -1, want: -1},
		{name: "large positive returns integer", input: 999999, want: 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullIfZero(tt.input)
			if got != tt.want {
				t.Errorf("nullIfZero(%d) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestToRawJSONB(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		want      []byte
		wantValid bool // whether the output should be valid JSON
	}{
		{
			name:  "nil slice returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty slice returns nil",
			input: []byte{},
			want:  nil,
		},
		{
			name:      "valid JSON object returns unchanged",
			input:     []byte(`{"key":"value"}`),
			want:      []byte(`{"key":"value"}`),
			wantValid: true,
		},
		{
			name:      "valid JSON array returns unchanged",
			input:     []byte(`[1,2,3]`),
			want:      []byte(`[1,2,3]`),
			wantValid: true,
		},
		{
			name:      "valid JSON string returns unchanged",
			input:     []byte(`"hello"`),
			want:      []byte(`"hello"`),
			wantValid: true,
		},
		{
			name:      "invalid JSON returns JSON-wrapped string",
			input:     []byte(`not json`),
			wantValid: true,
		},
		{
			name:      "XML-like content returns JSON-wrapped string",
			input:     []byte(`<xml>data</xml>`),
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toRawJSONB(tt.input)

			if tt.want != nil {
				if string(got) != string(tt.want) {
					t.Errorf("toRawJSONB(%q) = %q, want %q", tt.input, got, tt.want)
				}
			} else if tt.input == nil || len(tt.input) == 0 {
				if got != nil {
					t.Errorf("toRawJSONB(%q) = %q, want nil", tt.input, got)
				}
			}

			if tt.wantValid && got != nil && !json.Valid(got) {
				t.Errorf("toRawJSONB(%q) = %q, expected valid JSON", tt.input, got)
			}
		})
	}

	// Verify invalid JSON wrapping produces the original string as a JSON string value
	t.Run("invalid JSON wrapping preserves content", func(t *testing.T) {
		input := []byte("not json")
		got := toRawJSONB(input)

		var unwrapped string
		if err := json.Unmarshal(got, &unwrapped); err != nil {
			t.Fatalf("failed to unmarshal wrapped result: %v", err)
		}
		if unwrapped != "not json" {
			t.Errorf("unwrapped = %q, want %q", unwrapped, "not json")
		}
	})
}

func TestToJSONB(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantNil bool
		wantErr bool
	}{
		{
			name:    "nil returns nil bytes and nil error",
			input:   nil,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "string value returns JSON bytes",
			input:   "hello",
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "map value returns JSON bytes",
			input:   map[string]string{"key": "value"},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "int value returns JSON bytes",
			input:   42,
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "slice value returns JSON bytes",
			input:   []int{1, 2, 3},
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toJSONB(tt.input)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantNil && got != nil {
				t.Errorf("expected nil bytes, got %q", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("expected non-nil bytes, got nil")
			}

			if !tt.wantNil && got != nil && !json.Valid(got) {
				t.Errorf("toJSONB(%v) produced invalid JSON: %q", tt.input, got)
			}
		})
	}
}
