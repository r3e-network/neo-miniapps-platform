// Package secrets provides user secret management for internal services.
package secrets

import "time"

// SecretRecord is returned to callers (plaintext excluded unless specifically requested).
type SecretRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateSecretInput is used to create or update a secret.
type CreateSecretInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// GetSecretResponse returns decrypted secret value.
type GetSecretResponse struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Version int    `json:"version"`
}
