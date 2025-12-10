// Package errors provides unified error handling for the service layer
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode represents a unique error code
type ErrorCode string

const (
	// Authentication errors (1xxx)
	ErrCodeUnauthorized     ErrorCode = "AUTH_1001"
	ErrCodeInvalidToken     ErrorCode = "AUTH_1002"
	ErrCodeTokenExpired     ErrorCode = "AUTH_1003"
	ErrCodeInvalidSignature ErrorCode = "AUTH_1004"

	// Authorization errors (2xxx)
	ErrCodeForbidden        ErrorCode = "AUTHZ_2001"
	ErrCodeInsufficientFunds ErrorCode = "AUTHZ_2002"
	ErrCodeOwnershipRequired ErrorCode = "AUTHZ_2003"

	// Validation errors (3xxx)
	ErrCodeInvalidInput     ErrorCode = "VAL_3001"
	ErrCodeMissingParameter ErrorCode = "VAL_3002"
	ErrCodeInvalidFormat    ErrorCode = "VAL_3003"
	ErrCodeOutOfRange       ErrorCode = "VAL_3004"

	// Resource errors (4xxx)
	ErrCodeNotFound      ErrorCode = "RES_4001"
	ErrCodeAlreadyExists ErrorCode = "RES_4002"
	ErrCodeConflict      ErrorCode = "RES_4003"

	// Service errors (5xxx)
	ErrCodeInternal         ErrorCode = "SVC_5001"
	ErrCodeDatabaseError    ErrorCode = "SVC_5002"
	ErrCodeBlockchainError  ErrorCode = "SVC_5003"
	ErrCodeExternalAPI      ErrorCode = "SVC_5004"
	ErrCodeTimeout          ErrorCode = "SVC_5005"
	ErrCodeRateLimitExceeded ErrorCode = "SVC_5006"

	// Cryptographic errors (6xxx)
	ErrCodeEncryptionFailed ErrorCode = "CRYPTO_6001"
	ErrCodeDecryptionFailed ErrorCode = "CRYPTO_6002"
	ErrCodeSigningFailed    ErrorCode = "CRYPTO_6003"
	ErrCodeVerificationFailed ErrorCode = "CRYPTO_6004"

	// TEE errors (7xxx)
	ErrCodeAttestationFailed ErrorCode = "TEE_7001"
	ErrCodeSealingFailed     ErrorCode = "TEE_7002"
	ErrCodeUnsealingFailed   ErrorCode = "TEE_7003"
)

// ServiceError represents a structured error with code, message, and HTTP status
type ServiceError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	HTTPStatus int                    `json:"-"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Err        error                  `json:"-"`
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *ServiceError) Unwrap() error {
	return e.Err
}

// WithDetails adds additional details to the error
func (e *ServiceError) WithDetails(key string, value interface{}) *ServiceError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// New creates a new ServiceError
func New(code ErrorCode, message string, httpStatus int) *ServiceError {
	return &ServiceError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// Wrap wraps an existing error with a ServiceError
func Wrap(code ErrorCode, message string, httpStatus int, err error) *ServiceError {
	return &ServiceError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err,
	}
}

// Authentication Errors

func Unauthorized(message string) *ServiceError {
	return New(ErrCodeUnauthorized, message, http.StatusUnauthorized)
}

func InvalidToken(err error) *ServiceError {
	return Wrap(ErrCodeInvalidToken, "Invalid authentication token", http.StatusUnauthorized, err)
}

func TokenExpired() *ServiceError {
	return New(ErrCodeTokenExpired, "Authentication token has expired", http.StatusUnauthorized)
}

func InvalidSignature(err error) *ServiceError {
	return Wrap(ErrCodeInvalidSignature, "Invalid signature", http.StatusUnauthorized, err)
}

// Authorization Errors

func Forbidden(message string) *ServiceError {
	return New(ErrCodeForbidden, message, http.StatusForbidden)
}

func InsufficientFunds(required, available string) *ServiceError {
	return New(ErrCodeInsufficientFunds, "Insufficient funds", http.StatusPaymentRequired).
		WithDetails("required", required).
		WithDetails("available", available)
}

func OwnershipRequired(resource string) *ServiceError {
	return New(ErrCodeOwnershipRequired, "Ownership verification required", http.StatusForbidden).
		WithDetails("resource", resource)
}

// Validation Errors

func InvalidInput(field, reason string) *ServiceError {
	return New(ErrCodeInvalidInput, "Invalid input", http.StatusBadRequest).
		WithDetails("field", field).
		WithDetails("reason", reason)
}

func MissingParameter(param string) *ServiceError {
	return New(ErrCodeMissingParameter, "Missing required parameter", http.StatusBadRequest).
		WithDetails("parameter", param)
}

func InvalidFormat(field, expected string) *ServiceError {
	return New(ErrCodeInvalidFormat, "Invalid format", http.StatusBadRequest).
		WithDetails("field", field).
		WithDetails("expected", expected)
}

func OutOfRange(field string, min, max interface{}) *ServiceError {
	return New(ErrCodeOutOfRange, "Value out of range", http.StatusBadRequest).
		WithDetails("field", field).
		WithDetails("min", min).
		WithDetails("max", max)
}

// Resource Errors

func NotFound(resource, id string) *ServiceError {
	return New(ErrCodeNotFound, "Resource not found", http.StatusNotFound).
		WithDetails("resource", resource).
		WithDetails("id", id)
}

func AlreadyExists(resource, id string) *ServiceError {
	return New(ErrCodeAlreadyExists, "Resource already exists", http.StatusConflict).
		WithDetails("resource", resource).
		WithDetails("id", id)
}

func Conflict(message string) *ServiceError {
	return New(ErrCodeConflict, message, http.StatusConflict)
}

// Service Errors

func Internal(message string, err error) *ServiceError {
	return Wrap(ErrCodeInternal, message, http.StatusInternalServerError, err)
}

func DatabaseError(operation string, err error) *ServiceError {
	return Wrap(ErrCodeDatabaseError, "Database operation failed", http.StatusInternalServerError, err).
		WithDetails("operation", operation)
}

func BlockchainError(operation string, err error) *ServiceError {
	return Wrap(ErrCodeBlockchainError, "Blockchain operation failed", http.StatusServiceUnavailable, err).
		WithDetails("operation", operation)
}

func ExternalAPIError(service string, err error) *ServiceError {
	return Wrap(ErrCodeExternalAPI, "External API call failed", http.StatusBadGateway, err).
		WithDetails("service", service)
}

func Timeout(operation string) *ServiceError {
	return New(ErrCodeTimeout, "Operation timed out", http.StatusGatewayTimeout).
		WithDetails("operation", operation)
}

func RateLimitExceeded(limit int, window string) *ServiceError {
	return New(ErrCodeRateLimitExceeded, "Rate limit exceeded", http.StatusTooManyRequests).
		WithDetails("limit", limit).
		WithDetails("window", window)
}

// Cryptographic Errors

func EncryptionFailed(err error) *ServiceError {
	return Wrap(ErrCodeEncryptionFailed, "Encryption failed", http.StatusInternalServerError, err)
}

func DecryptionFailed(err error) *ServiceError {
	return Wrap(ErrCodeDecryptionFailed, "Decryption failed", http.StatusInternalServerError, err)
}

func SigningFailed(err error) *ServiceError {
	return Wrap(ErrCodeSigningFailed, "Signing failed", http.StatusInternalServerError, err)
}

func VerificationFailed(err error) *ServiceError {
	return Wrap(ErrCodeVerificationFailed, "Verification failed", http.StatusUnauthorized, err)
}

// TEE Errors

func AttestationFailed(err error) *ServiceError {
	return Wrap(ErrCodeAttestationFailed, "Remote attestation failed", http.StatusInternalServerError, err)
}

func SealingFailed(err error) *ServiceError {
	return Wrap(ErrCodeSealingFailed, "Data sealing failed", http.StatusInternalServerError, err)
}

func UnsealingFailed(err error) *ServiceError {
	return Wrap(ErrCodeUnsealingFailed, "Data unsealing failed", http.StatusInternalServerError, err)
}

// Helper functions

// IsServiceError checks if an error is a ServiceError
func IsServiceError(err error) bool {
	var serviceErr *ServiceError
	return errors.As(err, &serviceErr)
}

// GetServiceError extracts a ServiceError from an error chain
func GetServiceError(err error) *ServiceError {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr
	}
	return nil
}

// GetHTTPStatus returns the HTTP status code for an error
func GetHTTPStatus(err error) int {
	if serviceErr := GetServiceError(err); serviceErr != nil {
		return serviceErr.HTTPStatus
	}
	return http.StatusInternalServerError
}
