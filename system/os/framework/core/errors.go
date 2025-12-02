package service

import (
	"context"
	"errors"
	"fmt"
)

// Standard service errors for consistent error handling across all services.
// These errors enable unified error mapping in HTTP handlers and observability.

var (
	// ErrNotFound indicates a requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates a resource already exists (duplicate).
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidInput indicates malformed or invalid input data.
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized indicates missing or invalid authentication.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates the caller lacks permission.
	ErrForbidden = errors.New("forbidden")

	// ErrConflict indicates a state conflict (e.g., concurrent modification).
	ErrConflict = errors.New("conflict")

	// ErrRateLimited indicates the caller exceeded rate limits.
	ErrRateLimited = errors.New("rate limited")

	// ErrServiceUnavailable indicates the service is temporarily unavailable.
	ErrServiceUnavailable = errors.New("service unavailable")

	// ErrTimeout indicates an operation exceeded its deadline.
	ErrTimeout = errors.New("timeout")

	// ErrInternal indicates an unexpected internal error.
	ErrInternal = errors.New("internal error")

	// ErrInvalidManifest indicates a service manifest is invalid.
	ErrInvalidManifest = errors.New("invalid manifest")

	// ErrServiceAlreadyStarted indicates a service is already running.
	ErrServiceAlreadyStarted = errors.New("service already started")

	// ErrHookFailed indicates a lifecycle hook failed.
	ErrHookFailed = errors.New("lifecycle hook failed")

	// ErrBusUnavailable indicates no bus engine is configured.
	ErrBusUnavailable = errors.New("bus unavailable")
)

// NotFoundError provides detailed not-found errors with resource context.
type NotFoundError struct {
	Resource string // e.g., "account", "function", "secret"
	ID       string // identifier that was not found
}

func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
	}
	return fmt.Sprintf("%s not found", e.Resource)
}

func (e *NotFoundError) Unwrap() error { return ErrNotFound }

// NewNotFoundError creates a not-found error for a specific resource.
func NewNotFoundError(resource, id string) error {
	return &NotFoundError{Resource: resource, ID: id}
}

// ValidationError provides detailed validation errors with field context.
type ValidationError struct {
	Field   string // field name that failed validation
	Message string // human-readable validation message
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

func (e *ValidationError) Unwrap() error { return ErrInvalidInput }

// NewValidationError creates a validation error for a specific field.
func NewValidationError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// RequiredError creates a validation error for a required field.
func RequiredError(field string) error {
	return &ValidationError{Field: field, Message: "is required"}
}

// AccessDeniedError provides detailed access control errors.
type AccessDeniedError struct {
	Resource  string // resource type
	ID        string // resource identifier
	AccountID string // requesting account
	Reason    string // optional explanation
}

func (e *AccessDeniedError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("access denied to %s %q for account %s: %s",
			e.Resource, e.ID, e.AccountID, e.Reason)
	}
	return fmt.Sprintf("access denied to %s %q for account %s",
		e.Resource, e.ID, e.AccountID)
}

func (e *AccessDeniedError) Unwrap() error { return ErrForbidden }

// NewAccessDeniedError creates an access denied error.
func NewAccessDeniedError(resource, id, accountID string) error {
	return &AccessDeniedError{Resource: resource, ID: id, AccountID: accountID}
}

// ConflictError provides detailed conflict errors.
type ConflictError struct {
	Resource string
	ID       string
	Message  string
}

func (e *ConflictError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s %q: %s", e.Resource, e.ID, e.Message)
	}
	return fmt.Sprintf("%s %q already exists", e.Resource, e.ID)
}

func (e *ConflictError) Unwrap() error { return ErrAlreadyExists }

// NewConflictError creates a conflict error.
func NewConflictError(resource, id, message string) error {
	return &ConflictError{Resource: resource, ID: id, Message: message}
}

// ServiceError wraps errors with service context for observability.
type ServiceError struct {
	Service   string // service name
	Operation string // operation that failed
	Err       error  // underlying error
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s.%s: %v", e.Service, e.Operation, e.Err)
}

func (e *ServiceError) Unwrap() error { return e.Err }

// WrapServiceError wraps an error with service context.
func WrapServiceError(service, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &ServiceError{Service: service, Operation: operation, Err: err}
}

// IsNotFound checks if an error is a not-found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsForbidden checks if an error is an access denied error.
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// IsConflict checks if an error is a conflict error.
func IsConflict(err error) bool {
	return errors.Is(err, ErrAlreadyExists) || errors.Is(err, ErrConflict)
}

// OwnershipError indicates a resource does not belong to the requesting account.
// This is a common pattern across services for multi-tenant resource access control.
type OwnershipError struct {
	Resource  string // resource type (e.g., "feed", "key", "source")
	ID        string // resource identifier
	AccountID string // requesting account that doesn't own the resource
}

func (e *OwnershipError) Error() string {
	return fmt.Sprintf("%s %s does not belong to account %s", e.Resource, e.ID, e.AccountID)
}

func (e *OwnershipError) Unwrap() error { return ErrForbidden }

// NewOwnershipError creates an ownership error for a resource.
func NewOwnershipError(resource, id, accountID string) error {
	return &OwnershipError{Resource: resource, ID: id, AccountID: accountID}
}

// EnsureOwnership checks if a resource belongs to the specified account.
// Returns an OwnershipError if the resource's account doesn't match.
// This is a convenience function to reduce boilerplate in services.
func EnsureOwnership(resourceAccountID, requestAccountID, resourceType, resourceID string) error {
	if resourceAccountID != requestAccountID {
		return NewOwnershipError(resourceType, resourceID, requestAccountID)
	}
	return nil
}

// IsOwnershipError checks if an error is an ownership error.
func IsOwnershipError(err error) bool {
	var oe *OwnershipError
	return errors.As(err, &oe)
}

// HookError represents a lifecycle hook error.
type HookError struct {
	Service  string // Service name
	HookType string // Hook type (PreStart, PostStart, PreStop, PostStop)
	HookName string // Optional hook name
	Err      error  // Underlying error
}

// Error implements the error interface.
func (e *HookError) Error() string {
	if e.HookName != "" {
		return fmt.Sprintf("%s: %s hook %q failed: %v", e.Service, e.HookType, e.HookName, e.Err)
	}
	return fmt.Sprintf("%s: %s hook failed: %v", e.Service, e.HookType, e.Err)
}

// Unwrap returns the underlying error.
func (e *HookError) Unwrap() error {
	return e.Err
}

// NewHookError creates a new HookError.
func NewHookError(service, hookType string, err error) *HookError {
	return &HookError{
		Service:  service,
		HookType: hookType,
		Err:      err,
	}
}

// IsHookError returns true if the error is a hook error.
func IsHookError(err error) bool {
	return errors.Is(err, ErrHookFailed)
}

// EventPublisher is the standard interface for services that publish events.
// This consolidates the duplicate eventPublisher interfaces across services.
// Services implementing this interface can be used with the core engine adapter.
type EventPublisher interface {
	Publish(ctx context.Context, event string, payload any) error
}

// RowScanner is the standard interface for database row scanning.
// This consolidates the duplicate rowScanner interfaces across 12 service packages.
// Compatible with *sql.Row and *sql.Rows.
type RowScanner interface {
	Scan(dest ...any) error
}
