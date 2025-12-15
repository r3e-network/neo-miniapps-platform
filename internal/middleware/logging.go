// Package middleware provides HTTP middleware for the service layer.
package middleware

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/logging"
)

// LoggingMiddleware logs HTTP requests with trace ID.
func LoggingMiddleware(logger *logging.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate or extract trace ID
			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = logging.NewTraceID()
			}

			// Add trace ID to context
			ctx := logging.WithTraceID(r.Context(), traceID)
			r = r.WithContext(ctx)

			// Ensure downstream handlers (including reverse proxies) can forward the trace ID.
			r.Header.Set("X-Trace-ID", traceID)

			// Add trace ID to response header
			w.Header().Set("X-Trace-ID", traceID)

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Log request
			duration := time.Since(start)
			logger.LogRequest(ctx, r.Method, r.URL.Path, wrapped.statusCode, duration)
		})
	}
}
