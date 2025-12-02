package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/applications/auth"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

var refreshHTTPClient = &http.Client{Timeout: 5 * time.Second}

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
	if h.authManager == nil || !h.authManager.HasUsers() {
		core.WriteError(w, http.StatusUnauthorized, errUnauthorised)
		return
	}
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	user, err := h.authManager.Authenticate(payload.Username, payload.Password)
	if err != nil {
		core.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	token, exp, err := h.authManager.Issue(user, 24*time.Hour)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	resp := map[string]any{
		"token":      token,
		"expires_at": exp.UTC().Format(time.RFC3339),
		"role":       user.Role,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// refresh proxies a Supabase GoTrue refresh token exchange when configured.
func (h *handler) refresh(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil || strings.TrimSpace(payload.RefreshToken) == "" {
		core.WriteError(w, http.StatusBadRequest, fmt.Errorf("refresh_token required"))
		return
	}
	goTrueURL := strings.TrimSpace(h.supabaseGoTrueURL)
	if goTrueURL == "" {
		goTrueURL = strings.TrimSpace(os.Getenv("SUPABASE_GOTRUE_URL"))
	}
	if goTrueURL == "" {
		core.WriteError(w, http.StatusNotImplemented, fmt.Errorf("supabase gotrue url not configured"))
		return
	}
	reqBody := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": payload.RefreshToken,
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(goTrueURL, "/")+"/token?grant_type=refresh_token", strings.NewReader(string(body)))
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := refreshHTTPClient.Do(req)
	if err != nil {
		core.WriteError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// walletChallenge returns a nonce for the wallet to sign (HMAC-based in this implementation).
func (h *handler) walletChallenge(w http.ResponseWriter, r *http.Request) {
	if h.authManager == nil || !h.authManager.HasUsers() {
		core.WriteError(w, http.StatusUnauthorized, errUnauthorised)
		return
	}
	var payload struct {
		Wallet string `json:"wallet"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	nonce, exp, err := h.authManager.IssueWalletChallenge(payload.Wallet, 5*time.Minute)
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"nonce":      nonce,
		"expires_at": exp.UTC().Format(time.RFC3339),
	})
}

// walletLogin verifies a signed nonce and returns a JWT.
func (h *handler) walletLogin(w http.ResponseWriter, r *http.Request) {
	if h.authManager == nil || !h.authManager.HasUsers() {
		core.WriteError(w, http.StatusUnauthorized, errUnauthorised)
		return
	}
	var payload struct {
		Wallet    string `json:"wallet"`
		Signature string `json:"signature"`
		PubKey    string `json:"pubkey"`
	}
	if err := decodeJSON(r.Body, &payload); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	user, err := h.authManager.VerifyWalletSignature(payload.Wallet, payload.Signature, payload.PubKey)
	if err != nil {
		core.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	token, exp, err := h.authManager.Issue(user, 24*time.Hour)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
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
	user, _ := r.Context().Value(ctxUserKey).(string)
	role, _ := r.Context().Value(ctxRoleKey).(string)
	tenant, _ := r.Context().Value(ctxTenantKey).(string)
	if user == "" {
		unauthorised(w)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user":   user,
		"role":   role,
		"tenant": tenant,
	})
}
