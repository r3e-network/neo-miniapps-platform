package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/gorilla/mux"
)

// =============================================================================
// Wallet Handlers
// =============================================================================

func listWalletsHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		wallets, err := db.GetUserWallets(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get wallets", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wallets)
	}
}

func addWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")

		var req struct {
			Address   string `json:"address"`
			Label     string `json:"label"`
			PublicKey string `json:"publicKey"`
			Signature string `json:"signature"`
			Message   string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Verify Neo N3 signature to prove wallet ownership
		if req.PublicKey != "" && req.Signature != "" && req.Message != "" {
			if !verifyNeoSignature(req.Address, req.Message, req.Signature, req.PublicKey) {
				jsonError(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		wallet := &database.UserWallet{
			UserID:              userID,
			Address:             req.Address,
			Label:               req.Label,
			IsPrimary:           false,
			Verified:            true,
			VerificationMessage: req.Message,
			CreatedAt:           time.Now(),
		}

		if err := db.CreateWallet(r.Context(), wallet); err != nil {
			jsonError(w, "failed to add wallet", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(wallet)
	}
}

func setPrimaryWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		walletID := mux.Vars(r)["id"]

		if err := db.SetPrimaryWallet(r.Context(), userID, walletID); err != nil {
			jsonError(w, "failed to set primary wallet", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func verifyWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		walletID := mux.Vars(r)["id"]

		var req struct {
			PublicKey string `json:"publicKey"`
			Signature string `json:"signature"`
			Message   string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Get wallet to verify ownership
		wallet, err := db.GetWallet(r.Context(), walletID, userID)
		if err != nil {
			jsonError(w, "wallet not found", http.StatusNotFound)
			return
		}

		// Verify Neo N3 signature to prove wallet ownership
		if req.PublicKey != "" && req.Signature != "" && req.Message != "" {
			if !verifyNeoSignature(wallet.Address, req.Message, req.Signature, req.PublicKey) {
				jsonError(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		if err := db.VerifyWallet(r.Context(), walletID, req.Signature); err != nil {
			jsonError(w, "failed to verify wallet", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "verified"})
	}
}

func deleteWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		walletID := mux.Vars(r)["id"]

		if err := db.DeleteWallet(r.Context(), walletID, userID); err != nil {
			jsonError(w, "failed to delete wallet", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
