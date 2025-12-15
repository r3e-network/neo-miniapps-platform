package main

import (
	"context"
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

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/marble"
)

const (
	oauthStateCookieName      = "oauth_state"
	oauthTokenCookieName      = "sl_auth_token" // #nosec G101 -- cookie name, not a credential
	maxOAuthJSONResponseBytes = 256 << 10       // 256KiB
)

type oauthRepository interface {
	CreateOAuthProvider(ctx context.Context, provider *database.OAuthProvider) error
	CreateSession(ctx context.Context, session *database.UserSession) error
	CreateUser(ctx context.Context, user *database.User) error
	GetOAuthProvider(ctx context.Context, provider, providerID string) (*database.OAuthProvider, error)
	GetOrCreateGasBankAccount(ctx context.Context, userID string) (*database.GasBankAccount, error)
	GetUser(ctx context.Context, id string) (*database.User, error)
	GetUserByEmail(ctx context.Context, email string) (*database.User, error)
	UpdateOAuthProvider(ctx context.Context, provider *database.OAuthProvider) error
	UpdateUserEmail(ctx context.Context, userID, email string) error
}

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
		secure := isRequestSecure(r)
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
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

func googleCallbackHandler(db oauthRepository, m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		clientID, clientSecret, redirectURL := getOAuthConfig(m, "google")

		// Verify state
		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			jsonError(w, "invalid state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			jsonError(w, "missing code", http.StatusBadRequest)
			return
		}

		httpClient := &http.Client{Timeout: 15 * time.Second}
		if m != nil {
			httpClient = m.ExternalHTTPClient()
			httpClient.Timeout = 15 * time.Second
		}

		// Exchange code for tokens
		tokenValues := url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {redirectURL},
		}
		tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(tokenValues.Encode()))
		if err != nil {
			jsonError(w, "token exchange failed", http.StatusInternalServerError)
			return
		}
		tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		tokenReq.Header.Set("Accept", "application/json")

		tokenResp, err := httpClient.Do(tokenReq)
		if err != nil {
			jsonError(w, "token exchange failed", http.StatusInternalServerError)
			return
		}
		defer tokenResp.Body.Close()
		if tokenResp.StatusCode != http.StatusOK {
			body, truncated, readErr := httputil.ReadAllWithLimit(tokenResp.Body, 16<<10)
			if readErr != nil {
				log.Printf("google token exchange failed: status=%d read_error=%v", tokenResp.StatusCode, readErr)
			} else {
				msg := strings.TrimSpace(string(body))
				if truncated {
					msg += "...(truncated)"
				}
				log.Printf("google token exchange failed: status=%d body=%s", tokenResp.StatusCode, msg)
			}
			jsonError(w, "token exchange failed", http.StatusBadGateway)
			return
		}

		var tokenData struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			IDToken      string `json:"id_token"`
		}
		tokenBody, readErr := httputil.ReadAllStrict(tokenResp.Body, maxOAuthJSONResponseBytes)
		if readErr != nil {
			jsonError(w, "failed to parse token", http.StatusBadGateway)
			return
		}
		if decodeErr := json.Unmarshal(tokenBody, &tokenData); decodeErr != nil {
			jsonError(w, "failed to parse token", http.StatusBadGateway)
			return
		}
		if tokenData.AccessToken == "" {
			jsonError(w, "failed to parse token", http.StatusBadGateway)
			return
		}

		// Get user info
		userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", http.NoBody)
		if err != nil {
			jsonError(w, "failed to create user info request", http.StatusInternalServerError)
			return
		}
		userReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
		userResp, err := httpClient.Do(userReq)
		if err != nil {
			jsonError(w, "failed to get user info", http.StatusInternalServerError)
			return
		}
		defer userResp.Body.Close()
		if userResp.StatusCode != http.StatusOK {
			body, truncated, bodyReadErr := httputil.ReadAllWithLimit(userResp.Body, 16<<10)
			if bodyReadErr != nil {
				log.Printf("google user info failed: status=%d read_error=%v", userResp.StatusCode, bodyReadErr)
			} else {
				msg := strings.TrimSpace(string(body))
				if truncated {
					msg += "...(truncated)"
				}
				log.Printf("google user info failed: status=%d body=%s", userResp.StatusCode, msg)
			}
			jsonError(w, "failed to get user info", http.StatusBadGateway)
			return
		}

		var googleUser struct {
			ID      string `json:"id"`
			Email   string `json:"email"`
			Name    string `json:"name"`
			Picture string `json:"picture"`
		}
		userBody, readErr := httputil.ReadAllStrict(userResp.Body, maxOAuthJSONResponseBytes)
		if readErr != nil {
			jsonError(w, "failed to parse user info", http.StatusBadGateway)
			return
		}
		if decodeErr := json.Unmarshal(userBody, &googleUser); decodeErr != nil {
			jsonError(w, "failed to parse user info", http.StatusBadGateway)
			return
		}

		// Handle OAuth login/link (reuse the bounded context to avoid hanging DB calls).
		handleOAuthCallback(w, r.WithContext(ctx), db, "google", googleUser.ID, googleUser.Email, googleUser.Name, googleUser.Picture,
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
		secure := isRequestSecure(r)
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
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

func githubCallbackHandler(db oauthRepository, m *marble.Marble) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		clientID, clientSecret, _ := getOAuthConfig(m, "github")

		// Verify state
		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			jsonError(w, "invalid state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			jsonError(w, "missing code", http.StatusBadRequest)
			return
		}

		httpClient := &http.Client{Timeout: 15 * time.Second}
		if m != nil {
			httpClient = m.ExternalHTTPClient()
			httpClient.Timeout = 15 * time.Second
		}

		// Exchange code for token
		tokenValues := url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"code":          {code},
		}
		tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(tokenValues.Encode()))
		if err != nil {
			jsonError(w, "failed to create token request", http.StatusInternalServerError)
			return
		}
		tokenReq.Header.Set("Accept", "application/json")
		tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		tokenResp, err := httpClient.Do(tokenReq)
		if err != nil {
			jsonError(w, "token exchange failed", http.StatusInternalServerError)
			return
		}
		defer tokenResp.Body.Close()
		if tokenResp.StatusCode != http.StatusOK {
			body, truncated, readErr := httputil.ReadAllWithLimit(tokenResp.Body, 16<<10)
			if readErr != nil {
				log.Printf("github token exchange failed: status=%d read_error=%v", tokenResp.StatusCode, readErr)
			} else {
				msg := strings.TrimSpace(string(body))
				if truncated {
					msg += "...(truncated)"
				}
				log.Printf("github token exchange failed: status=%d body=%s", tokenResp.StatusCode, msg)
			}
			jsonError(w, "token exchange failed", http.StatusBadGateway)
			return
		}

		var tokenData struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			Scope       string `json:"scope"`
		}
		tokenBody, readErr := httputil.ReadAllStrict(tokenResp.Body, maxOAuthJSONResponseBytes)
		if readErr != nil {
			jsonError(w, "failed to parse token", http.StatusBadGateway)
			return
		}
		if decodeErr := json.Unmarshal(tokenBody, &tokenData); decodeErr != nil {
			jsonError(w, "failed to parse token", http.StatusBadGateway)
			return
		}
		if tokenData.AccessToken == "" {
			jsonError(w, "failed to parse token", http.StatusBadGateway)
			return
		}

		// Get user info
		userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", http.NoBody)
		if err != nil {
			jsonError(w, "failed to create user request", http.StatusInternalServerError)
			return
		}
		userReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
		userResp, err := httpClient.Do(userReq)
		if err != nil {
			jsonError(w, "failed to get user info", http.StatusInternalServerError)
			return
		}
		defer userResp.Body.Close()
		if userResp.StatusCode != http.StatusOK {
			body, truncated, bodyReadErr := httputil.ReadAllWithLimit(userResp.Body, 16<<10)
			if bodyReadErr != nil {
				log.Printf("github user info failed: status=%d read_error=%v", userResp.StatusCode, bodyReadErr)
			} else {
				msg := strings.TrimSpace(string(body))
				if truncated {
					msg += "...(truncated)"
				}
				log.Printf("github user info failed: status=%d body=%s", userResp.StatusCode, msg)
			}
			jsonError(w, "failed to get user info", http.StatusBadGateway)
			return
		}

		var githubUser struct {
			ID        int64  `json:"id"`
			Login     string `json:"login"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
		}
		userBody, readErr := httputil.ReadAllStrict(userResp.Body, maxOAuthJSONResponseBytes)
		if readErr != nil {
			jsonError(w, "failed to parse user info", http.StatusBadGateway)
			return
		}
		if decodeErr := json.Unmarshal(userBody, &githubUser); decodeErr != nil {
			jsonError(w, "failed to parse user info", http.StatusBadGateway)
			return
		}

		// If email is private, fetch from emails endpoint
		email := githubUser.Email
		if email == "" {
			emailReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", http.NoBody)
			if err == nil {
				emailReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
				emailResp, err := httpClient.Do(emailReq)
				if err == nil {
					defer emailResp.Body.Close()
					if emailResp.StatusCode == http.StatusOK {
						var emails []struct {
							Email    string `json:"email"`
							Primary  bool   `json:"primary"`
							Verified bool   `json:"verified"`
						}
						if emailsBody, readErr := httputil.ReadAllStrict(emailResp.Body, maxOAuthJSONResponseBytes); readErr == nil && json.Unmarshal(emailsBody, &emails) == nil {
							for _, e := range emails {
								if e.Primary && e.Verified {
									email = e.Email
									break
								}
							}
						}
					} else {
						body, truncated, readErr := httputil.ReadAllWithLimit(emailResp.Body, 16<<10)
						if readErr != nil {
							log.Printf("github email fetch failed: status=%d read_error=%v", emailResp.StatusCode, readErr)
						} else {
							msg := strings.TrimSpace(string(body))
							if truncated {
								msg += "...(truncated)"
							}
							log.Printf("github email fetch failed: status=%d body=%s", emailResp.StatusCode, msg)
						}
					}
				}
			}
		}

		name := githubUser.Name
		if name == "" {
			name = githubUser.Login
		}

		// Handle OAuth login/link (reuse the bounded context to avoid hanging DB calls).
		handleOAuthCallback(w, r.WithContext(ctx), db, "github", fmt.Sprintf("%d", githubUser.ID), email, name, githubUser.AvatarURL,
			tokenData.AccessToken, "", 0)
	}
}

// isOAuthCookieMode returns true if OAuth should use HTTP-only cookies instead of URL params.
// Default is true for security. Set OAUTH_COOKIE_MODE=false to disable.
func isOAuthCookieMode() bool {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("OAUTH_COOKIE_MODE")))
	if mode == "" {
		return true
	}
	switch mode {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func oauthTokenSameSite() http.SameSite {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("OAUTH_COOKIE_SAMESITE")))
	switch value {
	case "":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func isRequestSecure(r *http.Request) bool {
	if r != nil {
		if r.TLS != nil {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
			return true
		}
	}
	return false
}

// setAuthTokenCookie sets the JWT token in an HTTP-only secure cookie.
func setAuthTokenCookie(w http.ResponseWriter, token string, secure bool) {
	sameSite := oauthTokenSameSite()
	if sameSite == http.SameSiteNoneMode && !secure {
		sameSite = http.SameSiteLaxMode
	}

	maxAge := int(jwtExpiry.Round(time.Second).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   maxAge,
		Expires:  time.Now().Add(jwtExpiry),
	})
}

// clearOAuthStateCookie clears the oauth_state cookie after successful callback.
func clearOAuthStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Delete cookie
		Expires:  time.Unix(0, 0),
	})
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, db oauthRepository,
	provider, providerID, email, displayName, avatarURL, accessToken, refreshToken string, expiresIn int,
) {

	ctx := r.Context()
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	isSecure := isRequestSecure(r)
	useCookieMode := isOAuthCookieMode()

	// Check if this OAuth provider is already linked
	existingProvider, err := db.GetOAuthProvider(ctx, provider, providerID)
	if err == nil && existingProvider != nil {
		// Provider exists - log user in
		user, userErr := db.GetUser(ctx, existingProvider.UserID)
		if userErr != nil {
			http.Redirect(w, r, frontendURL+"/login?error=user_not_found", http.StatusTemporaryRedirect)
			return
		}

		// Generate JWT
		token, tokenErr := generateJWT(user.ID)
		if tokenErr != nil {
			http.Redirect(w, r, frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
			return
		}

		// Create session
		tokenHash := hashToken(token)
		session := &database.UserSession{
			UserID:    user.ID,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(jwtExpiry),
			CreatedAt: time.Now(),
		}
		if createSessionErr := db.CreateSession(ctx, session); createSessionErr != nil {
			http.Redirect(w, r, frontendURL+"/login?error=session_create_failed", http.StatusTemporaryRedirect)
			return
		}

		// Update OAuth tokens
		existingProvider.AccessToken = accessToken
		existingProvider.RefreshToken = refreshToken
		if expiresIn > 0 {
			existingProvider.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		}
		if updateProviderErr := db.UpdateOAuthProvider(ctx, existingProvider); updateProviderErr != nil {
			http.Redirect(w, r, frontendURL+"/login?error=provider_update_failed", http.StatusTemporaryRedirect)
			return
		}

		// Clear oauth_state cookie
		clearOAuthStateCookie(w, isSecure)

		// Redirect with token in cookie (secure) or URL (legacy fallback)
		if !useCookieMode {
			http.Redirect(w, r, frontendURL+"/login?error=cookie_mode_required", http.StatusTemporaryRedirect)
			return
		}

		setAuthTokenCookie(w, token, isSecure)
		http.Redirect(w, r, frontendURL+"/auth/callback?status=success", http.StatusTemporaryRedirect)
		return
	}

	// Check if user with this email exists
	var user *database.User
	if email != "" {
		if found, findErr := db.GetUserByEmail(ctx, email); findErr == nil {
			user = found
		}
	}

	if user == nil {
		// Create new user
		user = &database.User{
			ID:        uuid.New().String(),
			Email:     email,
			CreatedAt: time.Now(),
		}
		if createErr := db.CreateUser(ctx, user); createErr != nil {
			http.Redirect(w, r, frontendURL+"/login?error=user_creation_failed", http.StatusTemporaryRedirect)
			return
		}

		// Create gas bank account
		if _, gasErr := db.GetOrCreateGasBankAccount(ctx, user.ID); gasErr != nil {
			http.Redirect(w, r, frontendURL+"/login?error=gasbank_create_failed", http.StatusTemporaryRedirect)
			return
		}
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
	if createProviderErr := db.CreateOAuthProvider(ctx, oauthProvider); createProviderErr != nil {
		http.Redirect(w, r, frontendURL+"/login?error=provider_link_failed", http.StatusTemporaryRedirect)
		return
	}

	// Update user email if not set
	if user.Email == "" && email != "" {
		if updateEmailErr := db.UpdateUserEmail(ctx, user.ID, email); updateEmailErr != nil {
			http.Redirect(w, r, frontendURL+"/login?error=user_email_update_failed", http.StatusTemporaryRedirect)
			return
		}
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
		ExpiresAt: time.Now().Add(jwtExpiry),
		CreatedAt: time.Now(),
	}
	if err := db.CreateSession(ctx, session); err != nil {
		http.Redirect(w, r, frontendURL+"/login?error=session_create_failed", http.StatusTemporaryRedirect)
		return
	}

	// Clear oauth_state cookie
	clearOAuthStateCookie(w, isSecure)

	// Redirect with token in cookie (secure) or URL (legacy fallback)
	if !useCookieMode {
		http.Redirect(w, r, frontendURL+"/login?error=cookie_mode_required", http.StatusTemporaryRedirect)
		return
	}

	setAuthTokenCookie(w, token, isSecure)
	http.Redirect(w, r, frontendURL+"/auth/callback?status=success", http.StatusTemporaryRedirect)
}

func listOAuthProvidersHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		providers, err := db.GetUserOAuthProviders(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get OAuth providers", http.StatusInternalServerError)
			return
		}

		// Sanitize - don't expose tokens
		sanitized := make([]map[string]interface{}, len(providers))
		for i := range providers {
			p := &providers[i]
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
		if encodeErr := json.NewEncoder(w).Encode(sanitized); encodeErr != nil {
			log.Printf("encode oauth providers: %v", encodeErr)
		}
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
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("state-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
