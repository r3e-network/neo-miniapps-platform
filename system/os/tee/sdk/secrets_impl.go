// Package sdk provides the Enclave SDK implementation.
package sdk

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"sync"
	"time"
)

// secretsManagerImpl implements SecretsManager interface.
type secretsManagerImpl struct {
	mu       sync.RWMutex
	secrets  map[string]*sealedSecret
	sealKey  []byte // Enclave sealing key (derived from MRENCLAVE)
	callerID string
}

// sealedSecret represents an encrypted secret in storage.
type sealedSecret struct {
	ID            string
	Name          string
	EncryptedData []byte
	Type          SecretType
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     *time.Time
	Metadata      map[string]string
	Permissions   []Permission
	Nonce         []byte
}

// NewSecretsManager creates a new secrets manager instance.
func NewSecretsManager(sealKey []byte, callerID string) SecretsManager {
	return &secretsManagerImpl{
		secrets:  make(map[string]*sealedSecret),
		sealKey:  sealKey,
		callerID: callerID,
	}
}

func (m *secretsManagerImpl) Add(ctx context.Context, req *AddSecretRequest) (*AddSecretResponse, error) {
	if req.Name == "" {
		return nil, errors.New("secret name is required")
	}
	if len(req.Value) == 0 {
		return nil, errors.New("secret value is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if secret with same name exists
	for _, s := range m.secrets {
		if s.Name == req.Name {
			return nil, ErrSecretExists
		}
	}

	// Generate unique ID
	id := generateSecretID(req.Name)

	// Encrypt the secret value
	encryptedData, nonce, err := m.seal(req.Value)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	sealed := &sealedSecret{
		ID:            id,
		Name:          req.Name,
		EncryptedData: encryptedData,
		Type:          req.Type,
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     req.ExpiresAt,
		Metadata:      req.Metadata,
		Permissions:   req.Permissions,
		Nonce:         nonce,
	}

	m.secrets[id] = sealed

	return &AddSecretResponse{
		SecretID:  id,
		Version:   1,
		CreatedAt: now,
	}, nil
}

func (m *secretsManagerImpl) Update(ctx context.Context, req *UpdateSecretRequest) (*UpdateSecretResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sealed, exists := m.secrets[req.SecretID]
	if !exists {
		return nil, ErrSecretNotFound
	}

	// Check permission
	if !m.hasPermission(sealed, "write") {
		return nil, ErrPermissionDenied
	}

	// Update value if provided
	if len(req.Value) > 0 {
		encryptedData, nonce, err := m.seal(req.Value)
		if err != nil {
			return nil, err
		}
		sealed.EncryptedData = encryptedData
		sealed.Nonce = nonce
	}

	// Update metadata if provided
	if req.Metadata != nil {
		sealed.Metadata = req.Metadata
	}

	// Update permissions if provided
	if req.Permissions != nil {
		sealed.Permissions = req.Permissions
	}

	// Update expiration if provided
	if req.ExpiresAt != nil {
		sealed.ExpiresAt = req.ExpiresAt
	}

	sealed.Version++
	sealed.UpdatedAt = time.Now()

	return &UpdateSecretResponse{
		SecretID:  req.SecretID,
		Version:   sealed.Version,
		UpdatedAt: sealed.UpdatedAt,
	}, nil
}

func (m *secretsManagerImpl) Delete(ctx context.Context, req *DeleteSecretRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sealed, exists := m.secrets[req.SecretID]
	if !exists {
		return ErrSecretNotFound
	}

	// Check permission
	if !m.hasPermission(sealed, "delete") {
		return ErrPermissionDenied
	}

	delete(m.secrets, req.SecretID)
	return nil
}

func (m *secretsManagerImpl) Find(ctx context.Context, req *FindSecretRequest) (*FindSecretResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Secret
	var pattern *regexp.Regexp
	var err error

	if req.NamePattern != "" {
		pattern, err = regexp.Compile(req.NamePattern)
		if err != nil {
			return nil, errors.New("invalid name pattern")
		}
	}

	for _, sealed := range m.secrets {
		// Check permission
		if !m.hasPermission(sealed, "read") {
			continue
		}

		// Filter by name pattern
		if pattern != nil && !pattern.MatchString(sealed.Name) {
			continue
		}

		// Filter by type
		if req.Type != "" && sealed.Type != req.Type {
			continue
		}

		// Filter by metadata
		if req.Metadata != nil {
			match := true
			for k, v := range req.Metadata {
				if sealed.Metadata[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// Decrypt and add to results
		secret, err := m.unsealSecret(sealed)
		if err != nil {
			continue
		}
		results = append(results, secret)
	}

	// Apply pagination
	total := len(results)
	if req.Offset > 0 && req.Offset < len(results) {
		results = results[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(results) {
		results = results[:req.Limit]
	}

	return &FindSecretResponse{
		Secrets:    results,
		TotalCount: total,
	}, nil
}

func (m *secretsManagerImpl) Get(ctx context.Context, secretID string) (*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sealed, exists := m.secrets[secretID]
	if !exists {
		return nil, ErrSecretNotFound
	}

	// Check permission
	if !m.hasPermission(sealed, "read") {
		return nil, ErrPermissionDenied
	}

	// Check expiration
	if sealed.ExpiresAt != nil && time.Now().After(*sealed.ExpiresAt) {
		return nil, errors.New("secret has expired")
	}

	return m.unsealSecret(sealed)
}

func (m *secretsManagerImpl) List(ctx context.Context, req *ListSecretsRequest) (*ListSecretsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Secret
	for _, sealed := range m.secrets {
		// Check permission
		if !m.hasPermission(sealed, "read") {
			continue
		}

		secret, err := m.unsealSecret(sealed)
		if err != nil {
			continue
		}
		results = append(results, secret)
	}

	// Apply pagination
	total := len(results)
	if req.Offset > 0 && req.Offset < len(results) {
		results = results[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(results) {
		results = results[:req.Limit]
	}

	return &ListSecretsResponse{
		Secrets:    results,
		TotalCount: total,
	}, nil
}

func (m *secretsManagerImpl) Exists(ctx context.Context, secretID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sealed, exists := m.secrets[secretID]
	if !exists {
		return false, nil
	}

	// Check permission
	if !m.hasPermission(sealed, "read") {
		return false, ErrPermissionDenied
	}

	return true, nil
}

// seal encrypts data using the enclave sealing key.
func (m *secretsManagerImpl) seal(data []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(m.sealKey)
	if err != nil {
		return nil, nil, ErrEnclaveSealFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, ErrEnclaveSealFailed
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, ErrEnclaveSealFailed
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)
	return ciphertext, nonce, nil
}

// unseal decrypts data using the enclave sealing key.
func (m *secretsManagerImpl) unseal(ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.sealKey)
	if err != nil {
		return nil, ErrEnclaveUnsealFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrEnclaveUnsealFailed
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrEnclaveUnsealFailed
	}

	return plaintext, nil
}

// unsealSecret decrypts a sealed secret.
func (m *secretsManagerImpl) unsealSecret(sealed *sealedSecret) (*Secret, error) {
	value, err := m.unseal(sealed.EncryptedData, sealed.Nonce)
	if err != nil {
		return nil, err
	}

	return &Secret{
		ID:          sealed.ID,
		Name:        sealed.Name,
		Value:       value,
		Type:        sealed.Type,
		Version:     sealed.Version,
		CreatedAt:   sealed.CreatedAt,
		UpdatedAt:   sealed.UpdatedAt,
		ExpiresAt:   sealed.ExpiresAt,
		Metadata:    sealed.Metadata,
		Permissions: sealed.Permissions,
	}, nil
}

// hasPermission checks if the caller has the specified permission.
func (m *secretsManagerImpl) hasPermission(sealed *sealedSecret, action string) bool {
	// If no permissions defined, allow all
	if len(sealed.Permissions) == 0 {
		return true
	}

	for _, perm := range sealed.Permissions {
		if perm.Resource == m.callerID || perm.Resource == "*" {
			for _, a := range perm.Actions {
				if a == action || a == "*" {
					return true
				}
			}
		}
	}

	return false
}

// generateSecretID generates a unique secret ID.
func generateSecretID(name string) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte(time.Now().String()))
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	h.Write(randBytes)
	return hex.EncodeToString(h.Sum(nil))[:32]
}
