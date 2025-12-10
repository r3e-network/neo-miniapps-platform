// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"net/http"
	"strings"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
type CORSMiddleware struct {
	allowedOrigins []string
	allowAll       bool
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(allowedOrigins []string) *CORSMiddleware {
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
			break
		}
	}

	return &CORSMiddleware{
		allowedOrigins: allowedOrigins,
		allowAll:       allowAll,
	}
}

// Handler returns the CORS middleware handler
func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		if m.allowAll || m.isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Trace-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Trace-ID")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if an origin is in the allowed list
func (m *CORSMiddleware) isOriginAllowed(origin string) bool {
	for _, allowed := range m.allowedOrigins {
		if allowed == origin || strings.HasSuffix(origin, allowed) {
			return true
		}
	}
	return false
}
