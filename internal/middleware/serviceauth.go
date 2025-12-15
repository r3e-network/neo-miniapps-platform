// Package middleware provides HTTP middleware for the service layer.
package middleware

import (
	"context"
	"crypto/rsa"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/R3E-Network/service_layer/internal/errors"
	internalhttputil "github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

// =============================================================================
// Service Authentication Constants
// =============================================================================

const (
	// ServiceTokenHeader is the header name for service-to-service tokens.
	ServiceTokenHeader = serviceauth.ServiceTokenHeader

	// ServiceIDHeader is the header name for service identification.
	ServiceIDHeader = serviceauth.ServiceIDHeader

	// UserIDHeader is the header name for user identification.
	UserIDHeader = serviceauth.UserIDHeader

	// DefaultServiceTokenExpiry is the default expiration time for service tokens.
	DefaultServiceTokenExpiry = serviceauth.DefaultServiceTokenExpiry
)

// =============================================================================
// Service Claims
// =============================================================================

// ServiceClaims represents JWT claims for service-to-service authentication.
type ServiceClaims = serviceauth.ServiceClaims

// ServiceTokenGenerator generates service-to-service JWT tokens.
type ServiceTokenGenerator = serviceauth.ServiceTokenGenerator

// ServiceTokenRoundTripper injects X-Service-Token (and optionally X-User-ID)
// into outgoing HTTP requests.
type ServiceTokenRoundTripper = serviceauth.ServiceTokenRoundTripper

// NewServiceTokenGenerator creates a new service token generator.
func NewServiceTokenGenerator(privateKey *rsa.PrivateKey, serviceID string, expiry time.Duration) *ServiceTokenGenerator {
	return serviceauth.NewServiceTokenGenerator(privateKey, serviceID, expiry)
}

// NewServiceTokenRoundTripper wraps a base transport with service-token injection.
func NewServiceTokenRoundTripper(base http.RoundTripper, generator *ServiceTokenGenerator) http.RoundTripper {
	return serviceauth.NewServiceTokenRoundTripper(base, generator)
}

// =============================================================================
// Service Auth Middleware
// =============================================================================

// ServiceAuthMiddleware provides service-to-service JWT authentication.
type ServiceAuthMiddleware struct {
	publicKey       *rsa.PublicKey
	logger          *logging.Logger
	allowedServices map[string]bool
	requireUserID   bool
	skipPaths       map[string]bool
	mu              sync.RWMutex
	validatedTokens map[string]*cachedToken // In-memory cache for validated tokens
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

	logger := cfg.Logger
	if logger == nil {
		logger = logging.New("serviceauth", "info", "json")
	}

	return &ServiceAuthMiddleware{
		publicKey:       cfg.PublicKey,
		logger:          logger,
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
		ctx := serviceauth.WithServiceID(r.Context(), claims.ServiceID)
		if userID != "" {
			ctx = serviceauth.WithUserID(ctx, userID)
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
	if m.publicKey == nil {
		return nil, errors.Internal("service authentication is not configured", nil)
	}

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

	// Defense in depth: ensure token was minted by our service-layer issuer.
	if claims.Issuer != "service-layer" {
		return nil, errors.InvalidToken(nil).WithDetails("reason", "invalid issuer")
	}
	// Keep Subject consistent with the service identity.
	if claims.Subject != "" && claims.Subject != claims.ServiceID {
		return nil, errors.InvalidToken(nil).WithDetails("reason", "subject/service mismatch")
	}

	// Cache the validated token
	m.cacheToken(tokenString, claims)

	return claims, nil
}

// getCachedToken retrieves a cached token if valid.
func (m *ServiceAuthMiddleware) getCachedToken(tokenString string) *ServiceClaims {
	m.mu.RLock()
	cached, ok := m.validatedTokens[tokenString]
	if !ok {
		m.mu.RUnlock()
		return nil
	}

	// Check if cache entry has expired
	if time.Now().After(cached.expiresAt) {
		m.mu.RUnlock()
		m.mu.Lock()
		// Re-check under write lock before deleting.
		if current, ok := m.validatedTokens[tokenString]; ok && time.Now().After(current.expiresAt) {
			delete(m.validatedTokens, tokenString)
		}
		m.mu.Unlock()
		return nil
	}

	m.mu.RUnlock()
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

	internalhttputil.WriteErrorResponse(w, r, serviceErr.HTTPStatus, string(serviceErr.Code), serviceErr.Message, serviceErr.Details)

	m.logger.WithContext(r.Context()).WithError(err).WithFields(map[string]interface{}{
		"path":   r.URL.Path,
		"method": r.Method,
		"status": serviceErr.HTTPStatus,
	}).Warn("Service authentication failed")
}

// =============================================================================
// Helper Functions
// =============================================================================

// GetServiceID extracts service ID from context.
func GetServiceID(ctx context.Context) string {
	return serviceauth.GetServiceID(ctx)
}

// GetUserIDFromContext extracts user ID from context.
func GetUserIDFromContext(ctx context.Context) string {
	return serviceauth.GetUserID(ctx)
}

// WithServiceID returns a new context with the service ID set.
// This is useful for propagating service identity through internal calls.
func WithServiceID(ctx context.Context, serviceID string) context.Context {
	return serviceauth.WithServiceID(ctx, serviceID)
}

// WithUserID returns a new context with the user ID set.
// This is useful for propagating user ID through service-to-service calls.
func WithUserID(ctx context.Context, userID string) context.Context {
	return serviceauth.WithUserID(ctx, userID)
}

// ParseRSAPublicKeyFromPEM parses an RSA public key from PEM bytes.
// Supported PEM types: PUBLIC KEY (PKIX), RSA PUBLIC KEY (PKCS#1), CERTIFICATE.
func ParseRSAPublicKeyFromPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	return serviceauth.ParseRSAPublicKeyFromPEM(pemBytes)
}

// ParseRSAPrivateKeyFromPEM parses an RSA private key from PEM bytes.
// Supported PEM types: RSA PRIVATE KEY (PKCS#1), PRIVATE KEY (PKCS#8).
func ParseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	return serviceauth.ParseRSAPrivateKeyFromPEM(pemBytes)
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
			internalhttputil.WriteErrorResponse(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "service authentication required", nil)
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
			internalhttputil.WriteErrorResponse(w, r, http.StatusUnauthorized, "USER_ID_REQUIRED", "X-User-ID header required", nil)
			return
		}
		if !isValidUserID(userID) {
			internalhttputil.WriteErrorResponse(w, r, http.StatusBadRequest, "INVALID_USER_ID", "invalid X-User-ID format", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
