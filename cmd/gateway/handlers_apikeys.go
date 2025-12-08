package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/gorilla/mux"
)

// =============================================================================
// API Key Handlers
// =============================================================================

func listAPIKeysHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		keys, err := db.GetAPIKeys(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get API keys", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keys)
	}
}

func createAPIKeyHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")

		var req struct {
			Name   string   `json:"name"`
			Scopes []string `json:"scopes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			jsonError(w, "name is required", http.StatusBadRequest)
			return
		}

		// Generate API key
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			jsonError(w, "failed to generate key", http.StatusInternalServerError)
			return
		}
		rawKey := "sl_" + hex.EncodeToString(keyBytes)
		prefix := rawKey[:11]
		keyHash := hashToken(rawKey)

		apiKey := &database.APIKey{
			UserID:  userID,
			Name:    req.Name,
			KeyHash: keyHash,
			Prefix:  prefix,
			Scopes:  req.Scopes,
		}

		if err := db.CreateAPIKey(r.Context(), apiKey); err != nil {
			jsonError(w, "failed to create API key", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         apiKey.ID,
			"name":       apiKey.Name,
			"key":        rawKey, // Only returned once!
			"prefix":     prefix,
			"scopes":     apiKey.Scopes,
			"created_at": apiKey.CreatedAt,
		})
	}
}

func revokeAPIKeyHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		keyID := mux.Vars(r)["id"]

		if err := db.RevokeAPIKey(r.Context(), keyID, userID); err != nil {
			jsonError(w, "failed to revoke API key", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
