package telkomsel

import (
	"encoding/base64"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// EncryptElement1
// ---------------------------------------------------------------------------

func TestEncryptElement1(t *testing.T) {
	// A valid 16-byte key encoded as base64.
	validKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))

	tests := []struct {
		name    string
		pin     string
		key     string
		wantErr string
	}{
		{
			name: "valid PIN and key returns non-empty base64 ciphertext",
			pin:  "002314",
			key:  validKey,
		},
		{
			name:    "key decodes to != 16 bytes returns error",
			pin:     "002314",
			key:     base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr: "must decode to 16 bytes",
		},
		{
			name:    "invalid base64 key returns error",
			pin:     "002314",
			key:     "!!!not-base64!!!",
			wantErr: "base64 decode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncryptElement1(tc.pin, tc.key)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == "" {
				t.Fatal("expected non-empty ciphertext, got empty string")
			}
			// Verify output is valid base64.
			if _, decErr := base64.StdEncoding.DecodeString(got); decErr != nil {
				t.Fatalf("ciphertext is not valid base64: %v", decErr)
			}
		})
	}
}

func TestEncryptElement1_Determinism(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	pin := "002314"

	ct1, err := EncryptElement1(pin, validKey)
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	ct2, err := EncryptElement1(pin, validKey)
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if ct1 != ct2 {
		t.Fatalf("ciphertext differs between calls: %q vs %q", ct1, ct2)
	}
}

// ---------------------------------------------------------------------------
// pkcs5Pad
// ---------------------------------------------------------------------------

func TestPkcs5Pad(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		blockSize int
		wantLen   int
		wantPad   byte // expected padding byte value
	}{
		{
			name:      "empty input gets full block of padding",
			input:     []byte{},
			blockSize: 16,
			wantLen:   16,
			wantPad:   16,
		},
		{
			name:      "1 byte input padded to block size",
			input:     []byte{0x41},
			blockSize: 16,
			wantLen:   16,
			wantPad:   15,
		},
		{
			name:      "15 bytes input padded to 16",
			input:     []byte("123456789012345"),
			blockSize: 16,
			wantLen:   16,
			wantPad:   1,
		},
		{
			name:      "exact block size gets extra full block",
			input:     []byte("1234567890123456"),
			blockSize: 16,
			wantLen:   32,
			wantPad:   16,
		},
		{
			name:      "block size 8 with 3 bytes",
			input:     []byte("abc"),
			blockSize: 8,
			wantLen:   8,
			wantPad:   5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pkcs5Pad(tc.input, tc.blockSize)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			// Verify padding bytes are correct.
			padLen := int(tc.wantPad)
			for i := len(got) - padLen; i < len(got); i++ {
				if got[i] != tc.wantPad {
					t.Fatalf("padding byte at index %d = %d, want %d", i, got[i], tc.wantPad)
				}
			}
			// Verify original data is preserved.
			for i := 0; i < len(tc.input); i++ {
				if got[i] != tc.input[i] {
					t.Fatalf("data byte at index %d = %d, want %d", i, got[i], tc.input[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// EncryptElement1FromEnv
// ---------------------------------------------------------------------------

func TestEncryptElement1FromEnv_Valid(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	t.Setenv("PIN", "002314")
	t.Setenv("ENCRYPTION_KEY", validKey)

	got, err := EncryptElement1FromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty ciphertext")
	}
}

func TestEncryptElement1FromEnv_MissingPIN(t *testing.T) {
	t.Setenv("PIN", "")
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))

	_, err := EncryptElement1FromEnv()
	if err == nil {
		t.Fatal("expected error for missing PIN")
	}
	if !strings.Contains(err.Error(), "PIN") {
		t.Fatalf("error %q does not contain 'PIN'", err.Error())
	}
}

func TestEncryptElement1FromEnv_MissingEncryptionKey(t *testing.T) {
	t.Setenv("PIN", "002314")
	t.Setenv("ENCRYPTION_KEY", "")

	_, err := EncryptElement1FromEnv()
	if err == nil {
		t.Fatal("expected error for missing ENCRYPTION_KEY")
	}
	if !strings.Contains(err.Error(), "ENCRYPTION_KEY") {
		t.Fatalf("error %q does not contain 'ENCRYPTION_KEY'", err.Error())
	}
}
