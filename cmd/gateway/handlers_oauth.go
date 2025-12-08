package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// =============================================================================
// OAuth Handlers
// =============================================================================

func getOAuthConfig(m *marble.Marble, provider string) (clientID, clientSecret, redirectURL string) {
	prefix := strings.ToUpper(provider)
	if id, ok := m.Secret(prefix + "_CLIENT_ID"); ok {
		clientID = string(id)
	} else {
		clientID = os.Getenv(prefix + "_CLIENT_ID")
	}
	if secret, ok := m.Secret(prefix + "_CLIENT_SECRET"); ok {
		clientSecret = string(secret)
	} else {
		clientSecret = os.Getenv(prefix + "_CLIENT_SECRET")
	}
	redirectURL = os.Getenv("OAUTH_REDIRECT_BASE")
	if redirectURL == "" {
		redirectURL = "http://localhost:8080"
	}
	redirectURL += "/api/v1/auth/" + provider + "/callback"
	return
}

func googleAuthHandler(m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID, _, redirectURL := getOAuthConfig(m, "google")
		if clientID == "" {
			jsonError(w, "Google OAuth not configured", http.StatusServiceUnavailable)
			return
		}

		state := generateState()
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   m.TLSConfig() != nil,
			MaxAge:   600,
		})

		authURL := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
			url.QueryEscape(clientID),
			url.QueryEscape(redirectURL),
			url.QueryEscape("openid email profile"),
			url.QueryEscape(state),
		)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

func googleCallbackHandler(db *database.Repository, m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID, clientSecret, redirectURL := getOAuthConfig(m, "google")

		// Verify state
		stateCookie, err := r.Cookie("oauth_state")
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			jsonError(w, "invalid state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			jsonError(w, "missing code", http.StatusBadRequest)
			return
		}

		// Exchange code for tokens
		tokenResp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {redirectURL},
		})
		if err != nil {
			jsonError(w, "token exchange failed", http.StatusInternalServerError)
			return
		}
		defer tokenResp.Body.Close()

		var tokenData struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			IDToken      string `json:"id_token"`
		}
		if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
			jsonError(w, "failed to parse token", http.StatusInternalServerError)
			return
		}

		// Get user info
		userReq, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
		if err != nil {
			jsonError(w, "failed to create user info request", http.StatusInternalServerError)
			return
		}
		userReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
		userResp, err := http.DefaultClient.Do(userReq)
		if err != nil {
			jsonError(w, "failed to get user info", http.StatusInternalServerError)
			return
		}
		defer userResp.Body.Close()

		var googleUser struct {
			ID      string `json:"id"`
			Email   string `json:"email"`
			Name    string `json:"name"`
			Picture string `json:"picture"`
		}
		if err := json.NewDecoder(userResp.Body).Decode(&googleUser); err != nil {
			jsonError(w, "failed to parse user info", http.StatusInternalServerError)
			return
		}

		// Handle OAuth login/link
		handleOAuthCallback(w, r, db, "google", googleUser.ID, googleUser.Email, googleUser.Name, googleUser.Picture,
			tokenData.AccessToken, tokenData.RefreshToken, tokenData.ExpiresIn)
	}
}

func githubAuthHandler(m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID, _, redirectURL := getOAuthConfig(m, "github")
		if clientID == "" {
			jsonError(w, "GitHub OAuth not configured", http.StatusServiceUnavailable)
			return
		}

		state := generateState()
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   m.TLSConfig() != nil,
			MaxAge:   600,
		})

		authURL := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=%s&state=%s",
			url.QueryEscape(clientID),
			url.QueryEscape(redirectURL),
			url.QueryEscape("read:user user:email"),
			url.QueryEscape(state),
		)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

func githubCallbackHandler(db *database.Repository, m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID, clientSecret, _ := getOAuthConfig(m, "github")

		// Verify state
		stateCookie, err := r.Cookie("oauth_state")
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			jsonError(w, "invalid state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			jsonError(w, "missing code", http.StatusBadRequest)
			return
		}

		// Exchange code for token
		tokenReq, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(
			fmt.Sprintf("client_id=%s&client_secret=%s&code=%s", clientID, clientSecret, code),
		))
		if err != nil {
			jsonError(w, "failed to create token request", http.StatusInternalServerError)
			return
		}
		tokenReq.Header.Set("Accept", "application/json")
		tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		tokenResp, err := http.DefaultClient.Do(tokenReq)
		if err != nil {
			jsonError(w, "token exchange failed", http.StatusInternalServerError)
			return
		}
		defer tokenResp.Body.Close()

		var tokenData struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			Scope       string `json:"scope"`
		}
		if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
			jsonError(w, "failed to parse token", http.StatusInternalServerError)
			return
		}

		// Get user info
		userReq, err := http.NewRequest("GET", "https://api.github.com/user", nil)
		if err != nil {
			jsonError(w, "failed to create user request", http.StatusInternalServerError)
			return
		}
		userReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
		userResp, err := http.DefaultClient.Do(userReq)
		if err != nil {
			jsonError(w, "failed to get user info", http.StatusInternalServerError)
			return
		}
		defer userResp.Body.Close()

		var githubUser struct {
			ID        int64  `json:"id"`
			Login     string `json:"login"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.NewDecoder(userResp.Body).Decode(&githubUser); err != nil {
			jsonError(w, "failed to parse user info", http.StatusInternalServerError)
			return
		}

		// If email is private, fetch from emails endpoint
		email := githubUser.Email
		if email == "" {
			emailReq, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
			if err == nil {
				emailReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
				emailResp, err := http.DefaultClient.Do(emailReq)
				if err == nil {
					defer emailResp.Body.Close()
					var emails []struct {
						Email    string `json:"email"`
						Primary  bool   `json:"primary"`
						Verified bool   `json:"verified"`
					}
					if json.NewDecoder(emailResp.Body).Decode(&emails) == nil {
						for _, e := range emails {
							if e.Primary && e.Verified {
								email = e.Email
								break
							}
						}
					}
				}
			}
		}

		name := githubUser.Name
		if name == "" {
			name = githubUser.Login
		}

		handleOAuthCallback(w, r, db, "github", fmt.Sprintf("%d", githubUser.ID), email, name, githubUser.AvatarURL,
			tokenData.AccessToken, "", 0)
	}
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, db *database.Repository,
	provider, providerID, email, displayName, avatarURL, accessToken, refreshToken string, expiresIn int) {

	ctx := r.Context()
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Check if this OAuth provider is already linked
	existingProvider, err := db.GetOAuthProvider(ctx, provider, providerID)
	if err == nil && existingProvider != nil {
		// Provider exists - log user in
		user, err := db.GetUser(ctx, existingProvider.UserID)
		if err != nil {
			http.Redirect(w, r, frontendURL+"/login?error=user_not_found", http.StatusTemporaryRedirect)
			return
		}

		// Generate JWT
		token, err := generateJWT(user.ID)
		if err != nil {
			http.Redirect(w, r, frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
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
		_ = db.CreateSession(ctx, session)

		// Update OAuth tokens
		existingProvider.AccessToken = accessToken
		existingProvider.RefreshToken = refreshToken
		if expiresIn > 0 {
			existingProvider.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		}
		_ = db.UpdateOAuthProvider(ctx, existingProvider)

		http.Redirect(w, r, frontendURL+"/auth/callback?token="+token, http.StatusTemporaryRedirect)
		return
	}

	// Check if user with this email exists
	var user *database.User
	if email != "" {
		user, _ = db.GetUserByEmail(ctx, email)
	}

	if user == nil {
		// Create new user
		user = &database.User{
			ID:        uuid.New().String(),
			Email:     email,
			CreatedAt: time.Now(),
		}
		if err := db.CreateUser(ctx, user); err != nil {
			http.Redirect(w, r, frontendURL+"/login?error=user_creation_failed", http.StatusTemporaryRedirect)
			return
		}

		// Create gas bank account
		_, _ = db.GetOrCreateGasBankAccount(ctx, user.ID)
	}

	// Link OAuth provider to user
	oauthProvider := &database.OAuthProvider{
		UserID:      user.ID,
		Provider:    provider,
		ProviderID:  providerID,
		Email:       email,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		AccessToken: accessToken,
	}
	if refreshToken != "" {
		oauthProvider.RefreshToken = refreshToken
	}
	if expiresIn > 0 {
		oauthProvider.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	_ = db.CreateOAuthProvider(ctx, oauthProvider)

	// Update user email if not set
	if user.Email == "" && email != "" {
		_ = db.UpdateUserEmail(ctx, user.ID, email)
	}

	// Generate JWT
	token, err := generateJWT(user.ID)
	if err != nil {
		http.Redirect(w, r, frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
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
	_ = db.CreateSession(ctx, session)

	http.Redirect(w, r, frontendURL+"/auth/callback?token="+token, http.StatusTemporaryRedirect)
}

func listOAuthProvidersHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		providers, err := db.GetUserOAuthProviders(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get OAuth providers", http.StatusInternalServerError)
			return
		}

		// Sanitize - don't expose tokens
		sanitized := make([]map[string]interface{}, len(providers))
		for i, p := range providers {
			sanitized[i] = map[string]interface{}{
				"id":           p.ID,
				"provider":     p.Provider,
				"email":        p.Email,
				"display_name": p.DisplayName,
				"avatar_url":   p.AvatarURL,
				"created_at":   p.CreatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sanitized)
	}
}

func unlinkOAuthProviderHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		providerID := mux.Vars(r)["id"]

		// Check user has at least one other auth method (wallet or another OAuth)
		wallets, err := db.GetUserWallets(r.Context(), userID)
		if err != nil {
			log.Printf("Failed to get wallets for user %s: %v", userID, err)
		}
		providers, err := db.GetUserOAuthProviders(r.Context(), userID)
		if err != nil {
			log.Printf("Failed to get OAuth providers for user %s: %v", userID, err)
		}

		if len(wallets) == 0 && len(providers) <= 1 {
			jsonError(w, "cannot unlink last authentication method", http.StatusBadRequest)
			return
		}

		if err := db.DeleteOAuthProvider(r.Context(), providerID, userID); err != nil {
			jsonError(w, "failed to unlink provider", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func generateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
