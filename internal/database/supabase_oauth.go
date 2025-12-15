package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// OAuth Provider Operations
// =============================================================================

// CreateOAuthProvider links an OAuth provider to a user.
func (r *Repository) CreateOAuthProvider(ctx context.Context, provider *OAuthProvider) error {
	if provider == nil {
		return fmt.Errorf("%w: oauth provider cannot be nil", ErrInvalidInput)
	}
	if err := ValidateUserID(provider.UserID); err != nil {
		return err
	}

	payload := *provider
	if key, ok, err := r.oauthTokensMasterKeyOptional(); err != nil {
		return fmt.Errorf("%w: oauth token encryption key: %v", ErrInvalidInput, err)
	} else if ok {
		if payload.AccessToken != "" {
			enc, err := crypto.EncryptEnvelope(key, []byte(provider.UserID), oauthTokensEnvelopeInfo, []byte(payload.AccessToken))
			if err != nil {
				return fmt.Errorf("%w: encrypt access token: %v", ErrDatabaseError, err)
			}
			payload.AccessToken = string(enc)
		}
		if payload.RefreshToken != "" {
			enc, err := crypto.EncryptEnvelope(key, []byte(provider.UserID), oauthTokensEnvelopeInfo, []byte(payload.RefreshToken))
			if err != nil {
				return fmt.Errorf("%w: encrypt refresh token: %v", ErrDatabaseError, err)
			}
			payload.RefreshToken = string(enc)
		}
	}

	data, err := r.client.request(ctx, "POST", "oauth_providers", &payload, "")
	if err != nil {
		return fmt.Errorf("%w: create oauth provider: %v", ErrDatabaseError, err)
	}
	var providers []OAuthProvider
	if err := json.Unmarshal(data, &providers); err != nil {
		return fmt.Errorf("%w: unmarshal oauth providers: %v", ErrDatabaseError, err)
	}
	if len(providers) > 0 {
		provider.ID = providers[0].ID
	}
	return nil
}

// GetOAuthProvider retrieves an OAuth provider by provider and provider_id.
func (r *Repository) GetOAuthProvider(ctx context.Context, provider, providerID string) (*OAuthProvider, error) {
	if provider == "" {
		return nil, fmt.Errorf("%w: provider cannot be empty", ErrInvalidInput)
	}
	if providerID == "" {
		return nil, fmt.Errorf("%w: provider_id cannot be empty", ErrInvalidInput)
	}
	provider = SanitizeString(provider)
	providerID = SanitizeString(providerID)

	query := fmt.Sprintf("provider=eq.%s&provider_id=eq.%s&limit=1", provider, providerID)
	data, err := r.client.request(ctx, "GET", "oauth_providers", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get oauth provider: %v", ErrDatabaseError, err)
	}

	var providers []OAuthProvider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("%w: unmarshal oauth providers: %v", ErrDatabaseError, err)
	}
	if len(providers) == 0 {
		return nil, NewNotFoundError("oauth_provider", providerID)
	}
	return &providers[0], nil
}

// GetUserOAuthProviders retrieves all OAuth providers for a user.
func (r *Repository) GetUserOAuthProviders(ctx context.Context, userID string) ([]OAuthProvider, error) {
	if err := ValidateUserID(userID); err != nil {
		return nil, err
	}

	query := fmt.Sprintf("user_id=eq.%s&order=created_at.desc", userID)
	data, err := r.client.request(ctx, "GET", "oauth_providers", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get user oauth providers: %v", ErrDatabaseError, err)
	}

	var providers []OAuthProvider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("%w: unmarshal oauth providers: %v", ErrDatabaseError, err)
	}
	return providers, nil
}

// UpdateOAuthProvider updates OAuth tokens.
func (r *Repository) UpdateOAuthProvider(ctx context.Context, provider *OAuthProvider) error {
	if provider == nil {
		return fmt.Errorf("%w: oauth provider cannot be nil", ErrInvalidInput)
	}
	if err := ValidateID(provider.ID); err != nil {
		return err
	}
	if err := ValidateUserID(provider.UserID); err != nil {
		return err
	}

	accessToken := provider.AccessToken
	refreshToken := provider.RefreshToken
	if key, ok, err := r.oauthTokensMasterKeyOptional(); err != nil {
		return fmt.Errorf("%w: oauth token encryption key: %v", ErrInvalidInput, err)
	} else if ok {
		if accessToken != "" {
			enc, err := crypto.EncryptEnvelope(key, []byte(provider.UserID), oauthTokensEnvelopeInfo, []byte(accessToken))
			if err != nil {
				return fmt.Errorf("%w: encrypt access token: %v", ErrDatabaseError, err)
			}
			accessToken = string(enc)
		}
		if refreshToken != "" {
			enc, err := crypto.EncryptEnvelope(key, []byte(provider.UserID), oauthTokensEnvelopeInfo, []byte(refreshToken))
			if err != nil {
				return fmt.Errorf("%w: encrypt refresh token: %v", ErrDatabaseError, err)
			}
			refreshToken = string(enc)
		}
	}

	update := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    provider.ExpiresAt,
		"updated_at":    time.Now(),
	}
	query := fmt.Sprintf("id=eq.%s", provider.ID)
	_, err := r.client.request(ctx, "PATCH", "oauth_providers", update, query)
	if err != nil {
		return fmt.Errorf("%w: update oauth provider: %v", ErrDatabaseError, err)
	}
	return nil
}

// DeleteOAuthProvider unlinks an OAuth provider from a user.
func (r *Repository) DeleteOAuthProvider(ctx context.Context, providerID, userID string) error {
	if err := ValidateID(providerID); err != nil {
		return err
	}
	if err := ValidateUserID(userID); err != nil {
		return err
	}

	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", providerID, userID)
	_, err := r.client.request(ctx, "DELETE", "oauth_providers", nil, query)
	if err != nil {
		return fmt.Errorf("%w: delete oauth provider: %v", ErrDatabaseError, err)
	}
	return nil
}
