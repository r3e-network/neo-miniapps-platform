// Package vault provides secure secret storage inside the TEE enclave.
package vault

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/tee/enclave"
	"github.com/R3E-Network/service_layer/tee/types"
)

// Storage is the interface for persistent secret storage.
type Storage interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// Config holds vault configuration.
type Config struct {
	Runtime enclave.Runtime
	Storage Storage
}

// Vault implements types.SecureVault.
type Vault struct {
	mu      sync.RWMutex
	runtime enclave.Runtime
	storage Storage
	cache   map[string][]byte // Encrypted cache
}

// New creates a new Vault.
func New(cfg Config) (*Vault, error) {
	if cfg.Runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}

	return &Vault{
		runtime: cfg.Runtime,
		storage: cfg.Storage,
		cache:   make(map[string][]byte),
	}, nil
}

// makeKey creates a storage key from namespace and name.
func makeKey(namespace, name string) string {
	return namespace + "/" + name
}

// Store encrypts and stores a secret.
func (v *Vault) Store(ctx context.Context, namespace, name string, value []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Seal the secret
	sealed, err := v.runtime.Seal(value)
	if err != nil {
		return fmt.Errorf("seal secret: %w", err)
	}

	key := makeKey(namespace, name)

	// Store in persistent storage if available
	if v.storage != nil {
		if err := v.storage.Put(ctx, key, sealed); err != nil {
			return fmt.Errorf("store secret: %w", err)
		}
	}

	// Update cache
	v.cache[key] = sealed

	return nil
}

// Use executes a function with access to a secret.
// The secret is ONLY available inside the callback and is zeroed after.
func (v *Vault) Use(ctx context.Context, namespace, name string, fn types.SecretConsumer) error {
	v.mu.RLock()
	key := makeKey(namespace, name)
	sealed, ok := v.cache[key]
	v.mu.RUnlock()

	// Try to load from storage if not in cache
	if !ok && v.storage != nil {
		var err error
		sealed, err = v.storage.Get(ctx, key)
		if err != nil {
			return types.ErrSecretNotFound
		}

		// Update cache
		v.mu.Lock()
		v.cache[key] = sealed
		v.mu.Unlock()
	}

	if sealed == nil {
		return types.ErrSecretNotFound
	}

	// Unseal inside enclave
	plaintext, err := v.runtime.Unseal(sealed)
	if err != nil {
		return fmt.Errorf("unseal secret: %w", err)
	}

	// Execute consumer
	fnErr := fn(plaintext)

	// CRITICAL: Zero the plaintext immediately after use
	enclave.ZeroBytes(plaintext)

	return fnErr
}

// UseMultiple executes a function with access to multiple secrets.
func (v *Vault) UseMultiple(ctx context.Context, refs []types.SecretRef, fn types.MultiSecretConsumer) error {
	secrets := make(map[string][]byte)
	var plaintexts [][]byte

	// Collect all secrets
	for _, ref := range refs {
		err := v.Use(ctx, ref.Namespace, ref.Name, func(secret []byte) error {
			// Copy the secret
			cp := make([]byte, len(secret))
			copy(cp, secret)

			alias := ref.Alias
			if alias == "" {
				alias = ref.Name
			}
			secrets[alias] = cp
			plaintexts = append(plaintexts, cp)
			return nil
		})
		if err != nil {
			// Zero any secrets we've collected
			for _, pt := range plaintexts {
				enclave.ZeroBytes(pt)
			}
			return err
		}
	}

	// Execute consumer
	fnErr := fn(secrets)

	// CRITICAL: Zero all secrets
	for _, pt := range plaintexts {
		enclave.ZeroBytes(pt)
	}

	return fnErr
}

// Delete removes a secret.
func (v *Vault) Delete(ctx context.Context, namespace, name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	key := makeKey(namespace, name)

	// Delete from storage
	if v.storage != nil {
		if err := v.storage.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete secret: %w", err)
		}
	}

	// Remove from cache
	delete(v.cache, key)

	return nil
}

// List returns secret names in a namespace.
func (v *Vault) List(ctx context.Context, namespace string) ([]string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	prefix := namespace + "/"

	if v.storage != nil {
		return v.storage.List(ctx, prefix)
	}

	// List from cache
	var names []string
	for key := range v.cache {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			names = append(names, key[len(prefix):])
		}
	}
	return names, nil
}

// Exists checks if a secret exists.
func (v *Vault) Exists(ctx context.Context, namespace, name string) (bool, error) {
	v.mu.RLock()
	key := makeKey(namespace, name)
	_, ok := v.cache[key]
	v.mu.RUnlock()

	if ok {
		return true, nil
	}

	if v.storage != nil {
		data, err := v.storage.Get(ctx, key)
		return data != nil && err == nil, nil
	}

	return false, nil
}
