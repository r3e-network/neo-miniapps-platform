// Package secrets implements an internal secret management service.
package secrets

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
)

const (
	ServiceID   = "secrets"
	ServiceName = "Secrets Service"
	Version     = "1.0.0"

	// Marble secret name for envelope encryption key (32 bytes).
	SecretKeyEnv = "SECRETS_MASTER_KEY"
)

// allowedServiceCallers are internal services permitted to fetch secrets on behalf of a user.
var allowedServiceCallers = map[string]struct{}{
	"oracle":       {},
	"confidential": {},
	"automation":   {},
	"mixer":        {},
	"vrf":          {},
}

// Required header for authenticated service-to-service calls.
const ServiceIDHeader = "X-Service-ID"

// Service implements the Secrets service.
type Service struct {
	*marble.Service
	db         Store
	encryptKey []byte
}

// Store captures the persistence surface needed by the secrets service.
type Store interface {
	GetSecrets(ctx context.Context, userID string) ([]database.Secret, error)
	CreateSecret(ctx context.Context, secret *database.Secret) error
}

// Config configures the Secrets service.
type Config struct {
	Marble     *marble.Marble
	DB         Store
	EncryptKey []byte // optional override; otherwise loaded from Marble secrets
}

// New creates a new Secrets service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      nil, // secrets service uses its own Store interface
	})

	key := cfg.EncryptKey
	if len(key) == 0 && cfg.Marble != nil {
		if k, ok := cfg.Marble.Secret(SecretKeyEnv); ok {
			key = k
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secrets service requires 32-byte master key (env: %s)", SecretKeyEnv)
	}

	s := &Service{
		Service:    base,
		db:         cfg.DB,
		encryptKey: key,
	}
	s.registerRoutes()
	return s, nil
}

// encrypt encrypts plaintext using AES-GCM with the master key.
func (s *Service) encrypt(plaintext []byte) ([]byte, error) {
	return crypto.Encrypt(s.encryptKey, plaintext)
}

// decrypt decrypts ciphertext using AES-GCM with the master key.
func (s *Service) decrypt(ciphertext []byte) ([]byte, error) {
	return crypto.Decrypt(s.encryptKey, ciphertext)
}
