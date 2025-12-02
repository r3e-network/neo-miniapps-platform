package tee

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SecretManager provides high-level secret management with service isolation.
// It wraps the SecretVault and enforces access control policies.
type SecretManager struct {
	mu     sync.RWMutex
	vault  SecretVault
	engine Engine

	// Access control
	policies map[string]*SecretPolicy // service_id -> policy
	grants   map[string][]SecretGrant // target_service_id -> grants received
}

// SecretPolicy defines a service's secret access permissions.
type SecretPolicy struct {
	ServiceID       string   `json:"service_id"`
	AllowedPatterns []string `json:"allowed_patterns"` // Patterns this service can create/access
	MaxSecrets      int      `json:"max_secrets"`      // Maximum secrets per account
	CanGrantAccess  bool     `json:"can_grant_access"` // Can grant access to other services
}

// SecretGrant represents a cross-service secret access grant.
type SecretGrant struct {
	OwnerServiceID  string    `json:"owner_service_id"`
	TargetServiceID string    `json:"target_service_id"`
	AccountID       string    `json:"account_id"`
	SecretPattern   string    `json:"secret_pattern"` // Can be exact name or pattern
	GrantedAt       time.Time `json:"granted_at"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	GrantedBy       string    `json:"granted_by"` // User or system that created the grant
}

// SecretMetadata contains metadata about a stored secret.
type SecretMetadata struct {
	Name        string            `json:"name"`
	ServiceID   string            `json:"service_id"`
	AccountID   string            `json:"account_id"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Version     int               `json:"version"`
	Tags        map[string]string `json:"tags,omitempty"`
	AccessCount int64             `json:"access_count"`
}

// NewSecretManager creates a new secret manager.
func NewSecretManager(vault SecretVault) *SecretManager {
	return &SecretManager{
		vault:    vault,
		policies: make(map[string]*SecretPolicy),
		grants:   make(map[string][]SecretGrant),
	}
}

// RegisterPolicy registers a service's secret access policy.
func (m *SecretManager) RegisterPolicy(policy SecretPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ServiceID == "" {
		return fmt.Errorf("service_id required")
	}
	if policy.MaxSecrets <= 0 {
		policy.MaxSecrets = 100 // Default limit
	}

	m.policies[policy.ServiceID] = &policy
	return nil
}

// StoreSecret stores a secret for a service/account.
func (m *SecretManager) StoreSecret(ctx context.Context, serviceID, accountID, name string, value []byte, tags map[string]string) error {
	m.mu.RLock()
	policy, ok := m.policies[serviceID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("service %s not registered", serviceID)
	}

	// Validate name against allowed patterns
	if !m.matchesAnyPattern(name, policy.AllowedPatterns) {
		return fmt.Errorf("%w: service %s cannot create secret with name %s", ErrSecretAccessDenied, serviceID, name)
	}

	// Check secret count limit
	existing, err := m.vault.ListSecrets(ctx, serviceID, accountID)
	if err != nil {
		return fmt.Errorf("list secrets: %w", err)
	}
	if len(existing) >= policy.MaxSecrets {
		return fmt.Errorf("secret limit reached: %d/%d", len(existing), policy.MaxSecrets)
	}

	return m.vault.StoreSecret(ctx, serviceID, accountID, name, value)
}

// GetSecret retrieves a secret, checking access permissions.
func (m *SecretManager) GetSecret(ctx context.Context, serviceID, accountID, name string) ([]byte, error) {
	// First try direct access
	value, err := m.vault.GetSecret(ctx, serviceID, accountID, name)
	if err == nil {
		return value, nil
	}

	// Check if there's a grant from another service
	m.mu.RLock()
	grants := m.grants[serviceID]
	m.mu.RUnlock()

	for _, grant := range grants {
		if grant.AccountID != accountID {
			continue
		}
		if !grant.ExpiresAt.IsZero() && time.Now().After(grant.ExpiresAt) {
			continue
		}
		if m.matchesPattern(name, grant.SecretPattern) {
			return m.vault.GetSecret(ctx, grant.OwnerServiceID, accountID, name)
		}
	}

	return nil, fmt.Errorf("%w: %s cannot access secret %s", ErrSecretAccessDenied, serviceID, name)
}

// GetSecrets retrieves multiple secrets.
func (m *SecretManager) GetSecrets(ctx context.Context, serviceID, accountID string, names []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, name := range names {
		value, err := m.GetSecret(ctx, serviceID, accountID, name)
		if err != nil {
			return nil, err
		}
		result[name] = string(value)
	}

	return result, nil
}

// DeleteSecret removes a secret.
func (m *SecretManager) DeleteSecret(ctx context.Context, serviceID, accountID, name string) error {
	return m.vault.DeleteSecret(ctx, serviceID, accountID, name)
}

// ListSecrets lists secret names for a service/account.
func (m *SecretManager) ListSecrets(ctx context.Context, serviceID, accountID string) ([]string, error) {
	return m.vault.ListSecrets(ctx, serviceID, accountID)
}

// GrantAccess grants another service access to secrets.
func (m *SecretManager) GrantAccess(ctx context.Context, grant SecretGrant) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify owner service can grant access
	policy, ok := m.policies[grant.OwnerServiceID]
	if !ok {
		return fmt.Errorf("owner service %s not registered", grant.OwnerServiceID)
	}
	if !policy.CanGrantAccess {
		return fmt.Errorf("service %s cannot grant access", grant.OwnerServiceID)
	}

	// Verify target service is registered
	if _, ok := m.policies[grant.TargetServiceID]; !ok {
		return fmt.Errorf("target service %s not registered", grant.TargetServiceID)
	}

	grant.GrantedAt = time.Now()

	// Add to grants
	m.grants[grant.TargetServiceID] = append(m.grants[grant.TargetServiceID], grant)

	// Also update vault-level grant for direct access
	return m.vault.GrantAccess(ctx, grant.OwnerServiceID, grant.TargetServiceID, grant.AccountID, grant.SecretPattern)
}

// RevokeAccess revokes a previously granted access.
func (m *SecretManager) RevokeAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretPattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from grants
	grants := m.grants[targetServiceID]
	filtered := make([]SecretGrant, 0, len(grants))
	for _, g := range grants {
		if g.OwnerServiceID == ownerServiceID && g.AccountID == accountID && g.SecretPattern == secretPattern {
			continue
		}
		filtered = append(filtered, g)
	}
	m.grants[targetServiceID] = filtered

	return m.vault.RevokeAccess(ctx, ownerServiceID, targetServiceID, accountID, secretPattern)
}

// ListGrants lists all grants for a service.
func (m *SecretManager) ListGrants(ctx context.Context, serviceID string) ([]SecretGrant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Grants received by this service
	received := m.grants[serviceID]

	// Grants given by this service
	var given []SecretGrant
	for _, grants := range m.grants {
		for _, g := range grants {
			if g.OwnerServiceID == serviceID {
				given = append(given, g)
			}
		}
	}

	return append(received, given...), nil
}

func (m *SecretManager) matchesAnyPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if m.matchesPattern(name, pattern) {
			return true
		}
	}
	return false
}

func (m *SecretManager) matchesPattern(name, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == name {
		return true
	}
	// Simple prefix matching with *
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(name, prefix)
	}
	return false
}

// ServiceSecretResolver implements SecretResolver for a specific service.
// This is the interface services use to access secrets during TEE execution.
type ServiceSecretResolver struct {
	serviceID string
	manager   *SecretManager
}

// NewServiceSecretResolver creates a resolver scoped to a service.
func NewServiceSecretResolver(serviceID string, manager *SecretManager) *ServiceSecretResolver {
	return &ServiceSecretResolver{
		serviceID: serviceID,
		manager:   manager,
	}
}

func (r *ServiceSecretResolver) ServiceID() string {
	return r.serviceID
}

func (r *ServiceSecretResolver) Resolve(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	return r.manager.GetSecrets(ctx, r.serviceID, accountID, names)
}

// DefaultServicePolicies returns default policies for known services.
func DefaultServicePolicies() []SecretPolicy {
	return []SecretPolicy{
		{
			ServiceID:       "secrets",
			AllowedPatterns: []string{"*"}, // Secrets service can manage all secrets
			MaxSecrets:      1000,
			CanGrantAccess:  true,
		},
		{
			ServiceID:       "functions",
			AllowedPatterns: []string{"fn_*", "api_*", "db_*"},
			MaxSecrets:      100,
			CanGrantAccess:  false,
		},
		{
			ServiceID:       "oracle",
			AllowedPatterns: []string{"oracle_*", "api_*"},
			MaxSecrets:      50,
			CanGrantAccess:  false,
		},
		{
			ServiceID:       "mixer",
			AllowedPatterns: []string{"mixer_*", "wallet_*"},
			MaxSecrets:      50,
			CanGrantAccess:  false,
		},
		{
			ServiceID:       "datalink",
			AllowedPatterns: []string{"datalink_*", "webhook_*"},
			MaxSecrets:      50,
			CanGrantAccess:  false,
		},
		{
			ServiceID:       "vrf",
			AllowedPatterns: []string{"vrf_*"},
			MaxSecrets:      20,
			CanGrantAccess:  false,
		},
		{
			ServiceID:       "gasbank",
			AllowedPatterns: []string{"gasbank_*", "wallet_*"},
			MaxSecrets:      30,
			CanGrantAccess:  false,
		},
	}
}
