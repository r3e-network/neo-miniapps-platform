package service

import (
	"context"
	"errors"
	"testing"
)

func TestRequireString(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		wantErr   bool
	}{
		{"valid", "hello", "field", false},
		{"empty", "", "field", true},
		{"whitespace only", "   ", "field", true},
		{"with spaces", "  hello  ", "field", false},
		{"tabs", "\t\n", "field", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireString(tt.value, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("RequireString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("RequireString() error should wrap ErrInvalidInput")
			}
		})
	}
}

func TestRequireAndTrim(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		want      string
		wantErr   bool
	}{
		{"valid", "hello", "field", "hello", false},
		{"with spaces", "  hello  ", "field", "hello", false},
		{"empty", "", "field", "", true},
		{"whitespace only", "   ", "field", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RequireAndTrim(tt.value, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("RequireAndTrim() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("RequireAndTrim() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateLength(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		min     int
		max     int
		wantErr bool
	}{
		{"within bounds", "hello", 1, 10, false},
		{"exact min", "hi", 2, 10, false},
		{"exact max", "hello", 1, 5, false},
		{"too short", "hi", 5, 10, true},
		{"too long", "hello world", 1, 5, true},
		{"no min check", "a", 0, 10, false},
		{"no max check", "hello world", 1, 0, false},
		{"unicode", "你好世界", 2, 10, false}, // 4 runes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLength(tt.value, "field", tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		pattern string
		wantErr bool
	}{
		{"email valid", "test@example.com", "email", false},
		{"email invalid", "not-an-email", "email", true},
		{"uuid valid", "550e8400-e29b-41d4-a716-446655440000", "uuid", false},
		{"uuid no hyphens", "550e8400e29b41d4a716446655440000", "uuid", false},
		{"uuid invalid", "not-a-uuid", "uuid", true},
		{"hex valid", "0x1234abcd", "hex", false},
		{"hex no prefix", "1234abcd", "hex", false},
		{"hex invalid", "xyz", "hex", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.pattern {
			case "email":
				err = ValidatePattern(tt.value, "field", EmailPattern, "")
			case "uuid":
				err = ValidatePattern(tt.value, "field", UUIDPattern, "")
			case "hex":
				err = ValidatePattern(tt.value, "field", HexPattern, "")
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePattern() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNonEmpty(t *testing.T) {
	t.Run("non-empty slice", func(t *testing.T) {
		err := ValidateNonEmpty([]string{"a", "b"}, "items")
		if err != nil {
			t.Errorf("ValidateNonEmpty() unexpected error: %v", err)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		err := ValidateNonEmpty([]string{}, "items")
		if err == nil {
			t.Error("ValidateNonEmpty() expected error for empty slice")
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []int
		err := ValidateNonEmpty(s, "items")
		if err == nil {
			t.Error("ValidateNonEmpty() expected error for nil slice")
		}
	})
}

func TestValidateSliceLength(t *testing.T) {
	tests := []struct {
		name    string
		slice   []int
		min     int
		max     int
		wantErr bool
	}{
		{"within bounds", []int{1, 2, 3}, 1, 5, false},
		{"exact min", []int{1, 2}, 2, 5, false},
		{"exact max", []int{1, 2, 3, 4, 5}, 1, 5, false},
		{"too few", []int{1}, 3, 5, true},
		{"too many", []int{1, 2, 3, 4, 5, 6}, 1, 5, true},
		{"no min check", []int{1}, 0, 5, false},
		{"no max check", []int{1, 2, 3, 4, 5, 6}, 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSliceLength(tt.slice, "items", tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSliceLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePositive(t *testing.T) {
	t.Run("positive int", func(t *testing.T) {
		if err := ValidatePositive(5, "value"); err != nil {
			t.Errorf("ValidatePositive() unexpected error: %v", err)
		}
	})

	t.Run("zero", func(t *testing.T) {
		if err := ValidatePositive(0, "value"); err == nil {
			t.Error("ValidatePositive() expected error for zero")
		}
	})

	t.Run("negative", func(t *testing.T) {
		if err := ValidatePositive(-5, "value"); err == nil {
			t.Error("ValidatePositive() expected error for negative")
		}
	})

	t.Run("positive float", func(t *testing.T) {
		if err := ValidatePositive(3.14, "value"); err != nil {
			t.Errorf("ValidatePositive() unexpected error: %v", err)
		}
	})
}

func TestValidateNonNegative(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		if err := ValidateNonNegative(5, "value"); err != nil {
			t.Errorf("ValidateNonNegative() unexpected error: %v", err)
		}
	})

	t.Run("zero", func(t *testing.T) {
		if err := ValidateNonNegative(0, "value"); err != nil {
			t.Errorf("ValidateNonNegative() unexpected error for zero: %v", err)
		}
	})

	t.Run("negative", func(t *testing.T) {
		if err := ValidateNonNegative(-5, "value"); err == nil {
			t.Error("ValidateNonNegative() expected error for negative")
		}
	})
}

func TestValidateRange(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		min     int
		max     int
		wantErr bool
	}{
		{"within range", 5, 1, 10, false},
		{"at min", 1, 1, 10, false},
		{"at max", 10, 1, 10, false},
		{"below min", 0, 1, 10, true},
		{"above max", 11, 1, 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRange(tt.value, "value", tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Mock AccountChecker for testing
type mockAccountChecker struct {
	exists map[string]bool
}

func (m *mockAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	if m.exists[accountID] {
		return nil
	}
	return NewNotFoundError("account", accountID)
}

func (m *mockAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	return ""
}

func TestValidateAccountID(t *testing.T) {
	checker := &mockAccountChecker{
		exists: map[string]bool{"acc-123": true},
	}

	t.Run("valid account", func(t *testing.T) {
		err := ValidateAccountID(context.Background(), checker, "acc-123")
		if err != nil {
			t.Errorf("ValidateAccountID() unexpected error: %v", err)
		}
	})

	t.Run("empty account", func(t *testing.T) {
		err := ValidateAccountID(context.Background(), checker, "")
		if err == nil {
			t.Error("ValidateAccountID() expected error for empty account")
		}
	})

	t.Run("non-existent account", func(t *testing.T) {
		err := ValidateAccountID(context.Background(), checker, "acc-999")
		if err == nil {
			t.Error("ValidateAccountID() expected error for non-existent account")
		}
	})

	t.Run("nil checker", func(t *testing.T) {
		err := ValidateAccountID(context.Background(), nil, "acc-123")
		if err != nil {
			t.Errorf("ValidateAccountID() unexpected error with nil checker: %v", err)
		}
	})
}

func TestValidationErrors(t *testing.T) {
	t.Run("collect errors", func(t *testing.T) {
		v := NewValidator()
		v.Add(nil) // should be ignored
		v.Add(RequiredError("field1"))
		v.Add(RequiredError("field2"))

		if !v.HasErrors() {
			t.Error("HasErrors() should return true")
		}

		if len(v.All()) != 2 {
			t.Errorf("All() should return 2 errors, got %d", len(v.All()))
		}

		if v.Error() == nil {
			t.Error("Error() should return first error")
		}
	})

	t.Run("no errors", func(t *testing.T) {
		v := NewValidator()
		v.Add(nil)

		if v.HasErrors() {
			t.Error("HasErrors() should return false")
		}

		if v.Error() != nil {
			t.Error("Error() should return nil")
		}
	})
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"test@example.com", false},
		{"user.name@domain.org", false},
		{"user+tag@example.co.uk", false},
		{"", true},
		{"not-an-email", true},
		{"@example.com", true},
		{"test@", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := ValidateEmail(tt.email, "email")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		uuid    string
		wantErr bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", false},
		{"550e8400e29b41d4a716446655440000", false},
		{"", true},
		{"not-a-uuid", true},
		{"550e8400-e29b-41d4-a716", true}, // too short
	}

	for _, tt := range tests {
		t.Run(tt.uuid, func(t *testing.T) {
			err := ValidateUUID(tt.uuid, "uuid")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUUID(%q) error = %v, wantErr %v", tt.uuid, err, tt.wantErr)
			}
		})
	}
}

func TestValidateHex(t *testing.T) {
	tests := []struct {
		hex     string
		wantErr bool
	}{
		{"0x1234abcd", false},
		{"1234ABCD", false},
		{"deadbeef", false},
		{"", true},
		{"xyz", true},
		{"0xGHIJ", true},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			err := ValidateHex(tt.hex, "hex")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHex(%q) error = %v, wantErr %v", tt.hex, err, tt.wantErr)
			}
		})
	}
}
