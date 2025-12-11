package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// =============================================================================
// API Key Operations
// =============================================================================

// CreateAPIKey creates a new API key.
func (r *Repository) CreateAPIKey(ctx context.Context, key *APIKey) error {
	if key == nil {
		return fmt.Errorf("%w: api key cannot be nil", ErrInvalidInput)
	}
	if err := ValidateUserID(key.UserID); err != nil {
		return err
	}

	data, err := r.client.request(ctx, "POST", "api_keys", key, "")
	if err != nil {
		return fmt.Errorf("%w: create api key: %v", ErrDatabaseError, err)
	}
	var keys []APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("%w: unmarshal api keys: %v", ErrDatabaseError, err)
	}
	if len(keys) > 0 {
		key.ID = keys[0].ID
		key.CreatedAt = keys[0].CreatedAt
	}
	return nil
}

// GetAPIKeys retrieves all API keys for a user.
func (r *Repository) GetAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	if err := ValidateUserID(userID); err != nil {
		return nil, err
	}

	query := fmt.Sprintf("user_id=eq.%s&revoked=eq.false&order=created_at.desc", url.QueryEscape(userID))
	data, err := r.client.request(ctx, "GET", "api_keys", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get api keys: %v", ErrDatabaseError, err)
	}

	var keys []APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("%w: unmarshal api keys: %v", ErrDatabaseError, err)
	}
	return keys, nil
}

// GetAPIKeyByHash retrieves an API key by its hash.
func (r *Repository) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	if keyHash == "" {
		return nil, fmt.Errorf("%w: key_hash cannot be empty", ErrInvalidInput)
	}
	keyHash = SanitizeString(keyHash)

	query := fmt.Sprintf("key_hash=eq.%s&revoked=eq.false&limit=1", url.QueryEscape(keyHash))
	data, err := r.client.request(ctx, "GET", "api_keys", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get api key by hash: %v", ErrDatabaseError, err)
	}

	var keys []APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("%w: unmarshal api keys: %v", ErrDatabaseError, err)
	}
	if len(keys) == 0 {
		return nil, NewNotFoundError("api_key", keyHash)
	}
	return &keys[0], nil
}

// RevokeAPIKey revokes an API key.
func (r *Repository) RevokeAPIKey(ctx context.Context, keyID, userID string) error {
	if err := ValidateID(keyID); err != nil {
		return err
	}
	if err := ValidateUserID(userID); err != nil {
		return err
	}

	update := map[string]interface{}{
		"revoked": true,
	}
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", url.QueryEscape(keyID), url.QueryEscape(userID))
	_, err := r.client.request(ctx, "PATCH", "api_keys", update, query)
	if err != nil {
		return fmt.Errorf("%w: revoke api key: %v", ErrDatabaseError, err)
	}
	return nil
}

// UpdateAPIKeyLastUsed updates the last_used timestamp.
func (r *Repository) UpdateAPIKeyLastUsed(ctx context.Context, keyID string) error {
	if err := ValidateID(keyID); err != nil {
		return err
	}

	update := map[string]interface{}{
		"last_used": time.Now(),
	}
	_, err := r.client.request(ctx, "PATCH", "api_keys", update, "id=eq."+url.QueryEscape(keyID))
	if err != nil {
		return fmt.Errorf("%w: update api key last used: %v", ErrDatabaseError, err)
	}
	return nil
}
