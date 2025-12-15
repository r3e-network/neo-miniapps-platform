package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/headergate"
	"github.com/R3E-Network/service_layer/internal/httputil"
	sllogging "github.com/R3E-Network/service_layer/internal/logging"
)

func HeaderGateMiddleware(sharedSecret string) func(http.Handler) http.Handler {
	return headergate.Middleware(sharedSecret)
}

func authMiddleware(db *database.Repository) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try API Key first
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				keyHash := hashToken(apiKey)
				key, err := db.GetAPIKeyByHash(r.Context(), keyHash)
				if err != nil {
					// NotFound means the API key is invalid/revoked. Other errors mean the
					// auth backend is unhealthy; fail closed with 500.
					if !database.IsNotFound(err) {
						log.Printf("get api key by hash: %v", err)
						jsonError(w, "failed to validate api key", http.StatusInternalServerError)
						return
					}
				} else {
					role := resolveUserRole(key.UserID)
					ctx := sllogging.WithUserID(r.Context(), key.UserID)
					if role != "" {
						ctx = sllogging.WithRole(ctx, role)
					}
					r = r.WithContext(ctx)
					r.Header.Set("X-User-ID", key.UserID)
					if role != "" {
						r.Header.Set("X-User-Role", role)
					} else {
						r.Header.Del("X-User-Role")
					}
					if updateErr := db.UpdateAPIKeyLastUsed(r.Context(), key.ID); updateErr != nil {
						log.Printf("update api key last used: %v", updateErr)
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			// Try JWT token from Authorization header or auth_token cookie
			var token string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && len(authHeader) >= 7 && strings.HasPrefix(authHeader, "Bearer ") {
				token = authHeader[7:]
			} else {
				// Try auth_token cookie (for OAuth cookie-based flow)
				if cookie, err := r.Cookie(oauthTokenCookieName); err == nil && cookie.Value != "" {
					token = cookie.Value
				}
			}

			if token == "" {
				jsonError(w, "missing authorization", http.StatusUnauthorized)
				return
			}

			userID, err := validateJWT(token)
			if err != nil {
				jsonError(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// Verify session exists
			tokenHash := hashToken(token)
			session, err := db.GetSessionByTokenHash(r.Context(), tokenHash)
			if err != nil {
				if database.IsNotFound(err) {
					jsonError(w, "session expired", http.StatusUnauthorized)
					return
				}
				log.Printf("get session by token hash: %v", err)
				jsonError(w, "failed to validate session", http.StatusInternalServerError)
				return
			}

			// Update session activity
			if updateErr := db.UpdateSessionActivity(r.Context(), session.ID); updateErr != nil {
				log.Printf("update session activity: %v", updateErr)
			}

			role := resolveUserRole(userID)
			ctx := sllogging.WithUserID(r.Context(), userID)
			if role != "" {
				ctx = sllogging.WithRole(ctx, role)
			}
			r = r.WithContext(ctx)
			r.Header.Set("X-User-ID", userID)
			if role != "" {
				r.Header.Set("X-User-Role", role)
			} else {
				r.Header.Del("X-User-Role")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// =============================================================================
// JWT Helpers
// =============================================================================

func generateJWT(userID string) (string, error) {
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "neo-service-layer",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func validateJWT(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims.UserID, nil
	}

	return "", fmt.Errorf("invalid token")
}

// =============================================================================
// Utility Helpers
// =============================================================================

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func jsonError(w http.ResponseWriter, message string, status int) {
	httputil.WriteError(w, status, message)
}
