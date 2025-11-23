package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/R3E-Network/service_layer/internal/app/auth"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

var publicPaths = map[string]struct{}{
	"/healthz":               {},
	"/system/version":        {},
	"/auth/login":            {},
	"/auth/wallet/challenge": {},
	"/auth/wallet/login":     {},
}

type ctxKey string

const (
	ctxUserKey   ctxKey = "httpapi.user"
	ctxRoleKey   ctxKey = "httpapi.role"
	ctxTenantKey ctxKey = "httpapi.tenant"
)

var adminPrefixes = []string{
	"/admin",
}

func wrapWithAuth(next http.Handler, tokens []string, log *logger.Logger, jwtMgr JWTValidator) http.Handler {
	tokenSet := normaliseTokens(tokens)
	if len(tokenSet) == 0 && jwtMgr == nil && log != nil {
		log.Warn("API auth tokens or JWT auth not configured; rejecting authenticated endpoints")
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
				ctx = withTenant(ctx, r)
				if enforceRole(w, r, ctx) {
					next.ServeHTTP(w, r.WithContext(ctx))
				}
				return
			}
			if jwtMgr != nil {
				if claims, err := jwtMgr.Validate(token); err == nil {
					ctx := context.WithValue(r.Context(), ctxUserKey, claims.Username)
					ctx = context.WithValue(ctx, ctxRoleKey, claims.Role)
					ctx = withTenant(ctx, r)
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
		writeError(w, http.StatusForbidden, fmt.Errorf("forbidden: admin only"))
		return false
	}
	if isAdminPath(path) && strings.TrimSpace(tenant) == "" {
		writeError(w, http.StatusForbidden, fmt.Errorf("forbidden: tenant required"))
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

// extractToken supports the standard Authorization header and a token query
// parameter for convenience (e.g., dashboards launched with a prefilled link).
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
	writeError(w, http.StatusUnauthorized, fmt.Errorf("unauthorised"))
}
