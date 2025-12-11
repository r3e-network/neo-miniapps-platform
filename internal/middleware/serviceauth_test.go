package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/golang-jwt/jwt/v5"
)

// =============================================================================
// Test Helpers
// =============================================================================

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key pair: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func generateValidServiceToken(t *testing.T, privateKey *rsa.PrivateKey, serviceID string, expiry time.Duration) string {
	t.Helper()
	now := time.Now()
	claims := &ServiceClaims{
		ServiceID: serviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			Issuer:    "service-layer",
			Subject:   serviceID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}
	return tokenString
}

func generateExpiredServiceToken(t *testing.T, privateKey *rsa.PrivateKey, serviceID string) string {
	t.Helper()
	now := time.Now()
	claims := &ServiceClaims{
		ServiceID: serviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			Issuer:    "service-layer",
			Subject:   serviceID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}
	return tokenString
}

func newTestServiceAuthMiddleware(t *testing.T, publicKey *rsa.PublicKey, allowedServices []string, requireUserID bool) *ServiceAuthMiddleware {
	t.Helper()
	logger := logging.New("test", "error", "text")
	return NewServiceAuthMiddleware(ServiceAuthConfig{
		PublicKey:       publicKey,
		Logger:          logger,
		AllowedServices: allowedServices,
		RequireUserID:   requireUserID,
		SkipPaths:       []string{"/health"},
	})
}

// =============================================================================
// ServiceAuthMiddleware Tests
// =============================================================================

func TestServiceAuthMiddleware_ValidToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	token := generateValidServiceToken(t, privateKey, "gateway", time.Hour)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceID := GetServiceID(r.Context())
		if serviceID != "gateway" {
			t.Errorf("Expected service_id 'gateway', got '%s'", serviceID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_MissingToken(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_InvalidToken(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, "invalid-token")

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	token := generateExpiredServiceToken(t, privateKey, "gateway")

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_UnauthorizedService(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	// Token for a service not in allowed list
	token := generateValidServiceToken(t, privateKey, "unknown-service", time.Hour)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_SkipPath(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	called := false
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should be called for skip path")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_RequireUserID_Missing(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, true)

	token := generateValidServiceToken(t, privateKey, "gateway", time.Hour)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)
	// No X-User-ID header

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_RequireUserID_Valid(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, true)

	token := generateValidServiceToken(t, privateKey, "gateway", time.Hour)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)
	req.Header.Set(UserIDHeader, "550e8400-e29b-41d4-a716-446655440000")

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserIDFromContext(r.Context())
		if userID != "550e8400-e29b-41d4-a716-446655440000" {
			t.Errorf("Expected user_id, got '%s'", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_InvalidUserIDFormat(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, true)

	token := generateValidServiceToken(t, privateKey, "gateway", time.Hour)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)
	req.Header.Set(UserIDHeader, "invalid-user-id")

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_AllServicesAllowed(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	// Empty allowed services list = allow all
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{}, false)

	token := generateValidServiceToken(t, privateKey, "any-service", time.Hour)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

// =============================================================================
// ServiceTokenGenerator Tests
// =============================================================================

func TestServiceTokenGenerator_GenerateToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	generator := NewServiceTokenGenerator(privateKey, "gateway", time.Hour)

	tokenString, err := generator.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Verify the token
	token, err := jwt.ParseWithClaims(tokenString, &ServiceClaims{}, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("Failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(*ServiceClaims)
	if !ok {
		t.Fatal("Invalid claims type")
	}

	if claims.ServiceID != "gateway" {
		t.Errorf("Expected service_id 'gateway', got '%s'", claims.ServiceID)
	}
}

func TestServiceTokenGenerator_DefaultExpiry(t *testing.T) {
	privateKey, _ := generateTestKeyPair(t)
	generator := NewServiceTokenGenerator(privateKey, "gateway", 0)

	if generator.expiry != DefaultServiceTokenExpiry {
		t.Errorf("Expected default expiry %v, got %v", DefaultServiceTokenExpiry, generator.expiry)
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestIsValidUserID(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		expected bool
	}{
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid UUID lowercase", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid UUID uppercase", "550E8400-E29B-41D4-A716-446655440000", true},
		{"invalid - too short", "550e8400-e29b-41d4-a716", false},
		{"invalid - too long", "550e8400-e29b-41d4-a716-4466554400001", false},
		{"invalid - no dashes", "550e8400e29b41d4a716446655440000", false},
		{"invalid - wrong format", "550e8400-e29b-41d4-a716446655440000", false},
		{"invalid - non-hex chars", "550e8400-e29b-41d4-a716-44665544000g", false},
		{"empty string", "", false},
		{"random string", "not-a-uuid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUserID(tt.userID)
			if result != tt.expected {
				t.Errorf("isValidUserID(%q) = %v, want %v", tt.userID, result, tt.expected)
			}
		})
	}
}

func TestGetServiceID(t *testing.T) {
	ctx := context.Background()

	// Empty context
	if id := GetServiceID(ctx); id != "" {
		t.Errorf("Expected empty string, got '%s'", id)
	}

	// With service ID
	ctx = context.WithValue(ctx, serviceIDKey, "gateway")
	if id := GetServiceID(ctx); id != "gateway" {
		t.Errorf("Expected 'gateway', got '%s'", id)
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	ctx := context.Background()

	// Empty context
	if id := GetUserIDFromContext(ctx); id != "" {
		t.Errorf("Expected empty string, got '%s'", id)
	}

	// With user ID
	ctx = context.WithValue(ctx, userIDKey, "user-123")
	if id := GetUserIDFromContext(ctx); id != "user-123" {
		t.Errorf("Expected 'user-123', got '%s'", id)
	}
}

// =============================================================================
// RequireServiceAuth Middleware Tests
// =============================================================================

func TestRequireServiceAuth_WithServiceID(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	ctx := context.WithValue(req.Context(), serviceIDKey, "gateway")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	called := false
	handler := RequireServiceAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestRequireServiceAuth_WithoutServiceID(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler := RequireServiceAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

// =============================================================================
// RequireUserIDHeader Middleware Tests
// =============================================================================

func TestRequireUserIDHeader_Valid(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(UserIDHeader, "550e8400-e29b-41d4-a716-446655440000")

	rr := httptest.NewRecorder()
	called := false
	handler := RequireUserIDHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Handler should be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestRequireUserIDHeader_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler := RequireUserIDHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestRequireUserIDHeader_Invalid(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(UserIDHeader, "invalid")

	rr := httptest.NewRecorder()
	handler := RequireUserIDHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

// =============================================================================
// Token Cache Tests
// =============================================================================

func TestServiceAuthMiddleware_TokenCaching(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	token := generateValidServiceToken(t, privateKey, "gateway", time.Hour)

	// First request - should validate and cache
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.Header.Set(ServiceTokenHeader, token)
	rr1 := httptest.NewRecorder()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", rr1.Code)
	}

	// Second request - should use cache
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set(ServiceTokenHeader, token)
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("Second request: expected status 200, got %d", rr2.Code)
	}

	// Verify token is cached
	middleware.mu.RLock()
	_, cached := middleware.validatedTokens[token]
	middleware.mu.RUnlock()

	if !cached {
		t.Error("Token should be cached")
	}
}

// =============================================================================
// Cache Cleanup Tests
// =============================================================================

func TestServiceAuthMiddleware_CacheCleanup(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{}, false)

	// Fill cache with >1000 entries to trigger cleanup
	for i := 0; i < 1010; i++ {
		token := generateValidServiceToken(t, privateKey, fmt.Sprintf("service-%d", i), time.Hour)
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set(ServiceTokenHeader, token)
		rr := httptest.NewRecorder()

		handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(rr, req)
	}

	// Verify cleanup was triggered (cache should be smaller now)
	middleware.mu.RLock()
	cacheSize := len(middleware.validatedTokens)
	middleware.mu.RUnlock()

	// After cleanup, expired entries should be removed
	// Since all tokens are valid, cache should still have entries
	if cacheSize == 0 {
		t.Error("Cache should not be empty after cleanup")
	}
}

func TestServiceAuthMiddleware_CacheExpiry(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	// Generate a token with very short expiry
	now := time.Now()
	claims := &ServiceClaims{
		ServiceID: "gateway",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Millisecond)),
			Issuer:    "service-layer",
			Subject:   "gateway",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	// First request - should validate and cache
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.Header.Set(ServiceTokenHeader, tokenString)
	rr1 := httptest.NewRecorder()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr1, req1)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Second request - token should be expired
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set(ServiceTokenHeader, tokenString)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	// Should fail because token is expired
	if rr2.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for expired token, got %d", rr2.Code)
	}
}

func TestServiceAuthMiddleware_WrongSigningKey(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	wrongPrivateKey, _ := generateTestKeyPair(t) // Different key pair
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	// Sign with wrong key
	token := generateValidServiceToken(t, wrongPrivateKey, "gateway", time.Hour)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, token)

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_MissingServiceIDClaim(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	// Create token without service_id claim
	now := time.Now()
	claims := &ServiceClaims{
		ServiceID: "", // Empty service ID
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			Issuer:    "service-layer",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, tokenString)

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestServiceAuthMiddleware_WrongSigningMethod(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	middleware := newTestServiceAuthMiddleware(t, publicKey, []string{"gateway"}, false)

	// Create token with HMAC instead of RSA
	now := time.Now()
	claims := &ServiceClaims{
		ServiceID: "gateway",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("secret"))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(ServiceTokenHeader, tokenString)

	rr := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestConstants(t *testing.T) {
	if ServiceTokenHeader != "X-Service-Token" {
		t.Errorf("ServiceTokenHeader = %s, want X-Service-Token", ServiceTokenHeader)
	}
	if ServiceIDHeader != "X-Service-ID" {
		t.Errorf("ServiceIDHeader = %s, want X-Service-ID", ServiceIDHeader)
	}
	if UserIDHeader != "X-User-ID" {
		t.Errorf("UserIDHeader = %s, want X-User-ID", UserIDHeader)
	}
	if DefaultServiceTokenExpiry != time.Hour {
		t.Errorf("DefaultServiceTokenExpiry = %v, want 1h", DefaultServiceTokenExpiry)
	}
}
