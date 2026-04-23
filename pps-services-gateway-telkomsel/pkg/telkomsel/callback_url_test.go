package telkomsel

import "testing"

func TestGenerateCallbackURL_FromEnv(t *testing.T) {
	t.Setenv("CALLBACK_URL_TELKOMSEL", "https://azec-services-external.erajaya.com:8348/callback/ext")

	got := GenerateCallbackURL()
	if got != "https://azec-services-external.erajaya.com:8348/callback/ext" {
		t.Fatalf("GenerateCallbackURL() = %q, want %q", got, "https://azec-services-external.erajaya.com:8348/callback/ext")
	}
}

func TestGenerateCallbackURL_TrimsWhitespace(t *testing.T) {
	t.Setenv("CALLBACK_URL_TELKOMSEL", "  https://example.com/callback/ext  ")

	got := GenerateCallbackURL()
	if got != "https://example.com/callback/ext" {
		t.Fatalf("GenerateCallbackURL() = %q, want %q", got, "https://example.com/callback/ext")
	}
}

func TestGenerateCallbackURL_EmptyWhenUnset(t *testing.T) {
	t.Setenv("CALLBACK_URL_TELKOMSEL", "")

	got := GenerateCallbackURL()
	if got != "" {
		t.Fatalf("GenerateCallbackURL() = %q, want empty", got)
	}
}
