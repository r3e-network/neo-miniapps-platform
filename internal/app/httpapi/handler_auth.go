package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/auth"
)

type authProvider interface {
	HasUsers() bool
	Authenticate(username, password string) (any, error)
	Issue(any, time.Duration) (string, time.Time, error)
	IssueWalletChallenge(wallet string, ttl time.Duration) (string, time.Time, error)
	VerifyWalletSignature(wallet, signature, pubKey string) (auth.User, error)
}

var errUnauthorised = auth.ErrUnauthorised

// loginHandler issues JWT tokens for configured users. When no users are configured,
// it rejects requests to avoid giving a false sense of auth.
func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.authManager == nil || !h.authManager.HasUsers() {
		writeError(w, http.StatusUnauthorized, errUnauthorised)
		return
	}
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := h.authManager.Authenticate(payload.Username, payload.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	token, exp, err := h.authManager.Issue(user, 24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := map[string]any{
		"token":      token,
		"expires_at": exp.UTC().Format(time.RFC3339),
		"role":       user.Role,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// walletChallenge returns a nonce for the wallet to sign (HMAC-based in this implementation).
func (h *handler) walletChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.authManager == nil || !h.authManager.HasUsers() {
		writeError(w, http.StatusUnauthorized, errUnauthorised)
		return
	}
	var payload struct {
		Wallet string `json:"wallet"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	nonce, exp, err := h.authManager.IssueWalletChallenge(payload.Wallet, 5*time.Minute)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"nonce":      nonce,
		"expires_at": exp.UTC().Format(time.RFC3339),
	})
}

// walletLogin verifies a signed nonce and returns a JWT.
func (h *handler) walletLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.authManager == nil || !h.authManager.HasUsers() {
		writeError(w, http.StatusUnauthorized, errUnauthorised)
		return
	}
	var payload struct {
		Wallet    string `json:"wallet"`
		Signature string `json:"signature"`
		PubKey    string `json:"pubkey"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := h.authManager.VerifyWalletSignature(payload.Wallet, payload.Signature, payload.PubKey)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	token, exp, err := h.authManager.Issue(user, 24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"expires_at": exp.UTC().Format(time.RFC3339),
		"role":       user.Role,
	})
}

// whoami returns the current principal and role derived from token/JWT.
func (h *handler) whoami(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	user, _ := r.Context().Value(ctxUserKey).(string)
	role, _ := r.Context().Value(ctxRoleKey).(string)
	tenant, _ := r.Context().Value(ctxTenantKey).(string)
	if user == "" {
		unauthorised(w)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": user,
		"role": role,
		"tenant": tenant,
	})
}
