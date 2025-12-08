package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// API Key Operations
// =============================================================================

// CreateAPIKey creates a new API key.
func (r *Repository) CreateAPIKey(ctx context.Context, key *APIKey) error {
	data, err := r.client.request(ctx, "POST", "api_keys", key, "")
	if err != nil {
		return err
	}
	var keys []APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	if len(keys) > 0 {
		key.ID = keys[0].ID
		key.CreatedAt = keys[0].CreatedAt
	}
	return nil
}

// GetAPIKeys retrieves all API keys for a user.
func (r *Repository) GetAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	query := fmt.Sprintf("user_id=eq.%s&revoked=eq.false&order=created_at.desc", userID)
	data, err := r.client.request(ctx, "GET", "api_keys", nil, query)
	if err != nil {
		return nil, err
	}

	var keys []APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// GetAPIKeyByHash retrieves an API key by its hash.
func (r *Repository) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	query := fmt.Sprintf("key_hash=eq.%s&revoked=eq.false&limit=1", keyHash)
	data, err := r.client.request(ctx, "GET", "api_keys", nil, query)
	if err != nil {
		return nil, err
	}

	var keys []APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("API key not found")
	}
	return &keys[0], nil
}

// RevokeAPIKey revokes an API key.
func (r *Repository) RevokeAPIKey(ctx context.Context, keyID, userID string) error {
	update := map[string]interface{}{
		"revoked": true,
	}
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", keyID, userID)
	_, err := r.client.request(ctx, "PATCH", "api_keys", update, query)
	return err
}

// UpdateAPIKeyLastUsed updates the last_used timestamp.
func (r *Repository) UpdateAPIKeyLastUsed(ctx context.Context, keyID string) error {
	update := map[string]interface{}{
		"last_used": time.Now(),
	}
	_, err := r.client.request(ctx, "PATCH", "api_keys", update, "id=eq."+keyID)
	return err
}
