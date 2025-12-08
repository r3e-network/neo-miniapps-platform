package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/google/uuid"
)

// =============================================================================
// Health & Info Handlers
// =============================================================================

func healthHandler(m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"service":   "gateway",
			"version":   "1.0.0",
			"enclave":   m.IsEnclave(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

func attestationHandler(m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := m.Report()
		if report == nil {
			http.Error(w, "not running in enclave", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enclave":          true,
			"security_version": report.SecurityVersion,
			"debug":            report.Debug,
		})
	}
}

// =============================================================================
// Auth Handlers
// =============================================================================

func nonceHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Generate random nonce
		nonceBytes := make([]byte, 32)
		if _, err := rand.Read(nonceBytes); err != nil {
			jsonError(w, "failed to generate nonce", http.StatusInternalServerError)
			return
		}
		nonce := hex.EncodeToString(nonceBytes)

		// Get or create user
		user, err := db.GetUserByAddress(r.Context(), req.Address)
		if err != nil {
			// Create new user
			user = &database.User{
				ID:        uuid.New().String(),
				Address:   req.Address,
				CreatedAt: time.Now(),
			}
			if err := db.CreateUser(r.Context(), user); err != nil {
				jsonError(w, "failed to create user", http.StatusInternalServerError)
				return
			}
		}

		// Store nonce for verification
		if err := db.UpdateUserNonce(r.Context(), user.ID, nonce); err != nil {
			jsonError(w, "failed to store nonce", http.StatusInternalServerError)
			return
		}

		message := fmt.Sprintf("Sign this message to authenticate with Neo Service Layer.\n\nNonce: %s\nTimestamp: %d", nonce, time.Now().Unix())

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nonce":   nonce,
			"message": message,
		})
	}
}

func registerHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Address   string `json:"address"`
			PublicKey string `json:"publicKey"`
			Signature string `json:"signature"`
			Message   string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Verify Neo N3 signature (public key required for verification)
		if req.PublicKey != "" && req.Signature != "" && req.Message != "" {
			if !verifyNeoSignature(req.Address, req.Message, req.Signature, req.PublicKey) {
				jsonError(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Get or create user
		user, err := db.GetUserByAddress(r.Context(), req.Address)
		if err != nil {
			user = &database.User{
				ID:        uuid.New().String(),
				Address:   req.Address,
				CreatedAt: time.Now(),
			}
			if err := db.CreateUser(r.Context(), user); err != nil {
				jsonError(w, "failed to create user", http.StatusInternalServerError)
				return
			}
		}

		// Create primary wallet
		wallet := &database.UserWallet{
			UserID:    user.ID,
			Address:   req.Address,
			IsPrimary: true,
			Verified:  true,
			CreatedAt: time.Now(),
		}
		_ = db.CreateWallet(r.Context(), wallet)

		// Create gas bank account
		_, _ = db.GetOrCreateGasBankAccount(r.Context(), user.ID)

		// Generate JWT token
		token, err := generateJWT(user.ID)
		if err != nil {
			jsonError(w, "failed to generate token", http.StatusInternalServerError)
			return
		}

		// Create session
		tokenHash := hashToken(token)
		session := &database.UserSession{
			UserID:    user.ID,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
		}
		_ = db.CreateSession(r.Context(), session)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user_id": user.ID,
			"address": user.Address,
			"token":   token,
		})
	}
}

func loginHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Address   string `json:"address"`
			PublicKey string `json:"publicKey"`
			Signature string `json:"signature"`
			Message   string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Verify Neo N3 signature (public key required for verification)
		if req.PublicKey != "" && req.Signature != "" && req.Message != "" {
			if !verifyNeoSignature(req.Address, req.Message, req.Signature, req.PublicKey) {
				jsonError(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		user, err := db.GetUserByAddress(r.Context(), req.Address)
		if err != nil {
			jsonError(w, "user not found", http.StatusNotFound)
			return
		}

		// Generate JWT token
		token, err := generateJWT(user.ID)
		if err != nil {
			jsonError(w, "failed to generate token", http.StatusInternalServerError)
			return
		}

		// Create session
		tokenHash := hashToken(token)
		session := &database.UserSession{
			UserID:    user.ID,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
		}
		_ = db.CreateSession(r.Context(), session)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user_id": user.ID,
			"address": user.Address,
			"token":   token,
		})
	}
}

func logoutHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && len(authHeader) > 7 {
			token := authHeader[7:]
			tokenHash := hashToken(token)
			_ = db.DeleteSession(r.Context(), tokenHash)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
	}
}

func meHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		user, err := db.GetUser(r.Context(), userID)
		if err != nil {
			jsonError(w, "user not found", http.StatusNotFound)
			return
		}

		wallets, err := db.GetUserWallets(r.Context(), userID)
		if err != nil {
			log.Printf("Failed to get wallets for user %s: %v", userID, err)
		}
		account, err := db.GetOrCreateGasBankAccount(r.Context(), userID)
		if err != nil {
			log.Printf("Failed to get gas bank account for user %s: %v", userID, err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user":    user,
			"wallets": wallets,
			"gasbank": account,
		})
	}
}
