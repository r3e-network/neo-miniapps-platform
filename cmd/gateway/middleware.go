package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

// =============================================================================
// Middleware
// =============================================================================

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get origin from request for credentials support
		origin := r.Header.Get("Origin")
		if origin != "" && isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			w.Header().Set("Access-Control-Allow-Credentials", "true") // Required for cookie auth
		} else if origin != "" {
			// Reject unknown origins when credentials are used
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authMiddleware(db *database.Repository, m *marble.Marble) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try API Key first
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				keyHash := hashToken(apiKey)
				key, err := db.GetAPIKeyByHash(r.Context(), keyHash)
				if err == nil {
					r.Header.Set("X-User-ID", key.UserID)
					_ = db.UpdateAPIKeyLastUsed(r.Context(), key.ID)
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
				jsonError(w, "session expired", http.StatusUnauthorized)
				return
			}

			// Update session activity
			_ = db.UpdateSessionActivity(r.Context(), session.ID)

			r.Header.Set("X-User-ID", userID)
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
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func isOriginAllowed(origin string) bool {
	allowed := os.Getenv("CORS_ALLOWED_ORIGINS")
	if strings.TrimSpace(allowed) == "" {
		allowed = "http://localhost:3000,http://localhost:5173"
	}
	for _, candidate := range strings.Split(allowed, ",") {
		c := strings.TrimSpace(candidate)
		if c != "" && c == origin {
			return true
		}
	}
	return false
}
