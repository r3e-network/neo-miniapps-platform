package httputil

import "testing"

func TestNormalizeBaseURL_TrimsAndParses(t *testing.T) {
	got, parsed, err := NormalizeBaseURL(" https://example.com/ ", BaseURLOptions{})
	if err != nil {
		t.Fatalf("NormalizeBaseURL() error = %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("NormalizeBaseURL() = %q, want %q", got, "https://example.com")
	}
	if parsed == nil || parsed.Scheme != "https" || parsed.Host != "example.com" {
		t.Fatalf("parsed = %#v, want https://example.com", parsed)
	}
}

func TestNormalizeBaseURL_RejectsUserInfo(t *testing.T) {
	_, _, err := NormalizeBaseURL("https://user:pass@example.com", BaseURLOptions{})
	if err == nil {
		t.Fatal("NormalizeBaseURL() expected error")
	}
}

func TestNormalizeBaseURL_StrictModeRequiresHTTPS(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")
	t.Setenv("OE_SIMULATION", "1")

	_, _, err := NormalizeBaseURL("http://example.com", BaseURLOptions{RequireHTTPSInStrictMode: true})
	if err == nil {
		t.Fatal("NormalizeBaseURL() expected error in strict mode for http URL")
	}

	_, _, err = NormalizeBaseURL("https://example.com", BaseURLOptions{RequireHTTPSInStrictMode: true})
	if err != nil {
		t.Fatalf("NormalizeBaseURL() error = %v", err)
	}
}
