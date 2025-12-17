package httputil

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestBodyTooLargeError(t *testing.T) {
	err := &BodyTooLargeError{Limit: 1024}
	expected := "body exceeds limit of 1024 bytes"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestNormalizeServiceBaseURL(t *testing.T) {
	// Save and restore environment
	savedStrict := os.Getenv("STRICT_IDENTITY_MODE")
	defer func() {
		if savedStrict != "" {
			os.Setenv("STRICT_IDENTITY_MODE", savedStrict)
		} else {
			os.Unsetenv("STRICT_IDENTITY_MODE")
		}
	}()

	t.Run("valid https URL", func(t *testing.T) {
		os.Unsetenv("STRICT_IDENTITY_MODE")
		baseURL, parsed, err := NormalizeServiceBaseURL("https://example.com/api/")
		if err != nil {
			t.Fatalf("NormalizeServiceBaseURL() error = %v", err)
		}
		if baseURL != "https://example.com/api" {
			t.Errorf("baseURL = %s, want https://example.com/api", baseURL)
		}
		if parsed == nil {
			t.Error("parsed URL should not be nil")
		}
	})

	t.Run("valid http URL in non-strict mode", func(t *testing.T) {
		os.Unsetenv("STRICT_IDENTITY_MODE")
		baseURL, _, err := NormalizeServiceBaseURL("http://localhost:8080")
		if err != nil {
			t.Fatalf("NormalizeServiceBaseURL() error = %v", err)
		}
		if baseURL != "http://localhost:8080" {
			t.Errorf("baseURL = %s, want http://localhost:8080", baseURL)
		}
	})
}

func TestConflict(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		w := httptest.NewRecorder()
		Conflict(w, "resource already exists")
		if w.Code != http.StatusConflict {
			t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
		}
	})

	t.Run("empty message uses default", func(t *testing.T) {
		w := httptest.NewRecorder()
		Conflict(w, "")
		if w.Code != http.StatusConflict {
			t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
		}
	})
}

func TestStrictIdentityMode(t *testing.T) {
	// Save and restore environment
	savedStrict := os.Getenv("STRICT_IDENTITY_MODE")
	defer func() {
		if savedStrict != "" {
			os.Setenv("STRICT_IDENTITY_MODE", savedStrict)
		} else {
			os.Unsetenv("STRICT_IDENTITY_MODE")
		}
	}()

	t.Run("disabled by default", func(t *testing.T) {
		os.Unsetenv("STRICT_IDENTITY_MODE")
		// Just verify it doesn't panic
		_ = StrictIdentityMode()
	})

	t.Run("enabled when set", func(t *testing.T) {
		os.Setenv("STRICT_IDENTITY_MODE", "true")
		// Just verify it doesn't panic
		_ = StrictIdentityMode()
	})
}

func TestCanonicalizeServiceID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"globalsigner", "globalsigner"},
		{"GLOBALSIGNER", "globalsigner"},
		{"GlobalSigner", "globalsigner"},
		{"neofeeds", "neofeeds"},
		{"neoaccounts", "neoaccounts"},
		{"txproxy", "txproxy"},
		{"unknown-service", "unknown-service"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := CanonicalizeServiceID(tt.input)
			if result != tt.expected {
				t.Errorf("CanonicalizeServiceID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeBaseURLEdgeCases(t *testing.T) {
	t.Run("empty URL", func(t *testing.T) {
		_, _, err := NormalizeBaseURL("", BaseURLOptions{})
		if err == nil {
			t.Error("expected error for empty URL")
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		_, _, err := NormalizeBaseURL("://invalid", BaseURLOptions{})
		if err == nil {
			t.Error("expected error for invalid URL")
		}
	})

	t.Run("trailing slash removed", func(t *testing.T) {
		baseURL, _, err := NormalizeBaseURL("http://example.com/api/", BaseURLOptions{})
		if err != nil {
			t.Fatalf("NormalizeBaseURL() error = %v", err)
		}
		if baseURL != "http://example.com/api" {
			t.Errorf("baseURL = %s, want http://example.com/api", baseURL)
		}
	})
}

func TestClientIPEdgeCases(t *testing.T) {
	t.Run("RemoteAddr parsing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		// httptest.NewRequest sets RemoteAddr to "192.0.2.1:1234" by default
		ip := ClientIP(req)
		if ip == "" {
			t.Error("ClientIP() should return non-empty string")
		}
	})

	t.Run("nil request", func(t *testing.T) {
		ip := ClientIP(nil)
		if ip != "" {
			t.Errorf("ClientIP(nil) = %s, want empty", ip)
		}
	})
}
