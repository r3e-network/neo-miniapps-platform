package database

import "testing"

func TestNewClient_AllowsHTTPInNonStrictMode(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")
	t.Setenv("OE_SIMULATION", "1")
	t.Setenv("MARBLE_CERT", "")
	t.Setenv("MARBLE_KEY", "")
	t.Setenv("MARBLE_ROOT_CA", "")

	_, err := NewClient(Config{
		URL:        "http://localhost:54321",
		ServiceKey: "test",
	})
	if err != nil {
		t.Fatalf("expected http SUPABASE_URL to be allowed in non-strict mode, got err: %v", err)
	}
}

func TestNewClient_StrictModeRejectsNonHTTPS(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")
	t.Setenv("OE_SIMULATION", "1")
	t.Setenv("MARBLE_CERT", "")
	t.Setenv("MARBLE_KEY", "")
	t.Setenv("MARBLE_ROOT_CA", "")

	_, err := NewClient(Config{
		URL:        "http://example.com",
		ServiceKey: "test",
	})
	if err == nil {
		t.Fatal("expected error for http SUPABASE_URL in strict mode, got nil")
	}
}

func TestNewClient_StrictModeRejectsUserInfo(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")
	t.Setenv("OE_SIMULATION", "1")
	t.Setenv("MARBLE_CERT", "")
	t.Setenv("MARBLE_KEY", "")
	t.Setenv("MARBLE_ROOT_CA", "")

	_, err := NewClient(Config{
		URL:        "https://user:pass@example.com",
		ServiceKey: "test",
	})
	if err == nil {
		t.Fatal("expected error for SUPABASE_URL with user info, got nil")
	}
}
