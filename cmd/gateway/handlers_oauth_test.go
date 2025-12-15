package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/R3E-Network/service_layer/internal/database"
)

type mockOAuthRepo struct {
	existingProvider *database.OAuthProvider
	user             *database.User
	userByEmail      *database.User
	sessions         []*database.UserSession
	createdUsers     []*database.User
	createdProviders []*database.OAuthProvider
	updatedProvider  *database.OAuthProvider
}

func (m *mockOAuthRepo) GetOAuthProvider(_ context.Context, _, _ string) (*database.OAuthProvider, error) {
	if m.existingProvider == nil {
		return nil, errors.New("not found")
	}
	return m.existingProvider, nil
}

func (m *mockOAuthRepo) GetUser(_ context.Context, _ string) (*database.User, error) {
	if m.user == nil {
		return nil, errors.New("not found")
	}
	return m.user, nil
}

func (m *mockOAuthRepo) CreateSession(_ context.Context, session *database.UserSession) error {
	m.sessions = append(m.sessions, session)
	return nil
}

func (m *mockOAuthRepo) UpdateOAuthProvider(_ context.Context, provider *database.OAuthProvider) error {
	m.updatedProvider = provider
	return nil
}

func (m *mockOAuthRepo) GetUserByEmail(_ context.Context, _ string) (*database.User, error) {
	if m.userByEmail == nil {
		return nil, errors.New("not found")
	}
	return m.userByEmail, nil
}

func (m *mockOAuthRepo) CreateUser(_ context.Context, user *database.User) error {
	m.createdUsers = append(m.createdUsers, user)
	return nil
}

func (m *mockOAuthRepo) GetOrCreateGasBankAccount(_ context.Context, _ string) (*database.GasBankAccount, error) {
	return &database.GasBankAccount{ID: "acct"}, nil
}

func (m *mockOAuthRepo) CreateOAuthProvider(_ context.Context, provider *database.OAuthProvider) error {
	m.createdProviders = append(m.createdProviders, provider)
	return nil
}

func (m *mockOAuthRepo) UpdateUserEmail(_ context.Context, _, _ string) error {
	return nil
}

func TestHandleOAuthCallbackSetsCookie(t *testing.T) {
	t.Setenv("FRONTEND_URL", "http://example.com")
	t.Setenv("OAUTH_COOKIE_MODE", "true")
	originalSecret := jwtSecret
	jwtSecret = []byte("test-secret-value-has-32-bytes!!")
	t.Cleanup(func() { jwtSecret = originalSecret })

	repo := &mockOAuthRepo{
		existingProvider: &database.OAuthProvider{ID: "provider-1", UserID: "user-1"},
		user:             &database.User{ID: "user-1", Email: "user@example.com"},
	}

	req := httptest.NewRequest("GET", "/auth/google/callback", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	res := httptest.NewRecorder()

	handleOAuthCallback(res, req, repo, "google", "provider-1", "user@example.com", "User", "", "access", "refresh", 3600)

	result := res.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect, got %d", result.StatusCode)
	}
	if got := result.Header.Get("Location"); got != "http://example.com/auth/callback?status=success" {
		t.Fatalf("unexpected redirect location %s", got)
	}

	var authCookie, stateCookie *http.Cookie
	for _, c := range result.Cookies() {
		if c.Name == oauthTokenCookieName {
			authCookie = c
		} else if c.Name == oauthStateCookieName {
			stateCookie = c
		}
	}

	if authCookie == nil {
		t.Fatal("auth cookie not set")
	}
	if !authCookie.HttpOnly || !authCookie.Secure || authCookie.MaxAge != 86400 || authCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("auth cookie missing security attributes: %+v", authCookie)
	}
	if stateCookie == nil || stateCookie.MaxAge >= 0 {
		t.Fatalf("state cookie not cleared: %+v", stateCookie)
	}
	if len(repo.sessions) != 1 {
		t.Fatalf("expected session to be created")
	}
}

func TestHandleOAuthCallbackLegacyRedirect(t *testing.T) {
	// Legacy mode (token in URL) is no longer supported for security reasons.
	// When OAUTH_COOKIE_MODE=false, the system returns an error redirect.
	t.Setenv("FRONTEND_URL", "http://example.com")
	t.Setenv("OAUTH_COOKIE_MODE", "false")
	originalSecret := jwtSecret
	jwtSecret = []byte("test-secret-value-has-32-bytes!!")
	t.Cleanup(func() { jwtSecret = originalSecret })

	repo := &mockOAuthRepo{
		existingProvider: &database.OAuthProvider{ID: "provider-1", UserID: "user-1"},
		user:             &database.User{ID: "user-1", Email: "user@example.com"},
	}

	req := httptest.NewRequest("GET", "/auth/google/callback", nil)
	res := httptest.NewRecorder()

	handleOAuthCallback(res, req, repo, "google", "provider-1", "user@example.com", "User", "", "access", "", 3600)

	result := res.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect, got %d", result.StatusCode)
	}
	location := result.Header.Get("Location")
	// Legacy mode is deprecated - expect error redirect instead of token in URL
	if !strings.Contains(location, "error=cookie_mode_required") {
		t.Fatalf("expected cookie_mode_required error, got %s", location)
	}
}

func TestHandleOAuthCallbackCreatesUser(t *testing.T) {
	t.Setenv("FRONTEND_URL", "http://example.com")
	t.Setenv("OAUTH_COOKIE_MODE", "true")
	originalSecret := jwtSecret
	jwtSecret = []byte("test-secret-value-has-32-bytes!!")
	t.Cleanup(func() { jwtSecret = originalSecret })

	repo := &mockOAuthRepo{}
	req := httptest.NewRequest("GET", "/auth/github/callback", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	res := httptest.NewRecorder()

	handleOAuthCallback(res, req, repo, "github", "provider-2", "new@example.com", "New User", "", "access", "", 0)

	if len(repo.createdUsers) != 1 {
		t.Fatalf("expected user to be created")
	}
	if len(repo.createdProviders) != 1 {
		t.Fatalf("expected oauth provider to be linked")
	}
	if len(repo.sessions) != 1 {
		t.Fatalf("expected session to be created")
	}
}
