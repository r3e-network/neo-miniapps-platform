// Package middleware provides HTTP middleware functions
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/metrics"
)

// MetricsMiddleware records HTTP metrics for each request
func MetricsMiddleware(serviceName string, m *metrics.Metrics) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Increment in-flight requests
			m.IncrementInFlight()
			defer m.DecrementInFlight()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start)
			status := strconv.Itoa(wrapped.statusCode)
			path := r.URL.Path

			// Use route pattern if available
			if route := mux.CurrentRoute(r); route != nil {
				if pathTemplate, err := route.GetPathTemplate(); err == nil {
					path = pathTemplate
				}
			}

			m.RecordHTTPRequest(serviceName, r.Method, path, status, duration)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}
