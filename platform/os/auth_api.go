// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// authAPIImpl implements AuthAPI.
// Provides authentication and authorization via Supabase Auth or JWT validation.
type authAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
}

func newAuthAPI(ctx *ServiceContext, serviceID string) *authAPIImpl {
	return &authAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
	}
}

func (a *authAPIImpl) VerifyToken(ctx context.Context, token string) (*TokenClaims, error) {
	if err := a.ctx.RequireCapability(CapAuth); err != nil {
		return nil, err
	}

	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	// Prefer signature verification when a Supabase JWT secret is available.
	if secret := a.getJWTSecret(ctx); secret != "" {
		claims, err := a.verifyWithJWTSecret(token, secret)
		if err != nil {
			return nil, fmt.Errorf("invalid token: %w", err)
		}
		// Check expiration
		if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
			return nil, fmt.Errorf("token expired")
		}
		return claims, nil
	}

	// Fallback: decode without signature verification.
	claims, err := a.decodeJWT(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}

	// Optionally verify with Supabase Auth API
	if a.shouldVerifyWithSupabase() {
		verified, err := a.verifyWithSupabase(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("token verification failed: %w", err)
		}
		if !verified {
			return nil, fmt.Errorf("token verification failed")
		}
	}

	return claims, nil
}

// verifyWithJWTSecret verifies the JWT signature using the provided secret.
func (a *authAPIImpl) verifyWithJWTSecret(token, secret string) (*TokenClaims, error) {
	mapClaims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, mapClaims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("jwt invalid")
	}

	claims := &TokenClaims{
		Subject:   getString(mapClaims, "sub"),
		Audience:  getString(mapClaims, "aud"),
		Issuer:    getString(mapClaims, "iss"),
		ExpiresAt: getInt64(mapClaims, "exp"),
		IssuedAt:  getInt64(mapClaims, "iat"),
	}

	// Roles can be in "role" or "roles"
	if role := getString(mapClaims, "role"); role != "" {
		claims.Roles = append(claims.Roles, role)
	}
	if roles, ok := mapClaims["roles"]; ok {
		switch v := roles.(type) {
		case []any:
			for _, r := range v {
				if s, ok := r.(string); ok {
					claims.Roles = append(claims.Roles, s)
				}
			}
		case []string:
			claims.Roles = append(claims.Roles, v...)
		}
	}

	extra := make(map[string]any)
	if email := getString(mapClaims, "email"); email != "" {
		extra["email"] = email
	}
	if len(extra) > 0 {
		claims.Extra = extra
	}

	return claims, nil
}

func (a *authAPIImpl) GetUser(ctx context.Context, token string) (*AuthUser, error) {
	if err := a.ctx.RequireCapability(CapAuth); err != nil {
		return nil, err
	}

	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	// First verify the token
	claims, err := a.VerifyToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Try to get user from Supabase
	user, err := a.getUserFromSupabase(ctx, token)
	if err != nil {
		// Fall back to claims-based user
		// Extract email from Extra if available
		email := ""
		if claims.Extra != nil {
			if e, ok := claims.Extra["email"].(string); ok {
				email = e
			}
		}
		return &AuthUser{
			ID:    claims.Subject,
			Email: email,
			Roles: claims.Roles,
		}, nil
	}

	return user, nil
}

func (a *authAPIImpl) HasPermission(ctx context.Context, userID string, permission string) (bool, error) {
	if err := a.ctx.RequireCapability(CapAuth); err != nil {
		return false, err
	}

	if userID == "" || permission == "" {
		return false, nil
	}

	// Get user permissions from database or cache
	permissions, err := a.GetPermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, p := range permissions {
		if p == permission || p == "*" {
			return true, nil
		}
		// Check wildcard patterns (e.g., "service:*" matches "service:read")
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(permission, prefix) {
				return true, nil
			}
		}
	}

	return false, nil
}

func (a *authAPIImpl) HasRole(ctx context.Context, userID string, role string) (bool, error) {
	if err := a.ctx.RequireCapability(CapAuth); err != nil {
		return false, err
	}

	if userID == "" || role == "" {
		return false, nil
	}

	roles, err := a.GetRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}

	return false, nil
}

func (a *authAPIImpl) GetPermissions(ctx context.Context, userID string) ([]string, error) {
	if err := a.ctx.RequireCapability(CapAuth); err != nil {
		return nil, err
	}

	if userID == "" {
		return nil, nil
	}

	// Try to get from cache first
	cacheKey := fmt.Sprintf("auth:permissions:%s", userID)
	if cached, err := a.ctx.Cache().Get(ctx, cacheKey); err == nil && cached != nil {
		var permissions []string
		if err := json.Unmarshal(cached, &permissions); err == nil {
			return permissions, nil
		}
	}

	// Query from database
	permissions, err := a.queryPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(permissions); err == nil {
		a.ctx.Cache().Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return permissions, nil
}

func (a *authAPIImpl) GetRoles(ctx context.Context, userID string) ([]string, error) {
	if err := a.ctx.RequireCapability(CapAuth); err != nil {
		return nil, err
	}

	if userID == "" {
		return nil, nil
	}

	// Try to get from cache first
	cacheKey := fmt.Sprintf("auth:roles:%s", userID)
	if cached, err := a.ctx.Cache().Get(ctx, cacheKey); err == nil && cached != nil {
		var roles []string
		if err := json.Unmarshal(cached, &roles); err == nil {
			return roles, nil
		}
	}

	// Query from database
	roles, err := a.queryRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(roles); err == nil {
		a.ctx.Cache().Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return roles, nil
}

// Helper methods

func (a *authAPIImpl) decodeJWT(token string) (*TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try standard base64
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode payload: %w", err)
		}
	}

	var claims struct {
		Sub   string   `json:"sub"`
		Email string   `json:"email"`
		Role  string   `json:"role"`
		Roles []string `json:"roles"`
		Aud   string   `json:"aud"`
		Exp   int64    `json:"exp"`
		Iat   int64    `json:"iat"`
		Iss   string   `json:"iss"`
	}

	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Build extra claims map
	extra := make(map[string]any)
	if claims.Email != "" {
		extra["email"] = claims.Email
	}
	if claims.Role != "" {
		extra["role"] = claims.Role
	}

	return &TokenClaims{
		Subject:   claims.Sub,
		Roles:     claims.Roles,
		Audience:  claims.Aud,
		ExpiresAt: claims.Exp,
		IssuedAt:  claims.Iat,
		Issuer:    claims.Iss,
		Extra:     extra,
	}, nil
}

func (a *authAPIImpl) getJWTSecret(ctx context.Context) string {
	// Preferred: pull from config if available
	if a.ctx != nil && a.ctx.Config() != nil {
		if secret, err := a.ctx.Config().GetString(ctx, "auth.supabase.jwt_secret"); err == nil && strings.TrimSpace(secret) != "" {
			return strings.TrimSpace(secret)
		}
	}
	// Fallback: environment variable
	if env := strings.TrimSpace(os.Getenv("SUPABASE_JWT_SECRET")); env != "" {
		return env
	}
	return ""
}

func getString(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func getInt64(claims jwt.MapClaims, key string) int64 {
	val, ok := claims[key]
	if !ok {
		return 0
	}

	switch v := val.(type) {
	case float64:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func (a *authAPIImpl) shouldVerifyWithSupabase() bool {
	// Check if Supabase verification is enabled
	ctx := context.Background()
	enabled, err := a.ctx.Config().GetBool(ctx, "auth.supabase.verify_enabled")
	if err != nil {
		return false
	}
	return enabled
}

func (a *authAPIImpl) verifyWithSupabase(ctx context.Context, token string) (bool, error) {
	// Get Supabase URL from config
	supabaseURL, err := a.ctx.Config().GetString(ctx, "auth.supabase.url")
	if err != nil || supabaseURL == "" {
		return false, fmt.Errorf("supabase URL not configured")
	}

	// Call Supabase Auth API to verify token
	req := HTTPRequest{
		Method: "GET",
		URL:    supabaseURL + "/auth/v1/user",
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
			"Accept":        "application/json",
		},
	}

	resp, err := a.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.StatusCode == 200, nil
}

func (a *authAPIImpl) getUserFromSupabase(ctx context.Context, token string) (*AuthUser, error) {
	// Get Supabase URL from config
	supabaseURL, err := a.ctx.Config().GetString(ctx, "auth.supabase.url")
	if err != nil || supabaseURL == "" {
		return nil, fmt.Errorf("supabase URL not configured")
	}

	// Call Supabase Auth API
	req := HTTPRequest{
		Method: "GET",
		URL:    supabaseURL + "/auth/v1/user",
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
			"Accept":        "application/json",
		},
	}

	resp, err := a.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get user: status %d", resp.StatusCode)
	}

	var userData struct {
		ID               string           `json:"id"`
		Email            string           `json:"email"`
		Phone            string           `json:"phone"`
		Role             string           `json:"role"`
		AppMetadata      map[string]any   `json:"app_metadata"`
		UserMetadata     map[string]any   `json:"user_metadata"`
		CreatedAt        string           `json:"created_at"`
		UpdatedAt        string           `json:"updated_at"`
		EmailConfirmedAt string           `json:"email_confirmed_at"`
		Identities       []map[string]any `json:"identities"`
	}

	if err := json.Unmarshal(resp.Body, &userData); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	// Build metadata with additional fields
	metadata := userData.UserMetadata
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if userData.Phone != "" {
		metadata["phone"] = userData.Phone
	}
	if userData.Role != "" {
		metadata["role"] = userData.Role
	}

	user := &AuthUser{
		ID:       userData.ID,
		Email:    userData.Email,
		Metadata: metadata,
	}

	// Extract roles from app_metadata if present
	if roles, ok := userData.AppMetadata["roles"].([]any); ok {
		for _, r := range roles {
			if role, ok := r.(string); ok {
				user.Roles = append(user.Roles, role)
			}
		}
	}

	return user, nil
}

func (a *authAPIImpl) queryPermissions(ctx context.Context, userID string) ([]string, error) {
	// Query permissions from database
	// This uses the Database API to query a permissions table
	db := a.ctx.Database()

	data, err := db.From("user_permissions").
		Select("permission").
		Eq("user_id", userID).
		Execute(ctx)

	if err != nil {
		// Return empty permissions if query fails
		return []string{}, nil
	}

	var results []struct {
		Permission string `json:"permission"`
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return []string{}, nil
	}

	permissions := make([]string, len(results))
	for i, r := range results {
		permissions[i] = r.Permission
	}

	return permissions, nil
}

func (a *authAPIImpl) queryRoles(ctx context.Context, userID string) ([]string, error) {
	// Query roles from database
	db := a.ctx.Database()

	data, err := db.From("user_roles").
		Select("role").
		Eq("user_id", userID).
		Execute(ctx)

	if err != nil {
		// Return empty roles if query fails
		return []string{}, nil
	}

	var results []struct {
		Role string `json:"role"`
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return []string{}, nil
	}

	roles := make([]string, len(results))
	for i, r := range results {
		roles[i] = r.Role
	}

	return roles, nil
}
