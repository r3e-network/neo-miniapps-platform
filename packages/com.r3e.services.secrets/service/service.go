package secrets

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// CallerService identifies which service is requesting secret access.
// Used for ACL enforcement aligned with SecretsVault.cs contract.
type CallerService string

const (
	CallerOracle     CallerService = "oracle"
	CallerAutomation CallerService = "automation"
	CallerFunctions  CallerService = "functions"
	CallerJAM        CallerService = "jam"
)

// Resolver exposes secret lookup for other services.
type Resolver interface {
	ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error)
	ResolveSecretsWithACL(ctx context.Context, accountID string, names []string, caller CallerService) (map[string]string, error)
}

// Service manages account secrets.
// Uses SandboxedServiceEngine for capability-based access control.
type Service struct {
	*framework.SandboxedServiceEngine // Provides: ValidateAccount, Manifest, Descriptor, Logger, sandbox capabilities
	store                             Store
	cipher                            Cipher
}

// Option configures the secrets service.
type Option func(*Service)

// WithCipher supplies a custom cipher used to encrypt/decrypt stored values.
func WithCipher(c Cipher) Option {
	return func(s *Service) { s.cipher = c }
}

// New creates a secrets service.
func New(accounts AccountChecker, store Store, log *logger.Logger, opts ...Option) *Service {
	svc := &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "secrets",
				Domain:       "secrets",
				Description:  "Secret storage and resolution",
				DependsOn:    []string{"store", "svc-accounts"},
				Capabilities: []string{"secrets"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapServiceCall,
			},
			StorageQuota: 50 * 1024 * 1024, // 50MB for secrets
		}),
		store:  store,
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

// CreateOptions configures secret creation.
type CreateOptions struct {
	ACL ACL // Access control flags for service access
}

// Create stores a new secret value.
func (s *Service) Create(ctx context.Context, accountID, name, value string) (Metadata, error) {
	return s.CreateWithOptions(ctx, accountID, name, value, CreateOptions{})
}

// CreateWithOptions stores a new secret value with ACL settings.
// Aligned with SecretsVault.cs contract ACL support.
func (s *Service) CreateWithOptions(ctx context.Context, accountID, name, value string, opts CreateOptions) (Metadata, error) {
	// Use ServiceEngine's ValidateAccount (trims + validates existence)
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return Metadata{}, err
	}
	if err := validateName(name); err != nil {
		return Metadata{}, err
	}
	if value == "" {
		return Metadata{}, core.RequiredError("value")
	}

	ciphertext, err := s.encrypt(value)
	if err != nil {
		return Metadata{}, err
	}

	record := Secret{
		ID:        uuid.NewString(),
		AccountID: accountID,
		Name:      name,
		Value:     ciphertext,
		ACL:       opts.ACL,
	}

	attrs := map[string]string{"account_id": accountID, "resource": "secret"}
	ctx, finish := s.StartObservation(ctx, attrs)
	stored, err := s.store.CreateSecret(ctx, record)
	if err == nil && stored.ID != "" {
		attrs["secret_id"] = stored.ID
	}
	finish(err)
	if err != nil {
		return Metadata{}, err
	}
	s.LogCreated("secret", stored.ID, accountID)
	s.IncrementCounter("secrets_created_total", map[string]string{"account_id": accountID})
	return stored.ToMetadata(), nil
}

// UpdateOptions configures secret update.
type UpdateOptions struct {
	ACL   *ACL    // If set, updates the ACL; nil keeps existing ACL
	Value *string // If set, updates the value; nil keeps existing value
}

// Update replaces the secret value.
func (s *Service) Update(ctx context.Context, accountID, name, value string) (Metadata, error) {
	return s.UpdateWithOptions(ctx, accountID, name, UpdateOptions{Value: &value})
}

// UpdateWithOptions updates a secret with optional ACL and value changes.
// Aligned with SecretsVault.cs contract ACL support.
func (s *Service) UpdateWithOptions(ctx context.Context, accountID, name string, opts UpdateOptions) (Metadata, error) {
	// Use ServiceEngine's ValidateAccount
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return Metadata{}, err
	}
	if err := validateName(name); err != nil {
		return Metadata{}, err
	}

	// Get existing secret to preserve fields not being updated
	existing, err := s.store.GetSecret(ctx, accountID, name)
	if err != nil {
		return Metadata{}, err
	}

	record := Secret{
		ID:        existing.ID,
		AccountID: accountID,
		Name:      name,
		Value:     existing.Value,
		ACL:       existing.ACL,
		Version:   existing.Version,
	}

	// Update value if provided
	if opts.Value != nil {
		if *opts.Value == "" {
			return Metadata{}, core.RequiredError("value")
		}
		ciphertext, err := s.encrypt(*opts.Value)
		if err != nil {
			return Metadata{}, err
		}
		record.Value = ciphertext
	}

	// Update ACL if provided
	if opts.ACL != nil {
		record.ACL = *opts.ACL
	}

	attrs := map[string]string{"account_id": accountID, "resource": "secret", "secret_id": record.ID}
	ctx, finish := s.StartObservation(ctx, attrs)
	updated, err := s.store.UpdateSecret(ctx, record)
	finish(err)
	if err != nil {
		return Metadata{}, err
	}
	s.LogUpdated("secret", record.ID, accountID)
	s.IncrementCounter("secrets_updated_total", map[string]string{"account_id": accountID})
	return updated.ToMetadata(), nil
}

// Get retrieves a secret including its decrypted value.
func (s *Service) Get(ctx context.Context, accountID, name string) (Secret, error) {
	// Use ServiceEngine's ValidateAccount
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return Secret{}, err
	}
	if err := validateName(name); err != nil {
		return Secret{}, err
	}

	record, err := s.store.GetSecret(ctx, accountID, name)
	if err != nil {
		return Secret{}, err
	}

	plaintext, err := s.decrypt(record.Value)
	if err != nil {
		return Secret{}, err
	}
	record.Value = plaintext
	return record, nil
}

// List returns metadata for all secrets on the account.
func (s *Service) List(ctx context.Context, accountID string) ([]Metadata, error) {
	// Use ServiceEngine's ValidateAccount
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

	records, err := s.store.ListSecrets(ctx, accountID)
	if err != nil {
		return nil, err
	}

	result := make([]Metadata, 0, len(records))
	for _, rec := range records {
		result = append(result, rec.ToMetadata())
	}
	return result, nil
}

// Delete removes a secret.
func (s *Service) Delete(ctx context.Context, accountID, name string) error {
	// Use ServiceEngine's ValidateAccount
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if err := validateName(name); err != nil {
		return err
	}
	attrs := map[string]string{"account_id": accountID, "resource": "secret"}
	ctx, finish := s.StartObservation(ctx, attrs)
	if err := s.store.DeleteSecret(ctx, accountID, name); err != nil {
		finish(err)
		return err
	}
	finish(nil)
	s.LogDeleted("secret", name, accountID)
	s.IncrementCounter("secrets_deleted_total", map[string]string{"account_id": accountID})
	return nil
}

// ResolveSecrets returns a map of secret name -> plaintext value.
// Note: This method bypasses ACL checks for backward compatibility.
// Use ResolveSecretsWithACL for ACL-enforced access.
func (s *Service) ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	// Use ServiceEngine's ValidateAccount
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

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
	if len(resolved) > 0 {
		s.AddCounter("secrets_resolved_total", map[string]string{"account_id": accountID}, float64(len(resolved)))
	}
	return resolved, nil
}

// ResolveSecretsWithACL returns secrets that the caller service has access to.
// Aligned with SecretsVault.cs contract ACL enforcement.
func (s *Service) ResolveSecretsWithACL(ctx context.Context, accountID string, names []string, caller CallerService) (map[string]string, error) {
	// Use ServiceEngine's ValidateAccount
	accountID, err := s.ValidateAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

	requiredACL := callerToACL(caller)
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

		// Check ACL - if ACL is 0 (ACLNone), only account owner can access (via ResolveSecrets)
		// If ACL has the required flag, allow access
		if requiredACL != 0 && !record.ACL.HasAccess(requiredACL) {
			return nil, fmt.Errorf("secret %q: access denied for %s service (ACL: %d, required: %d)",
				name, caller, record.ACL, requiredACL)
		}

		plaintext, err := s.decrypt(record.Value)
		if err != nil {
			return nil, err
		}
		resolved[name] = plaintext
	}
	if len(resolved) > 0 {
		s.AddCounter("secrets_resolved_total", map[string]string{"account_id": accountID, "caller": string(caller)}, float64(len(resolved)))
	}
	return resolved, nil
}

// callerToACL maps a caller service to its required ACL flag.
func callerToACL(caller CallerService) ACL {
	switch caller {
	case CallerOracle:
		return ACLOracleAccess
	case CallerAutomation:
		return ACLAutomationAccess
	case CallerFunctions:
		return ACLFunctionAccess
	case CallerJAM:
		return ACLJAMAccess
	default:
		return ACLNone
	}
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
		return core.RequiredError("name")
	}
	if strings.Contains(trimmed, "|") {
		return fmt.Errorf("name cannot contain '|'")
	}
	return nil
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetSecrets handles GET /secrets - list all secrets metadata for an account.
func (s *Service) HTTPGetSecrets(ctx context.Context, req core.APIRequest) (any, error) {
	return s.List(ctx, req.AccountID)
}

// HTTPPostSecrets handles POST /secrets - create a new secret.
func (s *Service) HTTPPostSecrets(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	value, _ := req.Body["value"].(string)

	// Parse ACL if provided
	var opts CreateOptions
	if aclVal, ok := req.Body["acl"].(float64); ok {
		opts.ACL = ACL(int(aclVal))
	}

	return s.CreateWithOptions(ctx, req.AccountID, name, value, opts)
}

// HTTPGetSecretsByName handles GET /secrets/{name} - get a specific secret.
func (s *Service) HTTPGetSecretsByName(ctx context.Context, req core.APIRequest) (any, error) {
	name := req.PathParams["name"]
	secret, err := s.Get(ctx, req.AccountID, name)
	if err != nil {
		return nil, err
	}
	// Return metadata only (don't expose value via HTTP)
	return secret.ToMetadata(), nil
}

// HTTPPutSecretsByName handles PUT /secrets/{name} - update a secret.
func (s *Service) HTTPPutSecretsByName(ctx context.Context, req core.APIRequest) (any, error) {
	name := req.PathParams["name"]

	var opts UpdateOptions
	if value, ok := req.Body["value"].(string); ok {
		opts.Value = &value
	}
	if aclVal, ok := req.Body["acl"].(float64); ok {
		acl := ACL(int(aclVal))
		opts.ACL = &acl
	}

	return s.UpdateWithOptions(ctx, req.AccountID, name, opts)
}

// HTTPDeleteSecretsByName handles DELETE /secrets/{name} - delete a secret.
func (s *Service) HTTPDeleteSecretsByName(ctx context.Context, req core.APIRequest) (any, error) {
	name := req.PathParams["name"]
	if err := s.Delete(ctx, req.AccountID, name); err != nil {
		return nil, err
	}
	return map[string]string{"status": "deleted", "name": name}, nil
}
