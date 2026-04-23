package telkomsel

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// normalizeMSISDN
// ---------------------------------------------------------------------------

func TestNormalizeMSISDN(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "0-prefix valid", input: "081234567890", want: "6281234567890"},
		{name: "+62-prefix valid", input: "+6281234567890", want: "6281234567890"},
		{name: "62-prefix valid", input: "6281234567890", want: "6281234567890"},
		{name: "empty", input: "", wantErr: "msisdn is required"},
		{name: "whitespace only", input: "   ", wantErr: "msisdn is required"},
		{name: "too short after normalize (allowed)", input: "08123", want: "628123"},
		{name: "too long after normalize (allowed)", input: "081234567890123", want: "6281234567890123"},
		{name: "does not start with 62 after strip", input: "7112345678901", wantErr: "must start with 62"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeMSISDN(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// deriveSequence
// ---------------------------------------------------------------------------

func TestDeriveSequence(t *testing.T) {
	tests := []struct {
		name  string
		msgID string
	}{
		{name: "numeric", msgID: "12345"},
		{name: "large numeric", msgID: "999999999"},
		{name: "non-numeric", msgID: "abc-def-ghi"},
		{name: "empty", msgID: ""},
		{name: "negative numeric", msgID: "-42"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveSequence(tc.msgID)
			if got < 0 || got > 9999 {
				t.Fatalf("deriveSequence(%q) = %d, want [0, 9999]", tc.msgID, got)
			}
		})
	}
}

func TestDeriveSequence_NumericModulo(t *testing.T) {
	got := deriveSequence("12345")
	if got != 12345%10000 {
		t.Fatalf("deriveSequence(\"12345\") = %d, want %d", got, 12345%10000)
	}
}

func TestDeriveSequence_NegativeNumeric(t *testing.T) {
	got := deriveSequence("-42")
	if got != 42%10000 {
		t.Fatalf("deriveSequence(\"-42\") = %d, want %d", got, 42)
	}
}

// ---------------------------------------------------------------------------
// buildTelkomselTransactionID
// ---------------------------------------------------------------------------

func TestBuildTelkomselTransactionID(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 45, 123000000, time.UTC)

	tests := []struct {
		name    string
		orgCode string
		seq     int
	}{
		{name: "normal 6-char org", orgCode: "705009", seq: 42},
		{name: "short org padded", orgCode: "AB", seq: 0},
		{name: "long org truncated", orgCode: "ABCDEFGHIJ", seq: 9999},
		{name: "negative seq", orgCode: "ORG123", seq: -5},
		{name: "seq exceeds 10000", orgCode: "ORG123", seq: 12345},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildTelkomselTransactionID(tc.orgCode, ts, tc.seq)
			if len(got) != 25 {
				t.Fatalf("len(%q) = %d, want 25", got, len(got))
			}
		})
	}
}

func TestBuildTelkomselTransactionID_Segments(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 45, 123000000, time.UTC)
	got := buildTelkomselTransactionID("705009", ts, 42)

	// org prefix: 6 chars
	orgPart := got[:6]
	if orgPart != "705009" {
		t.Fatalf("org part = %q, want %q", orgPart, "705009")
	}

	// timestamp: 15 chars (YYMMDDHHmmSSmmm)
	tsPart := got[6:21]
	if len(tsPart) != 15 {
		t.Fatalf("timestamp part len = %d, want 15", len(tsPart))
	}

	// sequence: 4 chars
	seqPart := got[21:25]
	if seqPart != "0042" {
		t.Fatalf("seq part = %q, want %q", seqPart, "0042")
	}
}

// ---------------------------------------------------------------------------
// organizationCodeAndPINFromMIDEnv
// ---------------------------------------------------------------------------

func TestOrganizationCodeAndPINFromMIDEnv(t *testing.T) {
	tests := []struct {
		name    string
		mid     string
		envKey  string
		envVal  string
		wantOrg string
		wantPIN string
		wantErr string
	}{
		{
			name: "valid", mid: "swpps", envKey: "swpps", envVal: "705009_002314",
			wantOrg: "705009", wantPIN: "002314",
		},
		{
			name: "missing env", mid: "unknown_mid", envKey: "", envVal: "",
			wantErr: "is required",
		},
		{
			name: "malformed no underscore", mid: "badmid", envKey: "badmid", envVal: "NOUNDERSCORE",
			wantErr: "must be in format",
		},
		{
			name: "empty mid", mid: "", envKey: "", envVal: "",
			wantErr: "mid is required",
		},
		{
			name: "empty org code", mid: "emptyorg", envKey: "emptyorg", envVal: "_somepin",
			wantErr: "organization code is required",
		},
		{
			name: "empty pin", mid: "emptypin", envKey: "emptypin", envVal: "ORG_",
			wantErr: "pin is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envKey != "" {
				t.Setenv(tc.envKey, tc.envVal)
			}
			// Ensure unknown_mid variants are unset
			if tc.mid == "unknown_mid" {
				t.Setenv("unknown_mid", "")
				t.Setenv("UNKNOWN_MID", "")
			}

			orgCode, pin, err := organizationCodeAndPINFromMIDEnv(tc.mid)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if orgCode != tc.wantOrg {
				t.Fatalf("orgCode = %q, want %q", orgCode, tc.wantOrg)
			}
			if pin != tc.wantPIN {
				t.Fatalf("pin = %q, want %q", pin, tc.wantPIN)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateSignature
// ---------------------------------------------------------------------------

func TestGenerateSignature(t *testing.T) {
	t.Run("valid env vars returns 32-char hex", func(t *testing.T) {
		t.Setenv("API_KEY", "test-api-key")
		t.Setenv("SECRET_KEY", "test-secret-key")

		sig, err := generateSignature("1700000000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sig) != 32 {
			t.Fatalf("len(sig) = %d, want 32", len(sig))
		}
		// Verify it's valid hex
		for _, c := range sig {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("signature contains non-hex char: %c", c)
			}
		}
	})

	t.Run("missing API_KEY returns error", func(t *testing.T) {
		t.Setenv("API_KEY", "")
		t.Setenv("SECRET_KEY", "test-secret-key")

		_, err := generateSignature("1700000000")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "API_KEY") {
			t.Fatalf("error %q does not contain API_KEY", err.Error())
		}
	})

	t.Run("missing SECRET_KEY returns error", func(t *testing.T) {
		t.Setenv("API_KEY", "test-api-key")
		t.Setenv("SECRET_KEY", "")

		_, err := generateSignature("1700000000")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "SECRET_KEY") {
			t.Fatalf("error %q does not contain SECRET_KEY", err.Error())
		}
	})

	t.Run("empty timestamp returns error", func(t *testing.T) {
		t.Setenv("API_KEY", "test-api-key")
		t.Setenv("SECRET_KEY", "test-secret-key")

		_, err := generateSignature("")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "timestamp") {
			t.Fatalf("error %q does not contain 'timestamp'", err.Error())
		}
	})

	t.Run("deterministic output", func(t *testing.T) {
		t.Setenv("API_KEY", "key1")
		t.Setenv("SECRET_KEY", "secret1")

		sig1, err := generateSignature("12345")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sig2, err := generateSignature("12345")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sig1 != sig2 {
			t.Fatalf("signatures differ: %q vs %q", sig1, sig2)
		}
	})
}

// ---------------------------------------------------------------------------
// sanitizeHeadersForLog
// ---------------------------------------------------------------------------

func TestSanitizeHeadersForLog(t *testing.T) {
	t.Run("masks sensitive keys", func(t *testing.T) {
		h := http.Header{}
		h.Set("api-key", "super-secret-api-key-value")
		h.Set("x-signature", "super-secret-signature-value")
		h.Set("Authorization", "Bearer super-secret-token-value")
		h.Set("Channel-Id", "tp")

		got := sanitizeHeadersForLog(h)

		// Sensitive keys should be masked
		if got["Api-Key"] == "super-secret-api-key-value" {
			t.Fatal("api-key should be masked")
		}
		if got["X-Signature"] == "super-secret-signature-value" {
			t.Fatal("x-signature should be masked")
		}
		if got["Authorization"] == "Bearer super-secret-token-value" {
			t.Fatal("authorization should be masked")
		}

		// Non-sensitive keys should pass through
		if got["Channel-Id"] != "tp" {
			t.Fatalf("Channel-Id = %q, want %q", got["Channel-Id"], "tp")
		}
	})

	t.Run("nil header returns nil", func(t *testing.T) {
		got := sanitizeHeadersForLog(nil)
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("empty header returns empty map", func(t *testing.T) {
		got := sanitizeHeadersForLog(http.Header{})
		if got == nil {
			t.Fatal("expected non-nil map")
		}
		if len(got) != 0 {
			t.Fatalf("expected empty map, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// sanitizeJSONForLog
// ---------------------------------------------------------------------------

func TestSanitizeJSONForLog(t *testing.T) {
	t.Run("redacts third_party_password", func(t *testing.T) {
		body := []byte(`{"merchant_profile":{"third_party_password":"secret123","third_party_id":"NGRS"}}`)
		got := sanitizeJSONForLog(body)
		if contains(got, "secret123") {
			t.Fatalf("third_party_password not redacted: %s", got)
		}
		if !contains(got, `"***"`) {
			t.Fatalf("expected *** replacement in: %s", got)
		}
	})

	t.Run("redacts element1", func(t *testing.T) {
		body := []byte(`{"recharge":{"element1":"encrypted-value","amount":10000}}`)
		got := sanitizeJSONForLog(body)
		if contains(got, "encrypted-value") {
			t.Fatalf("element1 not redacted: %s", got)
		}
		if !contains(got, `"***"`) {
			t.Fatalf("expected *** replacement in: %s", got)
		}
	})

	t.Run("non-sensitive fields pass through", func(t *testing.T) {
		body := []byte(`{"transaction":{"transaction_id":"TX001","channel":"tp"}}`)
		got := sanitizeJSONForLog(body)
		if !contains(got, "TX001") {
			t.Fatalf("transaction_id should pass through: %s", got)
		}
		if !contains(got, "tp") {
			t.Fatalf("channel should pass through: %s", got)
		}
	})

	t.Run("empty body returns empty string", func(t *testing.T) {
		got := sanitizeJSONForLog(nil)
		if got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
		got = sanitizeJSONForLog([]byte{})
		if got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})

	t.Run("invalid JSON returns truncated raw string", func(t *testing.T) {
		body := []byte(`not-json-at-all`)
		got := sanitizeJSONForLog(body)
		if got != "not-json-at-all" {
			t.Fatalf("expected raw string, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// classifyError
// ---------------------------------------------------------------------------

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantType    string
		wantMessage string
	}{
		{
			name:        "nil error",
			err:         nil,
			wantType:    "",
			wantMessage: "",
		},
		{
			name:        "BusinessError",
			err:         &BusinessError{Code: "10001", Message: "insufficient balance", TransactionID: "TX1"},
			wantType:    "business",
			wantMessage: "business error: code=10001 message=insufficient balance transactionId=TX1",
		},
		{
			name:        "TechnicalError",
			err:         &TechnicalError{StatusCode: 500, Cause: errors.New("server down")},
			wantType:    "technical",
			wantMessage: "technical error: status=500 cause=server down",
		},
		{
			name:        "generic error",
			err:         errors.New("something went wrong"),
			wantType:    "technical",
			wantMessage: "something went wrong",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotMsg := classifyError(tc.err)
			if gotType != tc.wantType {
				t.Fatalf("type = %q, want %q", gotType, tc.wantType)
			}
			if gotMsg != tc.wantMessage {
				t.Fatalf("message = %q, want %q", gotMsg, tc.wantMessage)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isRetryableError
// ---------------------------------------------------------------------------

// mockNetError implements net.Error for testing.
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "BusinessError is not retryable",
			err:  &BusinessError{Code: "10001", Message: "fail"},
			want: false,
		},
		{
			name: "TechnicalError with 500 is retryable",
			err:  &TechnicalError{StatusCode: 500, Cause: errors.New("server error")},
			want: true,
		},
		{
			name: "TechnicalError with 502 is retryable",
			err:  &TechnicalError{StatusCode: 502, Cause: errors.New("bad gateway")},
			want: true,
		},
		{
			name: "TechnicalError with 400 is not retryable",
			err:  &TechnicalError{StatusCode: 400, Cause: errors.New("bad request")},
			want: false,
		},
		{
			name: "TechnicalError wrapping net.Error is retryable",
			err:  &TechnicalError{Cause: &mockNetError{timeout: true}},
			want: true,
		},
		{
			name: "TechnicalError wrapping context.DeadlineExceeded is retryable",
			err:  &TechnicalError{Cause: context.DeadlineExceeded},
			want: true,
		},
		{
			name: "TechnicalError wrapping context.Canceled is retryable",
			err:  &TechnicalError{Cause: context.Canceled},
			want: true,
		},
		{
			name: "generic error is not retryable",
			err:  errors.New("random error"),
			want: false,
		},
		{
			name: "nil is not retryable",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetryableError(tc.err)
			if got != tc.want {
				t.Fatalf("isRetryableError() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// maskForLog
// ---------------------------------------------------------------------------

func TestMaskForLog(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},
		{name: "short string <= 8 chars", input: "abcdefgh", want: "***"},
		{name: "exactly 8 chars", input: "12345678", want: "***"},
		{name: "longer than 8 chars", input: "1234567890", want: "1234...7890"},
		{name: "long value", input: "super-secret-api-key-value", want: "supe...alue"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := maskForLog(tc.input)
			if got != tc.want {
				t.Fatalf("maskForLog(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncateForLog
// ---------------------------------------------------------------------------

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{name: "within limit", input: "hello", max: 10, want: "hello"},
		{name: "at limit", input: "hello", max: 5, want: "hello"},
		{name: "exceeds limit", input: "hello world", max: 5, want: "hello..."},
		{name: "zero max returns original", input: "hello", max: 0, want: "hello"},
		{name: "negative max returns original", input: "hello", max: -1, want: "hello"},
		{name: "empty string", input: "", max: 10, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateForLog(tc.input, tc.max)
			if got != tc.want {
				t.Fatalf("truncateForLog(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) > 0 && containsStr(s, substr)))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Verify mockNetError satisfies net.Error at compile time.
var _ net.Error = (*mockNetError)(nil)

func TestSanitizeJSONForLog_WithNestedArray(t *testing.T) {
	input := []byte(`[{"third_party_password":"secret123","name":"test"},{"element1":"enc456"}]`)
	got := sanitizeJSONForLog(input)
	if strings.Contains(got, "secret123") {
		t.Error("expected third_party_password to be redacted")
	}
	if strings.Contains(got, "enc456") {
		t.Error("expected element1 to be redacted")
	}
	if !strings.Contains(got, `"***"`) {
		t.Error("expected redacted values to contain ***")
	}
}

func TestSanitizeJSONForLog_WithNestedObjects(t *testing.T) {
	input := []byte(`{"outer":{"third_party_password":"secret","normal":"value"}}`)
	got := sanitizeJSONForLog(input)
	if strings.Contains(got, "secret") {
		t.Error("expected nested third_party_password to be redacted")
	}
	if !strings.Contains(got, "value") {
		t.Error("expected normal field to be preserved")
	}
}

func TestGenerateCallbackURL_ReturnsEmpty(t *testing.T) {
	got := GenerateCallbackURL()
	if got != "" {
		t.Errorf("GenerateCallbackURL() = %q, want empty string", got)
	}
}

func TestInitiatePaketDataOnConsume_ReturnsNil(t *testing.T) {
	err := InitiatePaketDataOnConsume(context.Background(), "628123456789", "mid", "q", "msg", "pc", 10000)
	if err != nil {
		t.Errorf("InitiatePaketDataOnConsume() = %v, want nil", err)
	}
}
