package main

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// Wallet Handlers
// =============================================================================

func listWalletsHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		wallets, err := db.GetUserWallets(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get wallets", http.StatusInternalServerError)
			return
		}

		httputil.WriteJSON(w, http.StatusOK, wallets)
	}
}

func addWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		var req struct {
			Address   string `json:"address"`
			Label     string `json:"label"`
			PublicKey string `json:"publicKey"`
			Signature string `json:"signature"`
			Message   string `json:"message"`
		}
		if !httputil.DecodeJSON(w, r, &req) {
			return
		}

		// Require Neo N3 signature to prove wallet ownership
		if req.PublicKey == "" || req.Signature == "" || req.Message == "" {
			jsonError(w, "publicKey, signature, and message are required to add a wallet", http.StatusBadRequest)
			return
		}
		if !verifyNeoSignature(req.Address, req.Message, req.Signature, req.PublicKey) {
			jsonError(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		wallet := &database.UserWallet{
			UserID:                userID,
			Address:               req.Address,
			Label:                 req.Label,
			IsPrimary:             false,
			Verified:              true,
			VerificationMessage:   req.Message,
			VerificationSignature: req.Signature,
			CreatedAt:             time.Now(),
		}

		if err := db.CreateWallet(r.Context(), wallet); err != nil {
			jsonError(w, "failed to add wallet", http.StatusInternalServerError)
			return
		}

		httputil.WriteJSON(w, http.StatusCreated, wallet)
	}
}

func setPrimaryWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		walletID := mux.Vars(r)["id"]

		if err := db.SetPrimaryWallet(r.Context(), userID, walletID); err != nil {
			jsonError(w, "failed to set primary wallet", http.StatusInternalServerError)
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func verifyWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		walletID := mux.Vars(r)["id"]

		var req struct {
			PublicKey string `json:"publicKey"`
			Signature string `json:"signature"`
			Message   string `json:"message"`
		}
		if !httputil.DecodeJSON(w, r, &req) {
			return
		}

		if req.PublicKey == "" || req.Signature == "" || req.Message == "" {
			jsonError(w, "publicKey, signature, and message are required to verify wallet", http.StatusBadRequest)
			return
		}

		// Get wallet to verify ownership
		wallet, err := db.GetWallet(r.Context(), walletID, userID)
		if err != nil {
			jsonError(w, "wallet not found", http.StatusNotFound)
			return
		}

		// Verify Neo N3 signature to prove wallet ownership
		if !verifyNeoSignature(wallet.Address, req.Message, req.Signature, req.PublicKey) {
			jsonError(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		if err := db.VerifyWallet(r.Context(), walletID, req.Signature); err != nil {
			jsonError(w, "failed to verify wallet", http.StatusInternalServerError)
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "verified"})
	}
}

func deleteWalletHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		walletID := mux.Vars(r)["id"]

		if err := db.DeleteWallet(r.Context(), walletID, userID); err != nil {
			jsonError(w, "failed to delete wallet", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
