package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

var publicPaths = map[string]struct{}{
	"/healthz": {},
}

func wrapWithAuth(next http.Handler, tokens []string, log *logger.Logger) http.Handler {
	tokenSet := normaliseTokens(tokens)
	if len(tokenSet) == 0 {
		if log != nil {
			log.Warn("API auth tokens not configured; serving endpoints without authentication")
		}
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := publicPaths[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthorised(w)
			return
		}
		token := strings.TrimSpace(parts[1])
		if _, ok := tokenSet[token]; !ok {
			unauthorised(w)
			return
		}
		next.ServeHTTP(w, r)
	})
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
