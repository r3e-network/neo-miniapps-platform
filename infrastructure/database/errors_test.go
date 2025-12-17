package database

import (
	"errors"
	"strings"
	"testing"
)

func TestNotFoundError(t *testing.T) {
	t.Run("Error with ID", func(t *testing.T) {
		err := &NotFoundError{Entity: "user", ID: "123"}
		expected := "user with id '123' not found"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Error without ID", func(t *testing.T) {
		err := &NotFoundError{Entity: "user", ID: ""}
		expected := "user not found"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Unwrap returns ErrNotFound", func(t *testing.T) {
		err := &NotFoundError{Entity: "user", ID: "123"}
		if err.Unwrap() != ErrNotFound {
			t.Error("Unwrap() should return ErrNotFound")
		}
	})

	t.Run("errors.Is works with NotFoundError", func(t *testing.T) {
		err := &NotFoundError{Entity: "user", ID: "123"}
		if !errors.Is(err, ErrNotFound) {
			t.Error("errors.Is should return true for ErrNotFound")
		}
	})
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("wallet", "abc-123")
	if err == nil {
		t.Fatal("NewNotFoundError() returned nil")
	}

	nfe, ok := err.(*NotFoundError)
	if !ok {
		t.Fatal("NewNotFoundError() should return *NotFoundError")
	}
	if nfe.Entity != "wallet" {
		t.Errorf("Entity = %q, want %q", nfe.Entity, "wallet")
	}
	if nfe.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", nfe.ID, "abc-123")
	}
}

func TestIsNotFound(t *testing.T) {
	t.Run("true for ErrNotFound", func(t *testing.T) {
		if !IsNotFound(ErrNotFound) {
			t.Error("IsNotFound(ErrNotFound) should return true")
		}
	})

	t.Run("true for wrapped NotFoundError", func(t *testing.T) {
		err := NewNotFoundError("user", "123")
		if !IsNotFound(err) {
			t.Error("IsNotFound should return true for NotFoundError")
		}
	})

	t.Run("false for other errors", func(t *testing.T) {
		if IsNotFound(ErrAlreadyExists) {
			t.Error("IsNotFound should return false for ErrAlreadyExists")
		}
	})

	t.Run("false for nil", func(t *testing.T) {
		if IsNotFound(nil) {
			t.Error("IsNotFound(nil) should return false")
		}
	})
}

func TestIsAlreadyExists(t *testing.T) {
	t.Run("true for ErrAlreadyExists", func(t *testing.T) {
		if !IsAlreadyExists(ErrAlreadyExists) {
			t.Error("IsAlreadyExists(ErrAlreadyExists) should return true")
		}
	})

	t.Run("false for other errors", func(t *testing.T) {
		if IsAlreadyExists(ErrNotFound) {
			t.Error("IsAlreadyExists should return false for ErrNotFound")
		}
	})

	t.Run("false for nil", func(t *testing.T) {
		if IsAlreadyExists(nil) {
			t.Error("IsAlreadyExists(nil) should return false")
		}
	})
}

func TestIsUnauthorized(t *testing.T) {
	t.Run("true for ErrUnauthorized", func(t *testing.T) {
		if !IsUnauthorized(ErrUnauthorized) {
			t.Error("IsUnauthorized(ErrUnauthorized) should return true")
		}
	})

	t.Run("false for other errors", func(t *testing.T) {
		if IsUnauthorized(ErrNotFound) {
			t.Error("IsUnauthorized should return false for ErrNotFound")
		}
	})

	t.Run("false for nil", func(t *testing.T) {
		if IsUnauthorized(nil) {
			t.Error("IsUnauthorized(nil) should return false")
		}
	})
}

func TestIsInvalidInput(t *testing.T) {
	t.Run("true for ErrInvalidInput", func(t *testing.T) {
		if !IsInvalidInput(ErrInvalidInput) {
			t.Error("IsInvalidInput(ErrInvalidInput) should return true")
		}
	})

	t.Run("false for other errors", func(t *testing.T) {
		if IsInvalidInput(ErrNotFound) {
			t.Error("IsInvalidInput should return false for ErrNotFound")
		}
	})

	t.Run("false for nil", func(t *testing.T) {
		if IsInvalidInput(nil) {
			t.Error("IsInvalidInput(nil) should return false")
		}
	})
}

func TestValidateID(t *testing.T) {
	t.Run("valid UUID with hyphens", func(t *testing.T) {
		err := ValidateID("550e8400-e29b-41d4-a716-446655440000")
		if err != nil {
			t.Errorf("ValidateID() error = %v for valid UUID", err)
		}
	})

	t.Run("valid UUID without hyphens", func(t *testing.T) {
		err := ValidateID("550e8400e29b41d4a716446655440000")
		if err != nil {
			t.Errorf("ValidateID() error = %v for valid UUID without hyphens", err)
		}
	})

	t.Run("valid alphanumeric", func(t *testing.T) {
		err := ValidateID("user_123-abc")
		if err != nil {
			t.Errorf("ValidateID() error = %v for valid alphanumeric", err)
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		err := ValidateID("")
		if err == nil {
			t.Error("ValidateID() should return error for empty ID")
		}
		if !IsInvalidInput(err) {
			t.Error("error should be ErrInvalidInput")
		}
	})

	t.Run("ID too long", func(t *testing.T) {
		longID := strings.Repeat("a", 129)
		err := ValidateID(longID)
		if err == nil {
			t.Error("ValidateID() should return error for ID > 128 chars")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		err := ValidateID("invalid@id!")
		if err == nil {
			t.Error("ValidateID() should return error for invalid format")
		}
	})
}

func TestValidateUserID(t *testing.T) {
	t.Run("valid user ID", func(t *testing.T) {
		err := ValidateUserID("user-123")
		if err != nil {
			t.Errorf("ValidateUserID() error = %v", err)
		}
	})

	t.Run("empty user ID", func(t *testing.T) {
		err := ValidateUserID("")
		if err == nil {
			t.Error("ValidateUserID() should return error for empty user ID")
		}
		if !strings.Contains(err.Error(), "user_id") {
			t.Error("error message should mention user_id")
		}
	})
}

func TestValidateAddress(t *testing.T) {
	t.Run("valid Neo address", func(t *testing.T) {
		// Neo N3 addresses start with N and are 34 characters
		err := ValidateAddress("NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN")
		if err != nil {
			t.Errorf("ValidateAddress() error = %v for valid address", err)
		}
	})

	t.Run("empty address", func(t *testing.T) {
		err := ValidateAddress("")
		if err == nil {
			t.Error("ValidateAddress() should return error for empty address")
		}
	})

	t.Run("invalid format - wrong prefix", func(t *testing.T) {
		err := ValidateAddress("AXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN")
		if err == nil {
			t.Error("ValidateAddress() should return error for wrong prefix")
		}
	})

	t.Run("invalid format - wrong length", func(t *testing.T) {
		err := ValidateAddress("NXV7ZhHiyM1aHXwp")
		if err == nil {
			t.Error("ValidateAddress() should return error for wrong length")
		}
	})
}

func TestValidateEmail(t *testing.T) {
	t.Run("valid email", func(t *testing.T) {
		err := ValidateEmail("user@example.com")
		if err != nil {
			t.Errorf("ValidateEmail() error = %v for valid email", err)
		}
	})

	t.Run("empty email is valid (optional)", func(t *testing.T) {
		err := ValidateEmail("")
		if err != nil {
			t.Errorf("ValidateEmail() should return nil for empty email, got %v", err)
		}
	})

	t.Run("invalid email - no @", func(t *testing.T) {
		err := ValidateEmail("userexample.com")
		if err == nil {
			t.Error("ValidateEmail() should return error for email without @")
		}
	})

	t.Run("invalid email - no domain", func(t *testing.T) {
		err := ValidateEmail("user@")
		if err == nil {
			t.Error("ValidateEmail() should return error for email without domain")
		}
	})

	t.Run("valid email with subdomain", func(t *testing.T) {
		err := ValidateEmail("user@mail.example.com")
		if err != nil {
			t.Errorf("ValidateEmail() error = %v for valid email with subdomain", err)
		}
	})
}

func TestValidateLimit(t *testing.T) {
	t.Run("returns default for zero", func(t *testing.T) {
		result := ValidateLimit(0, 50, 1000)
		if result != 50 {
			t.Errorf("ValidateLimit(0, 50, 1000) = %d, want 50", result)
		}
	})

	t.Run("returns default for negative", func(t *testing.T) {
		result := ValidateLimit(-10, 50, 1000)
		if result != 50 {
			t.Errorf("ValidateLimit(-10, 50, 1000) = %d, want 50", result)
		}
	})

	t.Run("returns max for over limit", func(t *testing.T) {
		result := ValidateLimit(2000, 50, 1000)
		if result != 1000 {
			t.Errorf("ValidateLimit(2000, 50, 1000) = %d, want 1000", result)
		}
	})

	t.Run("returns value when valid", func(t *testing.T) {
		result := ValidateLimit(100, 50, 1000)
		if result != 100 {
			t.Errorf("ValidateLimit(100, 50, 1000) = %d, want 100", result)
		}
	})
}

func TestValidateOffset(t *testing.T) {
	t.Run("returns 0 for negative", func(t *testing.T) {
		result := ValidateOffset(-10)
		if result != 0 {
			t.Errorf("ValidateOffset(-10) = %d, want 0", result)
		}
	})

	t.Run("returns value for zero", func(t *testing.T) {
		result := ValidateOffset(0)
		if result != 0 {
			t.Errorf("ValidateOffset(0) = %d, want 0", result)
		}
	})

	t.Run("returns value for positive", func(t *testing.T) {
		result := ValidateOffset(100)
		if result != 100 {
			t.Errorf("ValidateOffset(100) = %d, want 100", result)
		}
	})
}

func TestSanitizeString(t *testing.T) {
	t.Run("removes null bytes", func(t *testing.T) {
		result := SanitizeString("hello\x00world")
		if result != "helloworld" {
			t.Errorf("SanitizeString() = %q, want %q", result, "helloworld")
		}
	})

	t.Run("removes control characters", func(t *testing.T) {
		result := SanitizeString("hello\x01\x02world")
		if result != "helloworld" {
			t.Errorf("SanitizeString() = %q, want %q", result, "helloworld")
		}
	})

	t.Run("preserves tabs", func(t *testing.T) {
		result := SanitizeString("hello\tworld")
		if result != "hello\tworld" {
			t.Errorf("SanitizeString() = %q, want %q", result, "hello\tworld")
		}
	})

	t.Run("preserves newlines", func(t *testing.T) {
		result := SanitizeString("hello\nworld")
		if result != "hello\nworld" {
			t.Errorf("SanitizeString() = %q, want %q", result, "hello\nworld")
		}
	})

	t.Run("preserves carriage returns", func(t *testing.T) {
		result := SanitizeString("hello\rworld")
		if result != "hello\rworld" {
			t.Errorf("SanitizeString() = %q, want %q", result, "hello\rworld")
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		result := SanitizeString("  hello world  ")
		if result != "hello world" {
			t.Errorf("SanitizeString() = %q, want %q", result, "hello world")
		}
	})
}

func TestValidateTxHash(t *testing.T) {
	t.Run("valid hex hash", func(t *testing.T) {
		err := ValidateTxHash("0xabcdef1234567890")
		if err != nil {
			t.Errorf("ValidateTxHash() error = %v for valid hash", err)
		}
	})

	t.Run("valid hex hash without 0x", func(t *testing.T) {
		err := ValidateTxHash("abcdef1234567890")
		if err != nil {
			t.Errorf("ValidateTxHash() error = %v for valid hash without 0x", err)
		}
	})

	t.Run("empty hash", func(t *testing.T) {
		err := ValidateTxHash("")
		if err == nil {
			t.Error("ValidateTxHash() should return error for empty hash")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		err := ValidateTxHash("not-a-hex-hash!")
		if err == nil {
			t.Error("ValidateTxHash() should return error for invalid format")
		}
	})
}

func TestValidateStatus(t *testing.T) {
	validStatuses := []string{"pending", "active", "completed", "failed"}

	t.Run("valid status", func(t *testing.T) {
		err := ValidateStatus("active", validStatuses)
		if err != nil {
			t.Errorf("ValidateStatus() error = %v for valid status", err)
		}
	})

	t.Run("empty status", func(t *testing.T) {
		err := ValidateStatus("", validStatuses)
		if err == nil {
			t.Error("ValidateStatus() should return error for empty status")
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		err := ValidateStatus("unknown", validStatuses)
		if err == nil {
			t.Error("ValidateStatus() should return error for invalid status")
		}
		if !strings.Contains(err.Error(), "unknown") {
			t.Error("error message should contain the invalid status")
		}
	})
}

func TestDefaultPagination(t *testing.T) {
	p := DefaultPagination()
	if p.Limit != 50 {
		t.Errorf("DefaultPagination().Limit = %d, want 50", p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("DefaultPagination().Offset = %d, want 0", p.Offset)
	}
}

func TestNewPagination(t *testing.T) {
	t.Run("valid values", func(t *testing.T) {
		p := NewPagination(100, 50)
		if p.Limit != 100 {
			t.Errorf("Limit = %d, want 100", p.Limit)
		}
		if p.Offset != 50 {
			t.Errorf("Offset = %d, want 50", p.Offset)
		}
	})

	t.Run("normalizes invalid limit", func(t *testing.T) {
		p := NewPagination(0, 0)
		if p.Limit != 50 {
			t.Errorf("Limit = %d, want 50 (default)", p.Limit)
		}
	})

	t.Run("caps limit at max", func(t *testing.T) {
		p := NewPagination(5000, 0)
		if p.Limit != 1000 {
			t.Errorf("Limit = %d, want 1000 (max)", p.Limit)
		}
	})

	t.Run("normalizes negative offset", func(t *testing.T) {
		p := NewPagination(50, -10)
		if p.Offset != 0 {
			t.Errorf("Offset = %d, want 0", p.Offset)
		}
	})
}

func TestPaginationParamsToQuery(t *testing.T) {
	t.Run("with offset", func(t *testing.T) {
		p := PaginationParams{Limit: 100, Offset: 50}
		expected := "limit=100&offset=50"
		if p.ToQuery() != expected {
			t.Errorf("ToQuery() = %q, want %q", p.ToQuery(), expected)
		}
	})

	t.Run("without offset", func(t *testing.T) {
		p := PaginationParams{Limit: 100, Offset: 0}
		expected := "limit=100"
		if p.ToQuery() != expected {
			t.Errorf("ToQuery() = %q, want %q", p.ToQuery(), expected)
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors are distinct
	errors := []error{ErrNotFound, ErrAlreadyExists, ErrUnauthorized, ErrInvalidInput, ErrConflict, ErrDatabaseError}
	for i, e1 := range errors {
		for j, e2 := range errors {
			if i != j && e1 == e2 {
				t.Errorf("Sentinel errors should be distinct: %v == %v", e1, e2)
			}
		}
	}
}
