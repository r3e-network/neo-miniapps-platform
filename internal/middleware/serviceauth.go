// Package middleware provides HTTP middleware for the service layer.
package middleware

import (
	"context"
	"crypto/rsa"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/errors"
	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/golang-jwt/jwt/v5"
)

// =============================================================================
// Service Authentication Constants
// =============================================================================

const (
	// ServiceTokenHeader is the header name for service-to-service tokens.
	ServiceTokenHeader = "X-Service-Token"

	// ServiceIDHeader is the header name for service identification.
	ServiceIDHeader = "X-Service-ID"

	// UserIDHeader is the header name for user identification.
	UserIDHeader = "X-User-ID"

	// DefaultServiceTokenExpiry is the default expiration time for service tokens.
	DefaultServiceTokenExpiry = 1 * time.Hour
)

// Context keys for service authentication.
type contextKey string

const (
	serviceIDKey contextKey = "service_id"
	userIDKey    contextKey = "user_id"
)

// =============================================================================
// Service Claims
// =============================================================================

// ServiceClaims represents JWT claims for service-to-service authentication.
type ServiceClaims struct {
	ServiceID string `json:"service_id"`
	jwt.RegisteredClaims
}

// =============================================================================
// Service Auth Middleware
// =============================================================================

// ServiceAuthMiddleware provides service-to-service JWT authentication.
type ServiceAuthMiddleware struct {
	publicKey        *rsa.PublicKey
	logger           *logging.Logger
	allowedServices  map[string]bool
	requireUserID    bool
	skipPaths        map[string]bool
	mu               sync.RWMutex
	validatedTokens  map[string]*cachedToken // In-memory cache for validated tokens
}

// cachedToken stores validated token info with expiry.
type cachedToken struct {
	claims    *ServiceClaims
	expiresAt time.Time
}

// ServiceAuthConfig configures the service authentication middleware.
type ServiceAuthConfig struct {
	PublicKey       *rsa.PublicKey
	Logger          *logging.Logger
	AllowedServices []string
	RequireUserID   bool
	SkipPaths       []string
}

// NewServiceAuthMiddleware creates a new service authentication middleware.
func NewServiceAuthMiddleware(cfg ServiceAuthConfig) *ServiceAuthMiddleware {
	allowed := make(map[string]bool)
	for _, svc := range cfg.AllowedServices {
		allowed[svc] = true
	}

	skip := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skip[path] = true
	}

	return &ServiceAuthMiddleware{
		publicKey:       cfg.PublicKey,
		logger:          cfg.Logger,
		allowedServices: allowed,
		requireUserID:   cfg.RequireUserID,
		skipPaths:       skip,
		validatedTokens: make(map[string]*cachedToken),
	}
}

// Handler returns the middleware handler function.
func (m *ServiceAuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for certain paths
		if m.skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Validate service token
		serviceToken := r.Header.Get(ServiceTokenHeader)
		if serviceToken == "" {
			m.respondError(w, r, errors.Unauthorized("Missing service token"))
			return
		}

		// Validate and extract claims
		claims, err := m.validateServiceToken(serviceToken)
		if err != nil {
			m.logger.WithContext(r.Context()).WithError(err).Warn("Service token validation failed")
			m.respondError(w, r, err)
			return
		}

		// Check if service is allowed
		if !m.isServiceAllowed(claims.ServiceID) {
			m.logger.WithContext(r.Context()).WithFields(map[string]interface{}{
				"service_id": claims.ServiceID,
			}).Warn("Service not in allowed list")
			m.respondError(w, r, errors.Forbidden("Service not authorized"))
			return
		}

		// Validate X-User-ID header if required
		userID := r.Header.Get(UserIDHeader)
		if m.requireUserID && userID == "" {
			m.respondError(w, r, errors.Unauthorized("Missing X-User-ID header"))
			return
		}

		// Validate X-User-ID format (UUID format check)
		if userID != "" && !isValidUserID(userID) {
			m.respondError(w, r, errors.InvalidFormat("X-User-ID", "UUID format required"))
			return
		}

		// Add service ID and user ID to context
		ctx := context.WithValue(r.Context(), serviceIDKey, claims.ServiceID)
		if userID != "" {
			ctx = context.WithValue(ctx, userIDKey, userID)
		}

		// Log successful authentication
		m.logger.WithContext(ctx).WithFields(map[string]interface{}{
			"service_id": claims.ServiceID,
			"user_id":    userID,
		}).Debug("Service authentication successful")

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateServiceToken validates a service JWT token.
func (m *ServiceAuthMiddleware) validateServiceToken(tokenString string) (*ServiceClaims, error) {
	// Check cache first
	if cached := m.getCachedToken(tokenString); cached != nil {
		return cached, nil
	}

	// Parse and validate token
	token, err := jwt.ParseWithClaims(tokenString, &ServiceClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method is RS256
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

	claims, ok := token.Claims.(*ServiceClaims)
	if !ok {
		return nil, errors.InvalidToken(nil).WithDetails("reason", "invalid claims type")
	}

	if claims.ServiceID == "" {
		return nil, errors.InvalidToken(nil).WithDetails("reason", "missing service_id claim")
	}

	// Cache the validated token
	m.cacheToken(tokenString, claims)

	return claims, nil
}

// getCachedToken retrieves a cached token if valid.
func (m *ServiceAuthMiddleware) getCachedToken(tokenString string) *ServiceClaims {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cached, ok := m.validatedTokens[tokenString]
	if !ok {
		return nil
	}

	// Check if cache entry has expired
	if time.Now().After(cached.expiresAt) {
		return nil
	}

	return cached.claims
}

// cacheToken stores a validated token in cache.
func (m *ServiceAuthMiddleware) cacheToken(tokenString string, claims *ServiceClaims) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cache for 5 minutes or until token expiry, whichever is sooner
	cacheExpiry := time.Now().Add(5 * time.Minute)
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(cacheExpiry) {
		cacheExpiry = claims.ExpiresAt.Time
	}

	m.validatedTokens[tokenString] = &cachedToken{
		claims:    claims,
		expiresAt: cacheExpiry,
	}

	// Cleanup old entries if cache is too large
	if len(m.validatedTokens) > 1000 {
		m.cleanupCache()
	}
}

// cleanupCache removes expired entries from the cache.
func (m *ServiceAuthMiddleware) cleanupCache() {
	now := time.Now()
	for key, cached := range m.validatedTokens {
		if now.After(cached.expiresAt) {
			delete(m.validatedTokens, key)
		}
	}
}

// isServiceAllowed checks if a service is in the allowed list.
func (m *ServiceAuthMiddleware) isServiceAllowed(serviceID string) bool {
	// If no allowed services configured, allow all
	if len(m.allowedServices) == 0 {
		return true
	}
	return m.allowedServices[serviceID]
}

// respondError sends an error response.
func (m *ServiceAuthMiddleware) respondError(w http.ResponseWriter, r *http.Request, err error) {
	serviceErr := errors.GetServiceError(err)
	if serviceErr == nil {
		serviceErr = errors.Internal("Service authentication failed", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(serviceErr.HTTPStatus)

	m.logger.WithContext(r.Context()).WithError(err).WithFields(map[string]interface{}{
		"path":   r.URL.Path,
		"method": r.Method,
		"status": serviceErr.HTTPStatus,
	}).Warn("Service authentication failed")
}

// =============================================================================
// Service Token Generator
// =============================================================================

// ServiceTokenGenerator generates service-to-service JWT tokens.
type ServiceTokenGenerator struct {
	privateKey *rsa.PrivateKey
	serviceID  string
	expiry     time.Duration
}

// NewServiceTokenGenerator creates a new service token generator.
func NewServiceTokenGenerator(privateKey *rsa.PrivateKey, serviceID string, expiry time.Duration) *ServiceTokenGenerator {
	if expiry == 0 {
		expiry = DefaultServiceTokenExpiry
	}
	return &ServiceTokenGenerator{
		privateKey: privateKey,
		serviceID:  serviceID,
		expiry:     expiry,
	}
}

// GenerateToken generates a new service token.
func (g *ServiceTokenGenerator) GenerateToken() (string, error) {
	now := time.Now()
	claims := &ServiceClaims{
		ServiceID: g.serviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(g.expiry)),
			Issuer:    "service-layer",
			Subject:   g.serviceID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.privateKey)
}

// =============================================================================
// Helper Functions
// =============================================================================

// GetServiceID extracts service ID from context.
func GetServiceID(ctx context.Context) string {
	if v := ctx.Value(serviceIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserIDFromContext extracts user ID from context.
func GetUserIDFromContext(ctx context.Context) string {
	if v := ctx.Value(userIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// WithUserID returns a new context with the user ID set.
// This is useful for propagating user ID through service-to-service calls.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// isValidUserID validates user ID format (UUID).
func isValidUserID(userID string) bool {
	// Basic UUID format validation: 8-4-4-4-12 hex characters
	if len(userID) != 36 {
		return false
	}

	// Check format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	parts := strings.Split(userID, "-")
	if len(parts) != 5 {
		return false
	}

	expectedLengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedLengths[i] {
			return false
		}
		for _, c := range part {
			if !isHexChar(c) {
				return false
			}
		}
	}

	return true
}

// isHexChar checks if a character is a valid hexadecimal character.
func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// RequireServiceAuth is a simple middleware that requires service authentication.
// Use this for endpoints that must only be called by authenticated services.
func RequireServiceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceID := GetServiceID(r.Context())
		if serviceID == "" {
			http.Error(w, `{"error":"service authentication required"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireUserIDHeader is a middleware that requires X-User-ID header.
func RequireUserIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(UserIDHeader)
		if userID == "" {
			http.Error(w, `{"error":"X-User-ID header required"}`, http.StatusUnauthorized)
			return
		}
		if !isValidUserID(userID) {
			http.Error(w, `{"error":"invalid X-User-ID format"}`, http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
