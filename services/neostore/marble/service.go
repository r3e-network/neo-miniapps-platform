// Package secrets implements an internal secret management service.
package neostoremarble

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/serviceauth"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	neostoresupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"
)

const (
	ServiceID   = "neostore"
	ServiceName = "NeoStore Service"
	Version     = "1.0.0"

	// Marble secret name for envelope encryption key (32 bytes).
	SecretKeyEnv = "SECRETS_MASTER_KEY"
)

// allowedServiceCallers are internal services permitted to fetch secrets on behalf of a user.
var allowedServiceCallers = map[string]struct{}{
	"neooracle":  {},
	"neocompute": {},
	"neoflow":    {},
	"neovault":   {},
	"neorand":    {},
}

// Required header for authenticated service-to-service calls.
const ServiceIDHeader = serviceauth.ServiceIDHeader

// Service implements the NeoStore service.
type Service struct {
	*commonservice.BaseService
	db         Store
	encryptKey []byte
}

// Store captures the persistence surface needed by the neostore service.
type Store interface {
	GetSecrets(ctx context.Context, userID string) ([]neostoresupabase.Secret, error)
	GetSecretByName(ctx context.Context, userID, name string) (*neostoresupabase.Secret, error)
	CreateSecret(ctx context.Context, secret *neostoresupabase.Secret) error
	UpdateSecret(ctx context.Context, secret *neostoresupabase.Secret) error
	DeleteSecret(ctx context.Context, userID, name string) error
	GetAllowedServices(ctx context.Context, userID, secretName string) ([]string, error)
	SetAllowedServices(ctx context.Context, userID, secretName string, services []string) error
	CreateAuditLog(ctx context.Context, log *neostoresupabase.AuditLog) error
	GetAuditLogs(ctx context.Context, userID string, limit int) ([]neostoresupabase.AuditLog, error)
	GetAuditLogsForSecret(ctx context.Context, userID, secretName string, limit int) ([]neostoresupabase.AuditLog, error)
}

// Config configures the NeoStore service.
type Config struct {
	Marble     *marble.Marble
	DB         Store
	EncryptKey []byte // optional override; otherwise loaded from Marble secrets
}

// New creates a new NeoStore service.
func New(cfg Config) (*Service, error) {
	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      nil, // neostore service uses its own Store interface
	})

	key := cfg.EncryptKey
	if len(key) == 0 && cfg.Marble != nil {
		if k, ok := cfg.Marble.Secret(SecretKeyEnv); ok {
			key = k
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("neostore service requires 32-byte master key (env: %s)", SecretKeyEnv)
	}

	s := &Service{
		BaseService: base,
		db:          cfg.DB,
		encryptKey:  key,
	}

	// Register standard routes (/health, /info) plus service-specific routes
	base.RegisterStandardRoutes()
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
