package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// OAuth Provider Operations
// =============================================================================

// CreateOAuthProvider links an OAuth provider to a user.
func (r *Repository) CreateOAuthProvider(ctx context.Context, provider *OAuthProvider) error {
	data, err := r.client.request(ctx, "POST", "oauth_providers", provider, "")
	if err != nil {
		return err
	}
	var providers []OAuthProvider
	if err := json.Unmarshal(data, &providers); err != nil {
		return err
	}
	if len(providers) > 0 {
		provider.ID = providers[0].ID
	}
	return nil
}

// GetOAuthProvider retrieves an OAuth provider by provider and provider_id.
func (r *Repository) GetOAuthProvider(ctx context.Context, provider, providerID string) (*OAuthProvider, error) {
	query := fmt.Sprintf("provider=eq.%s&provider_id=eq.%s&limit=1", provider, providerID)
	data, err := r.client.request(ctx, "GET", "oauth_providers", nil, query)
	if err != nil {
		return nil, err
	}

	var providers []OAuthProvider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("oauth provider not found")
	}
	return &providers[0], nil
}

// GetUserOAuthProviders retrieves all OAuth providers for a user.
func (r *Repository) GetUserOAuthProviders(ctx context.Context, userID string) ([]OAuthProvider, error) {
	query := fmt.Sprintf("user_id=eq.%s&order=created_at.desc", userID)
	data, err := r.client.request(ctx, "GET", "oauth_providers", nil, query)
	if err != nil {
		return nil, err
	}

	var providers []OAuthProvider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

// UpdateOAuthProvider updates OAuth tokens.
func (r *Repository) UpdateOAuthProvider(ctx context.Context, provider *OAuthProvider) error {
	update := map[string]interface{}{
		"access_token":  provider.AccessToken,
		"refresh_token": provider.RefreshToken,
		"expires_at":    provider.ExpiresAt,
		"updated_at":    time.Now(),
	}
	query := fmt.Sprintf("id=eq.%s", provider.ID)
	_, err := r.client.request(ctx, "PATCH", "oauth_providers", update, query)
	return err
}

// DeleteOAuthProvider unlinks an OAuth provider from a user.
func (r *Repository) DeleteOAuthProvider(ctx context.Context, providerID, userID string) error {
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", providerID, userID)
	_, err := r.client.request(ctx, "DELETE", "oauth_providers", nil, query)
	return err
}
