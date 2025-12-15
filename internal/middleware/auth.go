// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/R3E-Network/service_layer/internal/errors"
	internalhttputil "github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/logging"
)

// Claims represents JWT claims
type Claims struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email,omitempty"`
	NeoAddress string `json:"neo_address,omitempty"`
	AuthMethod string `json:"auth_method"`
	Role       string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// AuthMiddleware provides JWT authentication
type AuthMiddleware struct {
	publicKey interface{}
	logger    *logging.Logger
	skipPaths map[string]bool
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(publicKey interface{}, logger *logging.Logger, skipPaths []string) *AuthMiddleware {
	skip := make(map[string]bool)
	for _, path := range skipPaths {
		skip[path] = true
	}

	return &AuthMiddleware{
		publicKey: publicKey,
		logger:    logger,
		skipPaths: skip,
	}
}

// Handler returns the middleware handler
func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for certain paths
		if m.skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.respondError(w, r, errors.Unauthorized("Missing Authorization header"))
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			m.respondError(w, r, errors.Unauthorized("Invalid Authorization header format"))
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		claims, err := m.validateToken(tokenString)
		if err != nil {
			m.logger.WithContext(r.Context()).WithError(err).Warn("Token validation failed")
			m.respondError(w, r, err)
			return
		}

		// Add claims to context
		ctx := context.WithValue(r.Context(), logging.UserIDKey, claims.UserID)
		if claims.Role != "" {
			ctx = context.WithValue(ctx, logging.RoleKey, claims.Role)
		}
		ctx = logging.WithTraceID(ctx, logging.GetTraceID(r.Context()))

		// Log successful authentication
		m.logger.WithContext(ctx).WithFields(map[string]interface{}{
			"user_id":     claims.UserID,
			"auth_method": claims.AuthMethod,
		}).Debug("Authentication successful")

		// Continue with authenticated request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateToken validates a JWT token and returns claims
func (m *AuthMiddleware) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.InvalidToken(nil).WithDetails("method", token.Header["alg"])
		}
		return m.publicKey, nil
	})

	if err != nil {
		return nil, errors.InvalidToken(err)
	}

	if !token.Valid {
		return nil, errors.InvalidToken(nil)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.InvalidToken(nil).WithDetails("reason", "invalid claims type")
	}

	return claims, nil
}

// respondError sends an error response
func (m *AuthMiddleware) respondError(w http.ResponseWriter, r *http.Request, err error) {
	serviceErr := errors.GetServiceError(err)
	if serviceErr == nil {
		serviceErr = errors.Internal("Authentication failed", err)
	}

	internalhttputil.WriteErrorResponse(w, r, serviceErr.HTTPStatus, string(serviceErr.Code), serviceErr.Message, serviceErr.Details)

	// Log the error
	m.logger.WithContext(r.Context()).WithError(err).WithFields(map[string]interface{}{
		"path":   r.URL.Path,
		"method": r.Method,
		"status": serviceErr.HTTPStatus,
	}).Warn("Authentication failed")
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	return logging.GetUserID(ctx)
}

// GetUserRole extracts user role from context
func GetUserRole(ctx context.Context) string {
	return logging.GetRole(ctx)
}

// RequireUserID middleware ensures user ID is present in context
func RequireUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		if userID == "" {
			internalhttputil.Unauthorized(w, "")
			return
		}
		next.ServeHTTP(w, r)
	})
}
