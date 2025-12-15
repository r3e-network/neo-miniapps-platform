package database

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

func TestCreateOAuthProviderEncryptsTokens(t *testing.T) {
	t.Setenv(oauthTokensMasterKeyEnv, strings.Repeat("11", 32)) // 32 bytes hex

	accessToken := "access-token-plain"
	refreshToken := "refresh-token-plain"

	client := &Client{
		url:        "http://supabase.test",
		serviceKey: "test-key",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
				}
				if r.URL.Path != "/rest/v1/oauth_providers" {
					t.Fatalf("path = %s, want /rest/v1/oauth_providers", r.URL.Path)
				}

				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}

				gotAccess, ok := body["access_token"].(string)
				if !ok {
					t.Fatalf("access_token type = %T, want string", body["access_token"])
				}
				gotRefresh, ok := body["refresh_token"].(string)
				if !ok {
					t.Fatalf("refresh_token type = %T, want string", body["refresh_token"])
				}

				if gotAccess == accessToken || strings.Contains(gotAccess, accessToken) {
					t.Fatalf("access_token should be encrypted, got %q", gotAccess)
				}
				if gotRefresh == refreshToken || strings.Contains(gotRefresh, refreshToken) {
					t.Fatalf("refresh_token should be encrypted, got %q", gotRefresh)
				}

				key, err := oauthTokensMasterKey()
				if err != nil {
					t.Fatalf("oauthTokensMasterKey() error = %v", err)
				}

				decAccess, err := crypto.DecryptEnvelope(key, []byte("user-123"), oauthTokensEnvelopeInfo, []byte(gotAccess))
				if err != nil {
					t.Fatalf("DecryptEnvelope(access) error = %v", err)
				}
				if string(decAccess) != accessToken {
					t.Fatalf("decrypt(access) = %q, want %q", string(decAccess), accessToken)
				}

				decRefresh, err := crypto.DecryptEnvelope(key, []byte("user-123"), oauthTokensEnvelopeInfo, []byte(gotRefresh))
				if err != nil {
					t.Fatalf("DecryptEnvelope(refresh) error = %v", err)
				}
				if string(decRefresh) != refreshToken {
					t.Fatalf("decrypt(refresh) = %q, want %q", string(decRefresh), refreshToken)
				}

				return jsonResponse(r, http.StatusOK, `[{"id":"prov-1"}]`), nil
			}),
		},
	}
	repo := NewRepository(client)

	provider := &OAuthProvider{
		UserID:       "user-123",
		Provider:     "google",
		ProviderID:   "provider-abc",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	if err := repo.CreateOAuthProvider(context.Background(), provider); err != nil {
		t.Fatalf("CreateOAuthProvider() error = %v", err)
	}
	if provider.ID != "prov-1" {
		t.Fatalf("provider.ID = %s, want prov-1", provider.ID)
	}
}

func TestUpdateOAuthProviderEncryptsTokens(t *testing.T) {
	t.Setenv(oauthTokensMasterKeyEnv, strings.Repeat("22", 32)) // 32 bytes hex

	accessToken := "access-token-plain"
	refreshToken := "refresh-token-plain"

	client := &Client{
		url:        "http://supabase.test",
		serviceKey: "test-key",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPatch {
					t.Fatalf("method = %s, want %s", r.Method, http.MethodPatch)
				}
				if r.URL.Path != "/rest/v1/oauth_providers" {
					t.Fatalf("path = %s, want /rest/v1/oauth_providers", r.URL.Path)
				}
				if got := r.URL.Query().Get("id"); got != "eq.prov-1" {
					t.Fatalf("query id = %q, want %q", got, "eq.prov-1")
				}

				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}

				gotAccess, ok := body["access_token"].(string)
				if !ok {
					t.Fatalf("access_token type = %T, want string", body["access_token"])
				}
				gotRefresh, ok := body["refresh_token"].(string)
				if !ok {
					t.Fatalf("refresh_token type = %T, want string", body["refresh_token"])
				}

				if gotAccess == accessToken || strings.Contains(gotAccess, accessToken) {
					t.Fatalf("access_token should be encrypted, got %q", gotAccess)
				}
				if gotRefresh == refreshToken || strings.Contains(gotRefresh, refreshToken) {
					t.Fatalf("refresh_token should be encrypted, got %q", gotRefresh)
				}

				key, err := oauthTokensMasterKey()
				if err != nil {
					t.Fatalf("oauthTokensMasterKey() error = %v", err)
				}

				decAccess, err := crypto.DecryptEnvelope(key, []byte("user-123"), oauthTokensEnvelopeInfo, []byte(gotAccess))
				if err != nil {
					t.Fatalf("DecryptEnvelope(access) error = %v", err)
				}
				if string(decAccess) != accessToken {
					t.Fatalf("decrypt(access) = %q, want %q", string(decAccess), accessToken)
				}

				decRefresh, err := crypto.DecryptEnvelope(key, []byte("user-123"), oauthTokensEnvelopeInfo, []byte(gotRefresh))
				if err != nil {
					t.Fatalf("DecryptEnvelope(refresh) error = %v", err)
				}
				if string(decRefresh) != refreshToken {
					t.Fatalf("decrypt(refresh) = %q, want %q", string(decRefresh), refreshToken)
				}

				return jsonResponse(r, http.StatusOK, `[]`), nil
			}),
		},
	}
	repo := NewRepository(client)

	provider := &OAuthProvider{
		ID:           "prov-1",
		UserID:       "user-123",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	if err := repo.UpdateOAuthProvider(context.Background(), provider); err != nil {
		t.Fatalf("UpdateOAuthProvider() error = %v", err)
	}
}
