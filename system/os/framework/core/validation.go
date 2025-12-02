package service

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"
)

// =============================================================================
// String Validation Helpers
// =============================================================================

// NOTE: ValidateRequired and ValidateRequiredFields are defined in normalize.go
// Use those functions for basic required field validation.

// RequireString checks if a string value is non-empty after trimming.
// Returns a RequiredError if the value is empty.
// This is a simpler alternative to ValidateRequired for single fields.
func RequireString(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return RequiredError(fieldName)
	}
	return nil
}

// RequireAndTrim checks if a string is non-empty and returns the trimmed value.
// This combines validation and normalization in one call.
func RequireAndTrim(value, fieldName string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", RequiredError(fieldName)
	}
	return trimmed, nil
}

// ValidateLength checks if a string length is within bounds.
// Pass 0 for min or max to skip that check.
func ValidateLength(value, fieldName string, min, max int) error {
	length := utf8.RuneCountInString(value)
	if min > 0 && length < min {
		return NewValidationError(fieldName, "must be at least "+itoa(min)+" characters")
	}
	if max > 0 && length > max {
		return NewValidationError(fieldName, "must be at most "+itoa(max)+" characters")
	}
	return nil
}

// ValidatePattern checks if a string matches a regex pattern.
func ValidatePattern(value, fieldName string, pattern *regexp.Regexp, message string) error {
	if !pattern.MatchString(value) {
		if message == "" {
			message = "has invalid format"
		}
		return NewValidationError(fieldName, message)
	}
	return nil
}

// =============================================================================
// Slice Validation Helpers
// =============================================================================

// ValidateNonEmpty checks if a slice has at least one element.
func ValidateNonEmpty[T any](slice []T, fieldName string) error {
	if len(slice) == 0 {
		return NewValidationError(fieldName, "must not be empty")
	}
	return nil
}

// ValidateSliceLength checks if a slice length is within bounds.
func ValidateSliceLength[T any](slice []T, fieldName string, min, max int) error {
	length := len(slice)
	if min > 0 && length < min {
		return NewValidationError(fieldName, "must have at least "+itoa(min)+" items")
	}
	if max > 0 && length > max {
		return NewValidationError(fieldName, "must have at most "+itoa(max)+" items")
	}
	return nil
}

// =============================================================================
// Numeric Validation Helpers
// =============================================================================

// ValidatePositive checks if a number is positive (> 0).
func ValidatePositive[T ~int | ~int64 | ~float64](value T, fieldName string) error {
	if value <= 0 {
		return NewValidationError(fieldName, "must be positive")
	}
	return nil
}

// ValidateNonNegative checks if a number is non-negative (>= 0).
func ValidateNonNegative[T ~int | ~int64 | ~float64](value T, fieldName string) error {
	if value < 0 {
		return NewValidationError(fieldName, "must not be negative")
	}
	return nil
}

// ValidateRange checks if a number is within a range [min, max].
func ValidateRange[T ~int | ~int64 | ~float64](value T, fieldName string, min, max T) error {
	if value < min || value > max {
		return NewValidationError(fieldName, "must be between "+ftoa(min)+" and "+ftoa(max))
	}
	return nil
}

// =============================================================================
// Account Validation Helpers
// =============================================================================

// ValidateAccountID validates an account ID using the provided checker.
// This is a convenience function that combines required check and existence check.
func ValidateAccountID(ctx context.Context, checker AccountChecker, accountID string) error {
	if err := RequireString(accountID, "account_id"); err != nil {
		return err
	}
	if checker == nil {
		return nil
	}
	return checker.AccountExists(ctx, accountID)
}

// ValidateOwnership checks if a resource belongs to the requesting account.
// This is an alias for EnsureOwnership for consistency with other Validate* functions.
func ValidateOwnership(resourceAccountID, requestAccountID, resourceType, resourceID string) error {
	return EnsureOwnership(resourceAccountID, requestAccountID, resourceType, resourceID)
}

// =============================================================================
// Common Patterns
// =============================================================================

var (
	// EmailPattern matches basic email format.
	EmailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// UUIDPattern matches UUID format (with or without hyphens).
	UUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)

	// HexPattern matches hexadecimal strings.
	HexPattern = regexp.MustCompile(`^(0x)?[0-9a-fA-F]+$`)

	// AlphanumericPattern matches alphanumeric strings with underscores.
	AlphanumericPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

	// SlugPattern matches URL-safe slugs.
	SlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// ValidateEmail validates an email address format.
func ValidateEmail(value, fieldName string) error {
	if err := RequireString(value, fieldName); err != nil {
		return err
	}
	return ValidatePattern(value, fieldName, EmailPattern, "must be a valid email address")
}

// ValidateUUID validates a UUID format.
func ValidateUUID(value, fieldName string) error {
	if err := RequireString(value, fieldName); err != nil {
		return err
	}
	return ValidatePattern(value, fieldName, UUIDPattern, "must be a valid UUID")
}

// ValidateHex validates a hexadecimal string.
func ValidateHex(value, fieldName string) error {
	if err := RequireString(value, fieldName); err != nil {
		return err
	}
	return ValidatePattern(value, fieldName, HexPattern, "must be a valid hexadecimal string")
}

// =============================================================================
// Batch Validation
// =============================================================================

// ValidationErrors collects multiple validation errors.
type ValidationErrors struct {
	Errors []error
}

// Add adds an error if it's not nil.
func (v *ValidationErrors) Add(err error) {
	if err != nil {
		v.Errors = append(v.Errors, err)
	}
}

// HasErrors returns true if any errors were collected.
func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

// Error returns the first error or nil.
func (v *ValidationErrors) Error() error {
	if len(v.Errors) == 0 {
		return nil
	}
	return v.Errors[0]
}

// All returns all collected errors.
func (v *ValidationErrors) All() []error {
	return v.Errors
}

// NewValidator creates a new ValidationErrors collector.
func NewValidator() *ValidationErrors {
	return &ValidationErrors{}
}

// =============================================================================
// Internal helpers
// =============================================================================

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ftoa converts a numeric value to string.
func ftoa[T ~int | ~int64 | ~float64](v T) string {
	switch any(v).(type) {
	case int, int64:
		return itoa(int(v))
	default:
		// For float64, use a simple representation
		// In production, use strconv.FormatFloat
		return itoa(int(v))
	}
}
