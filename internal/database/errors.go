// Package database provides Supabase database integration.
package database

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// Standard Error Types
// =============================================================================

var (
	// ErrNotFound is returned when a record is not found.
	ErrNotFound = errors.New("record not found")

	// ErrAlreadyExists is returned when trying to create a duplicate record.
	ErrAlreadyExists = errors.New("record already exists")

	// ErrUnauthorized is returned when the user is not authorized.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrInvalidInput is returned when input validation fails.
	ErrInvalidInput = errors.New("invalid input")

	// ErrConflict is returned when there's a conflict (e.g., concurrent modification).
	ErrConflict = errors.New("conflict")

	// ErrDatabaseError is returned for general database errors.
	ErrDatabaseError = errors.New("database error")
)

// NotFoundError wraps ErrNotFound with context.
type NotFoundError struct {
	Entity string
	ID     string
}

func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s with id '%s' not found", e.Entity, e.ID)
	}
	return fmt.Sprintf("%s not found", e.Entity)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(entity, id string) error {
	return &NotFoundError{Entity: entity, ID: id}
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is an already exists error.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsUnauthorized checks if an error is an unauthorized error.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsInvalidInput checks if an error is an invalid input error.
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// =============================================================================
// Input Validation
// =============================================================================

var (
	// uuidRegex matches UUID format (with or without hyphens).
	uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{12}$`)

	// alphanumericRegex matches alphanumeric strings with hyphens and underscores.
	alphanumericRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// addressRegex matches Neo N3 addresses (starting with N, 34 chars).
	addressRegex = regexp.MustCompile(`^N[A-Za-z0-9]{33}$`)

	// hexRegex matches hexadecimal strings.
	hexRegex = regexp.MustCompile(`^(0x)?[a-fA-F0-9]+$`)

	// emailRegex is a simple email validation pattern.
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// ValidateID validates an ID string (UUID or alphanumeric).
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidInput)
	}
	if len(id) > 128 {
		return fmt.Errorf("%w: id too long", ErrInvalidInput)
	}
	if !uuidRegex.MatchString(id) && !alphanumericRegex.MatchString(id) {
		return fmt.Errorf("%w: invalid id format", ErrInvalidInput)
	}
	return nil
}

// ValidateUserID validates a user ID.
func ValidateUserID(userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: user_id cannot be empty", ErrInvalidInput)
	}
	return ValidateID(userID)
}

// ValidateAddress validates a Neo N3 address.
func ValidateAddress(address string) error {
	if address == "" {
		return fmt.Errorf("%w: address cannot be empty", ErrInvalidInput)
	}
	if !addressRegex.MatchString(address) {
		return fmt.Errorf("%w: invalid Neo address format", ErrInvalidInput)
	}
	return nil
}

// ValidateEmail validates an email address.
func ValidateEmail(email string) error {
	if email == "" {
		return nil // Email is optional
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("%w: invalid email format", ErrInvalidInput)
	}
	return nil
}

// ValidateLimit validates and normalizes a limit parameter.
func ValidateLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// ValidateOffset validates an offset parameter.
func ValidateOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

// SanitizeString removes potentially dangerous characters from a string.
func SanitizeString(s string) string {
	// Remove null bytes and control characters
	s = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)
	// Trim whitespace
	return strings.TrimSpace(s)
}

// ValidateTxHash validates a transaction hash.
func ValidateTxHash(txHash string) error {
	if txHash == "" {
		return fmt.Errorf("%w: tx_hash cannot be empty", ErrInvalidInput)
	}
	if !hexRegex.MatchString(txHash) {
		return fmt.Errorf("%w: invalid tx_hash format", ErrInvalidInput)
	}
	return nil
}

// ValidateStatus validates a status string.
func ValidateStatus(status string, validStatuses []string) error {
	if status == "" {
		return fmt.Errorf("%w: status cannot be empty", ErrInvalidInput)
	}
	for _, valid := range validStatuses {
		if status == valid {
			return nil
		}
	}
	return fmt.Errorf("%w: invalid status '%s'", ErrInvalidInput, status)
}

// =============================================================================
// Pagination
// =============================================================================

// PaginationParams holds pagination parameters.
type PaginationParams struct {
	Limit  int
	Offset int
}

// DefaultPagination returns default pagination parameters.
func DefaultPagination() PaginationParams {
	return PaginationParams{
		Limit:  50,
		Offset: 0,
	}
}

// NewPagination creates validated pagination parameters.
func NewPagination(limit, offset int) PaginationParams {
	return PaginationParams{
		Limit:  ValidateLimit(limit, 50, 1000),
		Offset: ValidateOffset(offset),
	}
}

// ToQuery converts pagination to query string parameters.
func (p PaginationParams) ToQuery() string {
	if p.Offset > 0 {
		return fmt.Sprintf("limit=%d&offset=%d", p.Limit, p.Offset)
	}
	return fmt.Sprintf("limit=%d", p.Limit)
}
