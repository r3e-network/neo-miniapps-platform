// Package httputil provides common HTTP utilities for service handlers.
package httputil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ErrorResponse represents a standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

// WriteErrorWithCode writes a JSON error response with an error code.
func WriteErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message, Code: code})
}

// BadRequest writes a 400 Bad Request response.
func BadRequest(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusBadRequest, message)
}

// Unauthorized writes a 401 Unauthorized response.
func Unauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "unauthorized"
	}
	WriteError(w, http.StatusUnauthorized, message)
}

// Forbidden writes a 403 Forbidden response.
func Forbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "forbidden"
	}
	WriteError(w, http.StatusForbidden, message)
}

// NotFound writes a 404 Not Found response.
func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "not found"
	}
	WriteError(w, http.StatusNotFound, message)
}

// InternalError writes a 500 Internal Server Error response.
func InternalError(w http.ResponseWriter, message string) {
	if message == "" {
		message = "internal server error"
	}
	WriteError(w, http.StatusInternalServerError, message)
}

// ServiceUnavailable writes a 503 Service Unavailable response.
func ServiceUnavailable(w http.ResponseWriter, message string) {
	if message == "" {
		message = "service unavailable"
	}
	WriteError(w, http.StatusServiceUnavailable, message)
}

// DecodeJSON decodes a JSON request body into the provided struct.
// Returns false and writes an error response if decoding fails.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		BadRequest(w, "invalid request body")
		return false
	}
	return true
}

// PathParam extracts a path parameter from the URL.
// Example: PathParam("/users/123/orders", "/users/", "/orders") returns "123"
func PathParam(path, prefix, suffix string) string {
	path = strings.TrimPrefix(path, prefix)
	if suffix != "" {
		if idx := strings.Index(path, suffix); idx >= 0 {
			path = path[:idx]
		}
	}
	// Handle trailing slashes and additional path segments
	if idx := strings.Index(path, "/"); idx >= 0 {
		path = path[:idx]
	}
	return path
}

// PathParamAt extracts a path parameter at the given index (0-based).
// Example: PathParamAt("/users/123/orders/456", 1) returns "123"
func PathParamAt(path string, index int) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if index >= 0 && index < len(parts) {
		return parts[index]
	}
	return ""
}

// QueryInt extracts an integer query parameter with a default value.
func QueryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	if n, err := strconv.Atoi(val); err == nil {
		return n
	}
	return defaultVal
}

// QueryInt64 extracts an int64 query parameter with a default value.
func QueryInt64(r *http.Request, key string, defaultVal int64) int64 {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	if n, err := strconv.ParseInt(val, 10, 64); err == nil {
		return n
	}
	return defaultVal
}

// QueryString extracts a string query parameter with a default value.
func QueryString(r *http.Request, key, defaultVal string) string {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// QueryBool extracts a boolean query parameter with a default value.
func QueryBool(r *http.Request, key string, defaultVal bool) bool {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	return val == "true" || val == "1" || val == "yes"
}

// GetUserID extracts the user ID from the X-User-ID header.
// Returns empty string if not present.
func GetUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

// GetUserRole extracts the user role from the X-User-Role header.
func GetUserRole(r *http.Request) string {
	return r.Header.Get("X-User-Role")
}

// RequireUserID extracts the user ID from the X-User-ID header.
// Returns false and writes an error response if not present.
func RequireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := GetUserID(r)
	if userID == "" {
		Unauthorized(w, "")
		return "", false
	}
	return userID, true
}

// RequireAdminRole verifies the user role is admin or super_admin.
// Returns false and writes a 403 Forbidden response if the role check fails.
func RequireAdminRole(w http.ResponseWriter, r *http.Request) bool {
	role := strings.ToLower(GetUserRole(r))
	if role == "admin" || role == "super_admin" {
		return true
	}
	Forbidden(w, "admin role required")
	return false
}

// PaginationParams extracts pagination parameters from the request.
func PaginationParams(r *http.Request, defaultLimit, maxLimit int) (offset, limit int) {
	offset = QueryInt(r, "offset", 0)
	limit = QueryInt(r, "limit", defaultLimit)
	if limit > maxLimit {
		limit = maxLimit
	}
	if limit < 1 {
		limit = 1
	}
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}

// CORSMiddleware adds CORS headers to responses.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-User-Role")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WrapError wraps an error with context.
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
