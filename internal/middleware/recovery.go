// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/R3E-Network/service_layer/internal/errors"
	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/logging"
)

// RecoveryMiddleware recovers from panics and logs them
type RecoveryMiddleware struct {
	logger *logging.Logger
}

// NewRecoveryMiddleware creates a new recovery middleware
func NewRecoveryMiddleware(logger *logging.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		logger: logger,
	}
}

// Handler returns the recovery middleware handler
func (m *RecoveryMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				stack := debug.Stack()
				m.logger.WithContext(r.Context()).WithFields(map[string]interface{}{
					"panic":       fmt.Sprintf("%v", err),
					"stack":       string(stack),
					"path":        r.URL.Path,
					"method":      r.Method,
					"remote_addr": r.RemoteAddr,
				}).Error("Panic recovered")

				// Send error response
				serviceErr := errors.Internal("Internal server error", fmt.Errorf("%v", err))
				httputil.WriteErrorResponse(w, r, serviceErr.HTTPStatus, string(serviceErr.Code), serviceErr.Message, serviceErr.Details)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
