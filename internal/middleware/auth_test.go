package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func generateTestToken(t *testing.T, privateKey *rsa.PrivateKey, userID string, expired bool) string {
	claims := &Claims{
		UserID:     userID,
		Email:      "test@example.com",
		AuthMethod: "test",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	if expired {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	return tokenString
}

func TestNewAuthMiddleware(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")
	skipPaths := []string{"/health", "/metrics"}

	middleware := NewAuthMiddleware(publicKey, logger, skipPaths)

	if middleware == nil {
		t.Fatal("NewAuthMiddleware() returned nil")
	}

	if middleware.publicKey != publicKey {
		t.Error("publicKey not set correctly")
	}

	if middleware.logger != logger {
		t.Error("logger not set correctly")
	}

	if len(middleware.skipPaths) != 2 {
		t.Errorf("skipPaths length = %d, want 2", len(middleware.skipPaths))
	}

	if !middleware.skipPaths["/health"] {
		t.Error("skipPaths does not contain /health")
	}
}

func TestAuthMiddleware_Handler_SkipPaths(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")
	skipPaths := []string{"/health"}

	middleware := NewAuthMiddleware(publicKey, logger, skipPaths)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_Handler_MissingAuthHeader(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	middleware := NewAuthMiddleware(publicKey, logger, nil)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_Handler_InvalidAuthHeaderFormat(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	middleware := NewAuthMiddleware(publicKey, logger, nil)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "token123"},
		{"wrong prefix", "Basic token123"},
		{"empty token", "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Status code = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthMiddleware_Handler_ValidToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	middleware := NewAuthMiddleware(publicKey, logger, nil)

	var capturedUserID string
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	token := generateTestToken(t, privateKey, "user-123", false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if capturedUserID != "user-123" {
		t.Errorf("User ID = %v, want user-123", capturedUserID)
	}
}

func TestAuthMiddleware_Handler_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	middleware := NewAuthMiddleware(publicKey, logger, nil)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := generateTestToken(t, privateKey, "user-123", true)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_Handler_InvalidToken(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	middleware := NewAuthMiddleware(publicKey, logger, nil)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_Handler_WrongSigningKey(t *testing.T) {
	privateKey1, _ := generateTestKeys(t)
	_, publicKey2 := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	// Create middleware with publicKey2
	middleware := NewAuthMiddleware(publicKey2, logger, nil)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Generate token with privateKey1
	token := generateTestToken(t, privateKey1, "user-123", false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_validateToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	middleware := NewAuthMiddleware(publicKey, logger, nil)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   generateTestToken(t, privateKey, "user-123", false),
			wantErr: false,
		},
		{
			name:    "expired token",
			token:   generateTestToken(t, privateKey, "user-123", true),
			wantErr: true,
		},
		{
			name:    "invalid token",
			token:   "invalid.token.here",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := middleware.validateToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && claims == nil {
				t.Error("validateToken() returned nil claims without error")
			}

			if !tt.wantErr && claims.UserID != "user-123" {
				t.Errorf("UserID = %v, want user-123", claims.UserID)
			}
		})
	}
}

func TestGetUserID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "with user ID",
			ctx:  logging.WithUserID(context.Background(), "user-123"),
			want: "user-123",
		},
		{
			name: "without user ID",
			ctx:  context.Background(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUserID(tt.ctx); got != tt.want {
				t.Errorf("GetUserID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequireUserID(t *testing.T) {
	handler := RequireUserID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		ctx        context.Context
		wantStatus int
	}{
		{
			name:       "with user ID",
			ctx:        logging.WithUserID(context.Background(), "user-123"),
			wantStatus: http.StatusOK,
		},
		{
			name:       "without user ID",
			ctx:        context.Background(),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req = req.WithContext(tt.ctx)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("Status code = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestAuthMiddleware_Handler_PreservesTraceID(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	logger := logging.New("test", "info", "json")

	middleware := NewAuthMiddleware(publicKey, logger, nil)

	var capturedTraceID string
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTraceID = logging.GetTraceID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	token := generateTestToken(t, privateKey, "user-123", false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	ctx := logging.WithTraceID(req.Context(), "trace-456")
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if capturedTraceID != "trace-456" {
		t.Errorf("Trace ID = %v, want trace-456", capturedTraceID)
	}
}

func TestClaims_Structure(t *testing.T) {
	claims := &Claims{
		UserID:     "user-123",
		Email:      "test@example.com",
		NeoAddress: "NXXXabc123",
		AuthMethod: "google",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	if claims.UserID != "user-123" {
		t.Errorf("UserID = %v, want user-123", claims.UserID)
	}

	if claims.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", claims.Email)
	}

	if claims.NeoAddress != "NXXXabc123" {
		t.Errorf("NeoAddress = %v, want NXXXabc123", claims.NeoAddress)
	}

	if claims.AuthMethod != "google" {
		t.Errorf("AuthMethod = %v, want google", claims.AuthMethod)
	}
}
