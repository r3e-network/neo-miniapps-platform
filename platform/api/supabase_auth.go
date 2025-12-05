// Package api provides HTTP API components.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SupabaseUser represents an authenticated Supabase user.
type SupabaseUser struct {
	ID            string                 `json:"id"`
	Email         string                 `json:"email"`
	Phone         string                 `json:"phone,omitempty"`
	Role          string                 `json:"role"`
	AppMetadata   map[string]interface{} `json:"app_metadata,omitempty"`
	UserMetadata  map[string]interface{} `json:"user_metadata,omitempty"`
	Aud           string                 `json:"aud"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	LastSignInAt  time.Time              `json:"last_sign_in_at,omitempty"`
	EmailVerified bool                   `json:"email_confirmed_at,omitempty"`
}

// SupabaseAuthConfig holds Supabase auth configuration.
type SupabaseAuthConfig struct {
	URL        string
	AnonKey    string
	ServiceKey string
	JWTSecret  string
}

// DefaultSupabaseAuthConfig returns config from environment variables.
func DefaultSupabaseAuthConfig() SupabaseAuthConfig {
	return SupabaseAuthConfig{
		URL:        os.Getenv("SUPABASE_URL"),
		AnonKey:    os.Getenv("SUPABASE_ANON_KEY"),
		ServiceKey: os.Getenv("SUPABASE_SERVICE_KEY"),
		JWTSecret:  os.Getenv("SUPABASE_JWT_SECRET"),
	}
}

// SupabaseAuthMiddleware provides Supabase JWT authentication.
type SupabaseAuthMiddleware struct {
	config SupabaseAuthConfig
	client *http.Client
}

// NewSupabaseAuthMiddleware creates a new Supabase auth middleware.
func NewSupabaseAuthMiddleware(config SupabaseAuthConfig) *SupabaseAuthMiddleware {
	return &SupabaseAuthMiddleware{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// contextKey is a custom type for context keys.
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "supabase_user"
	// TokenContextKey stores the raw bearer token for downstream RLS usage.
	TokenContextKey contextKey = "supabase_token"
)

// Middleware returns an HTTP middleware that validates Supabase JWT tokens.
func (m *SupabaseAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Allow unauthenticated requests to pass through
			// Individual handlers can check for user presence
			next.ServeHTTP(w, r)
			return
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}
		token := parts[1]

		// Validate token with Supabase
		user, err := m.validateToken(r.Context(), token)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid token: %s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, TokenContextKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuth returns a middleware that requires authentication.
func (m *SupabaseAuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole returns a middleware that requires a specific role.
func (m *SupabaseAuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}
			if user.Role != role && user.Role != "service_role" {
				http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// validateToken validates a JWT token with Supabase.
func (m *SupabaseAuthMiddleware) validateToken(ctx context.Context, token string) (*SupabaseUser, error) {
	// Prefer local verification with the Supabase JWT secret (avoids network dependency).
	if m.config.JWTSecret != "" {
		if user, err := m.validateTokenLocal(token); err == nil {
			return user, nil
		}
	}

	// Fallback: validate against Supabase Auth REST API.
	return m.validateTokenWithSupabase(ctx, token)
}

// validateTokenLocal verifies the JWT signature locally using the Supabase JWT secret.
func (m *SupabaseAuthMiddleware) validateTokenLocal(token string) (*SupabaseUser, error) {
	claims := jwt.MapClaims{}

	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(m.config.JWTSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt parse: %w", err)
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("jwt invalid")
	}

	user := &SupabaseUser{
		ID:           getStringClaim(claims, "sub"),
		Email:        getStringClaim(claims, "email"),
		Phone:        getStringClaim(claims, "phone"),
		Role:         getStringClaim(claims, "role"),
		Aud:          getStringClaim(claims, "aud"),
		AppMetadata:  getMapClaim(claims, "app_metadata"),
		UserMetadata: getMapClaim(claims, "user_metadata"),
	}

	user.CreatedAt = getTimeClaim(claims, "iat")
	user.UpdatedAt = getTimeClaim(claims, "updated_at")
	user.LastSignInAt = getTimeClaim(claims, "last_sign_in_at")
	if t := getTimeClaim(claims, "email_confirmed_at"); !t.IsZero() {
		user.EmailVerified = true
	}

	return user, nil
}

// validateTokenWithSupabase validates a JWT token via Supabase Auth REST API.
func (m *SupabaseAuthMiddleware) validateTokenWithSupabase(ctx context.Context, token string) (*SupabaseUser, error) {
	// Call Supabase auth API to validate token
	req, err := http.NewRequestWithContext(ctx, "GET", m.config.URL+"/auth/v1/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", m.config.AnonKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token validation failed: %s", string(body))
	}

	var user SupabaseUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// GetUserFromContext retrieves the authenticated user from context.
func GetUserFromContext(ctx context.Context) *SupabaseUser {
	user, ok := ctx.Value(UserContextKey).(*SupabaseUser)
	if !ok {
		return nil
	}
	return user
}

// GetUserID retrieves the user ID from context.
func GetUserID(ctx context.Context) string {
	user := GetUserFromContext(ctx)
	if user == nil {
		return ""
	}
	return user.ID
}

// GetTokenFromContext retrieves the bearer token from context (for RLS).
func GetTokenFromContext(ctx context.Context) string {
	token, ok := ctx.Value(TokenContextKey).(string)
	if !ok {
		return ""
	}
	return token
}

// =============================================================================
// API Key Authentication (for service-to-service calls)
// =============================================================================

// APIKeyMiddleware provides API key authentication.
type APIKeyMiddleware struct {
	validateKey func(ctx context.Context, key string) (string, error)
}

// NewAPIKeyMiddleware creates a new API key middleware.
func NewAPIKeyMiddleware(validateKey func(ctx context.Context, key string) (string, error)) *APIKeyMiddleware {
	return &APIKeyMiddleware{validateKey: validateKey}
}

// Middleware returns an HTTP middleware that validates API keys.
func (m *APIKeyMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for API key in header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// Fall through to next middleware (might use JWT auth)
			next.ServeHTTP(w, r)
			return
		}

		// Validate API key
		accountID, err := m.validateKey(r.Context(), apiKey)
		if err != nil {
			http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
			return
		}

		// Add account ID to context
		ctx := context.WithValue(r.Context(), contextKey("account_id"), accountID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAccountID retrieves the account ID from context (from API key auth).
func GetAccountID(ctx context.Context) string {
	id, ok := ctx.Value(contextKey("account_id")).(string)
	if !ok {
		return ""
	}
	return id
}

// =============================================================================
// Claim helpers
// =============================================================================

func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func getMapClaim(claims jwt.MapClaims, key string) map[string]interface{} {
	if val, ok := claims[key]; ok {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

func getTimeClaim(claims jwt.MapClaims, key string) time.Time {
	val, ok := claims[key]
	if !ok {
		return time.Time{}
	}

	switch v := val.(type) {
	case float64:
		return time.Unix(int64(v), 0)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return time.Unix(n, 0)
		}
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

// =============================================================================
// Combined Auth Middleware
// =============================================================================

// CombinedAuthMiddleware combines Supabase JWT and API key authentication.
func CombinedAuthMiddleware(supabaseAuth *SupabaseAuthMiddleware, apiKeyAuth *APIKeyMiddleware) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Chain: API Key -> Supabase JWT -> Handler
		return apiKeyAuth.Middleware(supabaseAuth.Middleware(next))
	}
}
