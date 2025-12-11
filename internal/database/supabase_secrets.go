// Deprecated: This file is deprecated. Use services/neostore/supabase package instead.
// This file is kept for backward compatibility with existing code that uses
// database.RepositoryInterface. New code should use:
//
//	import secretssupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"
//	secretsRepo := secretssupabase.NewRepository(baseRepo)
package database

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetSecrets retrieves all secrets for a user.
// Deprecated: Use services/neostore/supabase.Repository.GetSecrets instead.
func (r *Repository) GetSecrets(ctx context.Context, userID string) ([]Secret, error) {
	if err := ValidateUserID(userID); err != nil {
		return nil, err
	}

	data, err := r.client.request(ctx, "GET", "secrets", nil, "user_id=eq."+userID)
	if err != nil {
		return nil, fmt.Errorf("%w: get secrets: %v", ErrDatabaseError, err)
	}

	var secrets []Secret
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("%w: unmarshal secrets: %v", ErrDatabaseError, err)
	}
	return secrets, nil
}

// CreateSecret creates a new secret.
func (r *Repository) CreateSecret(ctx context.Context, secret *Secret) error {
	if secret == nil {
		return fmt.Errorf("%w: secret cannot be nil", ErrInvalidInput)
	}
	if err := ValidateUserID(secret.UserID); err != nil {
		return err
	}
	if secret.Name == "" {
		return fmt.Errorf("%w: secret name cannot be empty", ErrInvalidInput)
	}

	_, err := r.client.request(ctx, "POST", "secrets", secret, "")
	if err != nil {
		return fmt.Errorf("%w: create secret: %v", ErrDatabaseError, err)
	}
	return nil
}
