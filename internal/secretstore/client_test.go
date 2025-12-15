package secretstore

import "testing"

func TestNew_AllowsHTTPInNonStrictMode(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")
	t.Setenv("OE_SIMULATION", "1")
	t.Setenv("MARBLE_CERT", "")
	t.Setenv("MARBLE_KEY", "")
	t.Setenv("MARBLE_ROOT_CA", "")

	if _, err := New(Config{BaseURL: "http://neostore:8087"}); err != nil {
		t.Fatalf("expected http base URL to be allowed in non-strict mode, got err: %v", err)
	}
}

func TestNew_RequiresHTTPSInStrictMode(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")
	t.Setenv("OE_SIMULATION", "1")
	t.Setenv("MARBLE_CERT", "")
	t.Setenv("MARBLE_KEY", "")
	t.Setenv("MARBLE_ROOT_CA", "")

	if _, err := New(Config{BaseURL: "http://neostore:8087"}); err == nil {
		t.Fatal("expected error for http base URL in strict mode, got nil")
	}
	if _, err := New(Config{BaseURL: "https://neostore:8087"}); err != nil {
		t.Fatalf("expected https base URL to be allowed in strict mode, got err: %v", err)
	}
}

func TestNew_RejectsUserInfo(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")
	t.Setenv("OE_SIMULATION", "1")
	t.Setenv("MARBLE_CERT", "")
	t.Setenv("MARBLE_KEY", "")
	t.Setenv("MARBLE_ROOT_CA", "")

	if _, err := New(Config{BaseURL: "https://user:pass@neostore:8087"}); err == nil {
		t.Fatal("expected error for base URL with user info, got nil")
	}
}
