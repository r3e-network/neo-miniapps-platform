package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestServiceError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ServiceError
		want string
	}{
		{
			name: "error without underlying error",
			err:  New(ErrCodeUnauthorized, "test message", http.StatusUnauthorized),
			want: "[AUTH_1001] test message",
		},
		{
			name: "error with underlying error",
			err:  Wrap(ErrCodeInternal, "test message", http.StatusInternalServerError, errors.New("underlying")),
			want: "[SVC_5001] test message: underlying",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := Wrap(ErrCodeInternal, "test", http.StatusInternalServerError, underlying)

	if got := err.Unwrap(); got != underlying {
		t.Errorf("Unwrap() = %v, want %v", got, underlying)
	}
}

func TestServiceError_WithDetails(t *testing.T) {
	err := New(ErrCodeInvalidInput, "test", http.StatusBadRequest)
	err.WithDetails("field", "username").WithDetails("reason", "too short")

	if len(err.Details) != 2 {
		t.Errorf("Details length = %d, want 2", len(err.Details))
	}

	if err.Details["field"] != "username" {
		t.Errorf("Details[field] = %v, want username", err.Details["field"])
	}

	if err.Details["reason"] != "too short" {
		t.Errorf("Details[reason] = %v, want too short", err.Details["reason"])
	}
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("test message")

	if err.Code != ErrCodeUnauthorized {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeUnauthorized)
	}

	if err.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusUnauthorized)
	}

	if err.Message != "test message" {
		t.Errorf("Message = %v, want test message", err.Message)
	}
}

func TestInvalidToken(t *testing.T) {
	underlying := errors.New("token parse error")
	err := InvalidToken(underlying)

	if err.Code != ErrCodeInvalidToken {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInvalidToken)
	}

	if err.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusUnauthorized)
	}

	if err.Err != underlying {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
}

func TestTokenExpired(t *testing.T) {
	err := TokenExpired()

	if err.Code != ErrCodeTokenExpired {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeTokenExpired)
	}

	if err.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusUnauthorized)
	}
}

func TestForbidden(t *testing.T) {
	err := Forbidden("access denied")

	if err.Code != ErrCodeForbidden {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeForbidden)
	}

	if err.HTTPStatus != http.StatusForbidden {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusForbidden)
	}
}

func TestInsufficientFunds(t *testing.T) {
	err := InsufficientFunds("100", "50")

	if err.Code != ErrCodeInsufficientFunds {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInsufficientFunds)
	}

	if err.Details["required"] != "100" {
		t.Errorf("Details[required] = %v, want 100", err.Details["required"])
	}

	if err.Details["available"] != "50" {
		t.Errorf("Details[available] = %v, want 50", err.Details["available"])
	}
}

func TestInvalidInput(t *testing.T) {
	err := InvalidInput("email", "invalid format")

	if err.Code != ErrCodeInvalidInput {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInvalidInput)
	}

	if err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusBadRequest)
	}

	if err.Details["field"] != "email" {
		t.Errorf("Details[field] = %v, want email", err.Details["field"])
	}
}

func TestMissingParameter(t *testing.T) {
	err := MissingParameter("user_id")

	if err.Code != ErrCodeMissingParameter {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeMissingParameter)
	}

	if err.Details["parameter"] != "user_id" {
		t.Errorf("Details[parameter] = %v, want user_id", err.Details["parameter"])
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound("user", "123")

	if err.Code != ErrCodeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeNotFound)
	}

	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusNotFound)
	}

	if err.Details["resource"] != "user" {
		t.Errorf("Details[resource] = %v, want user", err.Details["resource"])
	}

	if err.Details["id"] != "123" {
		t.Errorf("Details[id] = %v, want 123", err.Details["id"])
	}
}

func TestAlreadyExists(t *testing.T) {
	err := AlreadyExists("user", "test@example.com")

	if err.Code != ErrCodeAlreadyExists {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeAlreadyExists)
	}

	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusConflict)
	}
}

func TestInternal(t *testing.T) {
	underlying := errors.New("database connection failed")
	err := Internal("internal error", underlying)

	if err.Code != ErrCodeInternal {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInternal)
	}

	if err.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusInternalServerError)
	}

	if err.Err != underlying {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
}

func TestDatabaseError(t *testing.T) {
	underlying := errors.New("connection timeout")
	err := DatabaseError("insert", underlying)

	if err.Code != ErrCodeDatabaseError {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeDatabaseError)
	}

	if err.Details["operation"] != "insert" {
		t.Errorf("Details[operation] = %v, want insert", err.Details["operation"])
	}
}

func TestBlockchainError(t *testing.T) {
	underlying := errors.New("rpc timeout")
	err := BlockchainError("invoke", underlying)

	if err.Code != ErrCodeBlockchainError {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeBlockchainError)
	}

	if err.HTTPStatus != http.StatusServiceUnavailable {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusServiceUnavailable)
	}
}

func TestRateLimitExceeded(t *testing.T) {
	err := RateLimitExceeded(100, "1m")

	if err.Code != ErrCodeRateLimitExceeded {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeRateLimitExceeded)
	}

	if err.HTTPStatus != http.StatusTooManyRequests {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusTooManyRequests)
	}

	if err.Details["limit"] != 100 {
		t.Errorf("Details[limit] = %v, want 100", err.Details["limit"])
	}
}

func TestEncryptionFailed(t *testing.T) {
	underlying := errors.New("key derivation failed")
	err := EncryptionFailed(underlying)

	if err.Code != ErrCodeEncryptionFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeEncryptionFailed)
	}

	if err.Err != underlying {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
}

func TestDecryptionFailed(t *testing.T) {
	underlying := errors.New("invalid ciphertext")
	err := DecryptionFailed(underlying)

	if err.Code != ErrCodeDecryptionFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeDecryptionFailed)
	}
}

func TestSigningFailed(t *testing.T) {
	underlying := errors.New("private key not found")
	err := SigningFailed(underlying)

	if err.Code != ErrCodeSigningFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeSigningFailed)
	}
}

func TestVerificationFailed(t *testing.T) {
	underlying := errors.New("signature mismatch")
	err := VerificationFailed(underlying)

	if err.Code != ErrCodeVerificationFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeVerificationFailed)
	}

	if err.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusUnauthorized)
	}
}

func TestAttestationFailed(t *testing.T) {
	underlying := errors.New("quote verification failed")
	err := AttestationFailed(underlying)

	if err.Code != ErrCodeAttestationFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeAttestationFailed)
	}
}

func TestIsServiceError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "service error",
			err:  New(ErrCodeInternal, "test", http.StatusInternalServerError),
			want: true,
		},
		{
			name: "standard error",
			err:  errors.New("standard error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsServiceError(tt.err); got != tt.want {
				t.Errorf("IsServiceError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetServiceError(t *testing.T) {
	serviceErr := New(ErrCodeInternal, "test", http.StatusInternalServerError)
	standardErr := errors.New("standard error")

	tests := []struct {
		name string
		err  error
		want *ServiceError
	}{
		{
			name: "service error",
			err:  serviceErr,
			want: serviceErr,
		},
		{
			name: "standard error",
			err:  standardErr,
			want: nil,
		},
		{
			name: "nil error",
			err:  nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetServiceError(tt.err)
			if got != tt.want {
				t.Errorf("GetServiceError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "service error",
			err:  New(ErrCodeUnauthorized, "test", http.StatusUnauthorized),
			want: http.StatusUnauthorized,
		},
		{
			name: "standard error",
			err:  errors.New("standard error"),
			want: http.StatusInternalServerError,
		},
		{
			name: "nil error",
			err:  nil,
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHTTPStatus(tt.err); got != tt.want {
				t.Errorf("GetHTTPStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutOfRange(t *testing.T) {
	err := OutOfRange("age", 0, 120)

	if err.Code != ErrCodeOutOfRange {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeOutOfRange)
	}

	if err.Details["field"] != "age" {
		t.Errorf("Details[field] = %v, want age", err.Details["field"])
	}

	if err.Details["min"] != 0 {
		t.Errorf("Details[min] = %v, want 0", err.Details["min"])
	}

	if err.Details["max"] != 120 {
		t.Errorf("Details[max] = %v, want 120", err.Details["max"])
	}
}

func TestConflict(t *testing.T) {
	err := Conflict("resource locked")

	if err.Code != ErrCodeConflict {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeConflict)
	}

	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusConflict)
	}

	if err.Message != "resource locked" {
		t.Errorf("Message = %v, want resource locked", err.Message)
	}
}

func TestTimeout(t *testing.T) {
	err := Timeout("database query")

	if err.Code != ErrCodeTimeout {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeTimeout)
	}

	if err.HTTPStatus != http.StatusGatewayTimeout {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusGatewayTimeout)
	}

	if err.Details["operation"] != "database query" {
		t.Errorf("Details[operation] = %v, want database query", err.Details["operation"])
	}
}
