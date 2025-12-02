package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/R3E-Network/service_layer/applications/auth"
	"github.com/R3E-Network/service_layer/pkg/logger"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	"github.com/golang-jwt/jwt/v5"
)

var publicPaths = map[string]struct{}{
	"/healthz":               {},
	"/system/version":        {},
	"/auth/login":            {},
	"/auth/wallet/challenge": {},
	"/auth/wallet/login":     {},
	"/auth/refresh":          {},
}

type ctxKey string

const (
	ctxUserKey   ctxKey = "httpapi.user"
	ctxRoleKey   ctxKey = "httpapi.role"
	ctxTenantKey ctxKey = "httpapi.tenant"
	ctxTokenKey  ctxKey = "httpapi.token"
)

func requireTenantHeaderEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("REQUIRE_TENANT_HEADER"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

var adminPrefixes = []string{
	"/admin",
}

func wrapWithAuth(next http.Handler, tokens []string, log *logger.Logger, validator JWTValidator) http.Handler {
	tokenSet := normaliseTokens(tokens)
	if len(tokenSet) == 0 && validator == nil && log != nil {
		log.Warn("API auth tokens or JWT auth not configured; rejecting authenticated endpoints")
	}
	if !requireTenantHeaderEnabled() && log != nil {
		log.Warn("REQUIRE_TENANT_HEADER not set; enable it in production to enforce tenant scoping")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if _, ok := publicPaths[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}
		token := extractToken(r)
		if token != "" {
			if _, ok := tokenSet[token]; ok {
				ctx := context.WithValue(r.Context(), ctxUserKey, "token")
				ctx = context.WithValue(ctx, ctxRoleKey, "token")
				ctx = context.WithValue(ctx, ctxTokenKey, token)
				ctx = withTenant(ctx, r)
				if requireTenantHeaderEnabled() && strings.TrimSpace(tenantFromCtx(ctx)) == "" {
					core.WriteError(w, http.StatusForbidden, fmt.Errorf("tenant header required"))
					return
				}
				if enforceRole(w, r, ctx) {
					next.ServeHTTP(w, r.WithContext(ctx))
				}
				return
			}
			if validator != nil {
				if claims, err := validator.Validate(token); err == nil {
					ctx := context.WithValue(r.Context(), ctxUserKey, claims.Username)
					ctx = context.WithValue(ctx, ctxRoleKey, claims.Role)
					ctx = context.WithValue(ctx, ctxTokenKey, claims.Username)
					if claims.Tenant != "" && strings.TrimSpace(r.Header.Get("X-Tenant-ID")) == "" {
						ctx = context.WithValue(ctx, ctxTenantKey, strings.TrimSpace(claims.Tenant))
					} else {
						ctx = withTenant(ctx, r)
					}
					if requireTenantHeaderEnabled() && strings.TrimSpace(tenantFromCtx(ctx)) == "" {
						core.WriteError(w, http.StatusForbidden, fmt.Errorf("tenant header required"))
						return
					}
					if enforceRole(w, r, ctx) {
						next.ServeHTTP(w, r.WithContext(ctx))
					}
					return
				}
			}
		}

		unauthorised(w)
		return
	})
}

// JWTValidator abstracts validation so we can plug in the auth manager without
// tying httpapi to a concrete implementation.
type JWTValidator interface {
	Validate(token string) (*auth.Claims, error)
}

// SupabaseJWTValidator validates Supabase-issued JWTs (HS256) using the Supabase JWT secret.
type SupabaseJWTValidator struct {
	secret      []byte
	aud         string
	adminRoles  map[string]struct{}
	tenantClaim string
	roleClaim   string
}

func NewSupabaseJWTValidator(secret, aud string, adminRoles []string, tenantClaim, roleClaim string) *SupabaseJWTValidator {
	secret = strings.TrimSpace(secret)
	aud = strings.TrimSpace(aud)
	if secret == "" {
		return nil
	}
	return &SupabaseJWTValidator{
		secret:      []byte(secret),
		aud:         aud,
		adminRoles:  normaliseRoles(adminRoles),
		tenantClaim: strings.TrimSpace(tenantClaim),
		roleClaim:   strings.TrimSpace(roleClaim),
	}
}

func (v *SupabaseJWTValidator) Validate(token string) (*auth.Claims, error) {
	if v == nil || len(v.secret) == 0 {
		return nil, fmt.Errorf("supabase jwt secret not configured")
	}
	claims := &auth.Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if v.aud != "" && claims.Audience != nil {
		validAud := false
		for _, a := range claims.Audience {
			if strings.EqualFold(strings.TrimSpace(a), v.aud) {
				validAud = true
				break
			}
		}
		if !validAud {
			return nil, fmt.Errorf("invalid audience")
		}
	}
	if v.roleClaim != "" {
		if role := v.extractClaim(token, v.roleClaim); role != "" {
			claims.Role = role
		}
	}
	if len(v.adminRoles) > 0 {
		role := strings.ToLower(strings.TrimSpace(claims.Role))
		if _, ok := v.adminRoles[role]; ok {
			claims.Role = "admin"
		}
	}
	if v.tenantClaim != "" {
		if tenant := v.extractClaim(token, v.tenantClaim); tenant != "" {
			claims.Tenant = tenant
		}
	}
	return claims, nil
}

func (v *SupabaseJWTValidator) extractTenant(token string) string {
	return v.extractClaim(token, v.tenantClaim)
}

func (v *SupabaseJWTValidator) extractClaim(token string, path string) string {
	raw := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, raw, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil {
		return ""
	}
	parts := strings.Split(path, ".")
	var current any = raw
	for _, p := range parts {
		switch m := current.(type) {
		case jwt.MapClaims:
			current = m[p]
		case map[string]any:
			current = m[p]
		default:
			return ""
		}
	}
	if s, ok := current.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func normaliseRoles(roles []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, r := range roles {
		role := strings.ToLower(strings.TrimSpace(r))
		if role == "" {
			continue
		}
		set[role] = struct{}{}
	}
	return set
}

// compositeValidator tries multiple validators until one succeeds.
type compositeValidator struct {
	validators []JWTValidator
}

// NewCompositeValidator returns a JWTValidator that tries each provided validator in order.
// Nil validators are skipped; nil is returned if no validators remain.
func NewCompositeValidator(validators ...JWTValidator) JWTValidator {
	filtered := make([]JWTValidator, 0, len(validators))
	for _, v := range validators {
		if v != nil {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return compositeValidator{validators: filtered}
}

func (c compositeValidator) Validate(token string) (*auth.Claims, error) {
	var lastErr error
	for _, v := range c.validators {
		if v == nil {
			continue
		}
		claims, err := v.Validate(token)
		if err == nil {
			return claims, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no validators configured")
}

func withTenant(ctx context.Context, r *http.Request) context.Context {
	tenant := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if tenant == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxTenantKey, tenant)
}

func enforceRole(w http.ResponseWriter, r *http.Request, ctx context.Context) bool {
	path := r.URL.Path
	role, _ := ctx.Value(ctxRoleKey).(string)
	tenant, _ := ctx.Value(ctxTenantKey).(string)
	if isAdminPath(path) && role != "admin" {
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden: admin only"))
		return false
	}
	if isAdminPath(path) && strings.TrimSpace(tenant) == "" {
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden: tenant required"))
		return false
	}
	if requireTenantHeaderEnabled() && strings.TrimSpace(tenant) == "" {
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("tenant header required"))
		return false
	}
	return true
}

func isAdminPath(path string) bool {
	for _, p := range adminPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// extractToken supports the standard Authorization header only; avoid query tokens.
func extractToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.Fields(authHeader)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func normaliseTokens(tokens []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, token := range tokens {
		t := strings.TrimSpace(token)
		if t == "" {
			continue
		}
		set[t] = struct{}{}
	}
	return set
}

func unauthorised(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	core.WriteError(w, http.StatusUnauthorized, fmt.Errorf("unauthorised"))
}
