// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"net/http"

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
	// Keep the public TracingMiddleware API, but delegate to the shared
	// implementation used by the gateway and services.
	return LoggingMiddleware(m.logger)(next)
}

// Note: responseWriter type is defined in metrics.go
