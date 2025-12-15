// Package middleware provides HTTP middleware for the service layer.
package middleware

import (
	"net/http"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

const defaultMaxRequestBodyBytes int64 = 8 << 20 // 8MiB

// BodyLimitMiddleware caps request bodies to reduce memory/CPU DoS risk.
// It applies http.MaxBytesReader so downstream handlers/decoders cannot read
// beyond the configured limit.
type BodyLimitMiddleware struct {
	maxBytes int64
}

// NewBodyLimitMiddleware creates a request body limiting middleware.
// When maxBytes <= 0, a conservative default is applied.
func NewBodyLimitMiddleware(maxBytes int64) *BodyLimitMiddleware {
	if maxBytes <= 0 {
		maxBytes = defaultMaxRequestBodyBytes
	}
	return &BodyLimitMiddleware{maxBytes: maxBytes}
}

// Handler returns the body limiting middleware handler.
func (m *BodyLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || m.maxBytes <= 0 || r == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Fast-path reject when Content-Length is known and too large.
		if r.ContentLength > m.maxBytes {
			httputil.WriteErrorResponse(
				w,
				r,
				http.StatusRequestEntityTooLarge,
				"",
				"request body too large",
				map[string]any{"limit_bytes": m.maxBytes},
			)
			return
		}

		if r.Body != nil && r.Body != http.NoBody {
			r.Body = http.MaxBytesReader(w, r.Body, m.maxBytes)
		}

		next.ServeHTTP(w, r)
	})
}
