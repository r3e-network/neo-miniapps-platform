// Package httputil provides common HTTP utilities for service handlers.
package httputil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/runtime"
	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

// ErrorResponse represents a standard error response.
type ErrorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

var defaultLogger = logging.NewFromEnv("httputil")

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		defaultLogger.WithError(err).Warn("write json response")
	}
}

func traceIDFromRequestOrResponse(w http.ResponseWriter, r *http.Request) string {
	if r != nil {
		if traceID := logging.GetTraceID(r.Context()); traceID != "" {
			return traceID
		}
		if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
			return traceID
		}
	}

	return w.Header().Get("X-Trace-ID")
}

// WriteErrorResponse writes a standard JSON error response envelope.
func WriteErrorResponse(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
	if code == "" {
		code = fmt.Sprintf("HTTP_%d", status)
	}

	traceID := traceIDFromRequestOrResponse(w, r)
	if traceID != "" && w.Header().Get("X-Trace-ID") == "" {
		w.Header().Set("X-Trace-ID", traceID)
	}

	WriteJSON(w, status, ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
		TraceID: traceID,
	})
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteErrorResponse(w, nil, status, "", message, nil)
}

// WriteErrorWithCode writes a JSON error response with an error code.
func WriteErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	WriteErrorResponse(w, nil, status, code, message, nil)
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

// Conflict writes a 409 Conflict response.
func Conflict(w http.ResponseWriter, message string) {
	if message == "" {
		message = "conflict"
	}
	WriteError(w, http.StatusConflict, message)
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
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			WriteErrorResponse(w, r, http.StatusRequestEntityTooLarge, "", "request body too large", map[string]any{
				"limit_bytes": maxErr.Limit,
			})
			return false
		}
		BadRequest(w, "invalid request body")
		return false
	}
	return true
}

// DecodeJSONOptional decodes a JSON request body into the provided struct when present.
// It returns true when the body is empty and no decoding is needed.
func DecodeJSONOptional(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if r == nil || r.Body == nil || r.Body == http.NoBody {
		return true
	}

	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}

		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			WriteErrorResponse(w, r, http.StatusRequestEntityTooLarge, "", "request body too large", map[string]any{
				"limit_bytes": maxErr.Limit,
			})
			return false
		}
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

// StrictIdentityMode returns true when the service should only trust identity
// headers that are protected by verified mTLS.
func StrictIdentityMode() bool {
	return runtime.StrictIdentityMode()
}

func hasVerifiedMTLS(r *http.Request) bool {
	return r != nil && r.TLS != nil && len(r.TLS.VerifiedChains) > 0
}

// GetUserID extracts the user ID from the X-User-ID header.
// Returns empty string if not present.
func GetUserID(r *http.Request) string {
	// Prefer IDs injected by service-to-service authentication middleware.
	if userID := serviceauth.GetUserID(r.Context()); userID != "" {
		if runtime.StrictIdentityMode() && !hasVerifiedMTLS(r) {
			return ""
		}
		return userID
	}
	// Support user-auth middleware which stores IDs on the request context.
	if userID := logging.GetUserID(r.Context()); userID != "" {
		return userID
	}

	userID := r.Header.Get(serviceauth.UserIDHeader)
	if userID == "" {
		return ""
	}

	// In production, only trust headers when the request is authenticated over
	// verified mTLS (i.e., the caller is another Marble). We treat running on SGX
	// hardware (OE_SIMULATION=0) and MarbleRun-injected TLS credentials as strict
	// too, so a mis-set MARBLE_ENV cannot silently weaken trust boundaries.
	if runtime.StrictIdentityMode() && !hasVerifiedMTLS(r) {
		return ""
	}

	return userID
}

// GetUserRole extracts the user role from the X-User-Role header.
func GetUserRole(r *http.Request) string {
	// Prefer roles set by auth middleware on the request context.
	if role := logging.GetRole(r.Context()); role != "" {
		return role
	}

	role := r.Header.Get("X-User-Role")
	if role == "" {
		return ""
	}

	// In strict mode, only trust headers that are protected by verified mTLS.
	if runtime.StrictIdentityMode() && !hasVerifiedMTLS(r) {
		return ""
	}

	return role
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

var canonicalServiceIDs = map[string]struct{}{
	"gateway":     {},
	"neooracle":   {},
	"neorand":     {},
	"neovault":    {},
	"neostore":    {},
	"neofeeds":    {},
	"neoflow":     {},
	"neocompute":  {},
	"neoaccounts": {},
}

var serviceIDAliases = map[string]string{
	"vrf":          "neorand",
	"oracle":       "neooracle",
	"secrets":      "neostore",
	"accountpool":  "neoaccounts",
	"mixer":        "neovault",
	"datafeeds":    "neofeeds",
	"automation":   "neoflow",
	"confidential": "neocompute",
}

func canonicalizeServiceID(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if i := strings.IndexByte(raw, '.'); i >= 0 {
		raw = raw[:i]
	}
	if mapped, ok := serviceIDAliases[raw]; ok {
		raw = mapped
	}
	return raw
}

// CanonicalizeServiceID maps legacy service IDs and DNS names (e.g. "vrf",
// "accountpool") to the canonical service ID used throughout the service layer.
func CanonicalizeServiceID(raw string) string {
	return canonicalizeServiceID(raw)
}

func serviceIDFromVerifiedChains(chains [][]*x509.Certificate) string {
	if len(chains) == 0 || len(chains[0]) == 0 || chains[0][0] == nil {
		return ""
	}
	leaf := chains[0][0]

	for _, dnsName := range leaf.DNSNames {
		id := canonicalizeServiceID(dnsName)
		if _, ok := canonicalServiceIDs[id]; ok {
			return id
		}
	}

	for _, dnsName := range leaf.DNSNames {
		if id := canonicalizeServiceID(dnsName); id != "" {
			return id
		}
	}

	if cn := canonicalizeServiceID(leaf.Subject.CommonName); cn != "" {
		return cn
	}

	// As a last resort, return the raw CommonName if present (might be a UUID).
	return strings.ToLower(strings.TrimSpace(leaf.Subject.CommonName))
}

func serviceIDFromVerifiedMTLS(state *tls.ConnectionState) string {
	if state == nil || len(state.VerifiedChains) == 0 {
		return ""
	}
	return serviceIDFromVerifiedChains(state.VerifiedChains)
}

// GetServiceID returns the authenticated caller service ID.
// In strict environments (production/SGX/MarbleRun TLS), this is derived from
// the verified mTLS peer identity to avoid header spoofing.
func GetServiceID(r *http.Request) string {
	strict := runtime.StrictIdentityMode()
	if strict {
		// In strict environments, only trust the verified mTLS peer identity.
		derived := serviceIDFromVerifiedMTLS(r.TLS)
		if derived == "" {
			return ""
		}

		// If a service-auth middleware injected a service ID, ensure it matches the
		// peer identity. This prevents configuration drift and header/token spoofing.
		if ctxID := canonicalizeServiceID(serviceauth.GetServiceID(r.Context())); ctxID != "" && ctxID != derived {
			return ""
		}
		return derived
	}

	if serviceID := canonicalizeServiceID(serviceauth.GetServiceID(r.Context())); serviceID != "" {
		return serviceID
	}

	// Prefer verified mTLS identity when available even in non-strict mode.
	if id := serviceIDFromVerifiedMTLS(r.TLS); id != "" {
		return id
	}

	serviceID := canonicalizeServiceID(r.Header.Get(serviceauth.ServiceIDHeader))
	if serviceID == "" {
		return ""
	}
	return serviceID
}

// RequireServiceID extracts the service ID from the request context.
// Returns false and writes an error response if not present.
func RequireServiceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	serviceID := GetServiceID(r)
	if serviceID == "" {
		Unauthorized(w, "service authentication required")
		return "", false
	}
	return serviceID, true
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
