// Package database provides mock implementations for shared service interfaces.
package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Secret Operations (implements SecretRepository)
// =============================================================================

func (m *MockRepository) GetSecrets(ctx context.Context, userID string) ([]Secret, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Secret
	for _, secret := range m.secrets {
		if secret.UserID == userID {
			result = append(result, *secret)
		}
	}
	return result, nil
}

func (m *MockRepository) CreateSecret(ctx context.Context, secret *Secret) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if secret.ID == "" {
		secret.ID = uuid.New().String()
	}
	if secret.CreatedAt.IsZero() {
		secret.CreatedAt = time.Now()
	}
	secret.UpdatedAt = time.Now()
	m.secrets[secret.ID] = secret
	return nil
}
