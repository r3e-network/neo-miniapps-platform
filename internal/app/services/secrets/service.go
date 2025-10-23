package secrets

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/app/domain/secret"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Resolver exposes secret lookup for other services.
type Resolver interface {
	ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error)
}

// Service manages account secrets.
type Service struct {
	store  storage.SecretStore
	log    *logger.Logger
	cipher Cipher
}

// Option configures the secrets service.
type Option func(*Service)

// WithCipher supplies a custom cipher used to encrypt/decrypt stored values.
func WithCipher(c Cipher) Option {
	return func(s *Service) { s.cipher = c }
}

// New creates a secrets service.
func New(store storage.SecretStore, log *logger.Logger, opts ...Option) *Service {
	if log == nil {
		log = logger.NewDefault("secrets")
	}
	svc := &Service{
		store:  store,
		log:    log,
		cipher: noopCipher{},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// SetCipher overrides the encryption cipher used by the service.
func (s *Service) SetCipher(cipher Cipher) {
	if cipher == nil {
		s.cipher = noopCipher{}
		return
	}
	s.cipher = cipher
}

// Create stores a new secret value.
func (s *Service) Create(ctx context.Context, accountID, name, value string) (secret.Metadata, error) {
	if err := validateName(name); err != nil {
		return secret.Metadata{}, err
	}
	if value == "" {
		return secret.Metadata{}, fmt.Errorf("value is required")
	}

	ciphertext, err := s.encrypt(value)
	if err != nil {
		return secret.Metadata{}, err
	}

	record := secret.Secret{
		ID:        uuid.NewString(),
		AccountID: accountID,
		Name:      name,
		Value:     ciphertext,
	}

	stored, err := s.store.CreateSecret(ctx, record)
	if err != nil {
		return secret.Metadata{}, err
	}
	return stored.ToMetadata(), nil
}

// Update replaces the secret value.
func (s *Service) Update(ctx context.Context, accountID, name, value string) (secret.Metadata, error) {
	if err := validateName(name); err != nil {
		return secret.Metadata{}, err
	}
	if value == "" {
		return secret.Metadata{}, fmt.Errorf("value is required")
	}

	ciphertext, err := s.encrypt(value)
	if err != nil {
		return secret.Metadata{}, err
	}

	record := secret.Secret{
		AccountID: accountID,
		Name:      name,
		Value:     ciphertext,
	}

	updated, err := s.store.UpdateSecret(ctx, record)
	if err != nil {
		return secret.Metadata{}, err
	}
	return updated.ToMetadata(), nil
}

// Get retrieves a secret including its decrypted value.
func (s *Service) Get(ctx context.Context, accountID, name string) (secret.Secret, error) {
	if err := validateName(name); err != nil {
		return secret.Secret{}, err
	}

	record, err := s.store.GetSecret(ctx, accountID, name)
	if err != nil {
		return secret.Secret{}, err
	}

	plaintext, err := s.decrypt(record.Value)
	if err != nil {
		return secret.Secret{}, err
	}
	record.Value = plaintext
	return record, nil
}

// List returns metadata for all secrets on the account.
func (s *Service) List(ctx context.Context, accountID string) ([]secret.Metadata, error) {
	records, err := s.store.ListSecrets(ctx, accountID)
	if err != nil {
		return nil, err
	}

	result := make([]secret.Metadata, 0, len(records))
	for _, rec := range records {
		result = append(result, rec.ToMetadata())
	}
	return result, nil
}

// Delete removes a secret.
func (s *Service) Delete(ctx context.Context, accountID, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	return s.store.DeleteSecret(ctx, accountID, name)
}

// ResolveSecrets returns a map of secret name -> plaintext value.
func (s *Service) ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	resolved := make(map[string]string, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		record, err := s.store.GetSecret(ctx, accountID, name)
		if err != nil {
			return nil, err
		}
		plaintext, err := s.decrypt(record.Value)
		if err != nil {
			return nil, err
		}
		resolved[name] = plaintext
	}
	return resolved, nil
}

func (s *Service) encrypt(value string) (string, error) {
	ciphertext, err := s.cipher.Encrypt([]byte(value))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *Service) decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	buf, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	plaintext, err := s.cipher.Decrypt(buf)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func validateName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("name is required")
	}
	if strings.Contains(trimmed, "|") {
		return fmt.Errorf("name cannot contain '|'")
	}
	return nil
}
