// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/logging"
)

// TracingMiddleware adds trace ID to all requests
type TracingMiddleware struct {
	logger *logging.Logger
}

// NewTracingMiddleware creates a new tracing middleware
func NewTracingMiddleware(logger *logging.Logger) *TracingMiddleware {
	return &TracingMiddleware{
		logger: logger,
	}
}

// Handler returns the tracing middleware handler
func (m *TracingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate or extract trace ID
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = logging.NewTraceID()
		}

		// Add trace ID to context
		ctx := logging.WithTraceID(r.Context(), traceID)

		// Add trace ID to response header
		w.Header().Set("X-Trace-ID", traceID)

		// Create response writer wrapper to capture status code
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Record start time
		start := time.Now()

		// Process request
		next.ServeHTTP(rw, r.WithContext(ctx))

		// Log request
		duration := time.Since(start)
		m.logger.LogRequest(ctx, r.Method, r.URL.Path, rw.statusCode, duration)
	})
}

// Note: responseWriter type is defined in metrics.go
