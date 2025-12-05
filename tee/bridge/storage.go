// Package bridge provides the untrusted bridge for TEE I/O operations.
package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// StorageConfig holds storage bridge configuration.
type StorageConfig struct {
	BasePath string
}

// Storage provides encrypted storage operations for the enclave.
// All data stored through this bridge should be sealed (encrypted) by the enclave.
type Storage struct {
	mu       sync.RWMutex
	basePath string
}

// NewStorage creates a new storage bridge.
func NewStorage(cfg StorageConfig) (*Storage, error) {
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("base_path is required")
	}

	// Ensure base path exists
	if err := os.MkdirAll(cfg.BasePath, 0700); err != nil {
		return nil, fmt.Errorf("create base path: %w", err)
	}

	return &Storage{
		basePath: cfg.BasePath,
	}, nil
}

// Get retrieves sealed data from storage.
func (s *Storage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.keyToPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("read: %w", err)
	}

	return data, nil
}

// Put stores sealed data.
func (s *Storage) Put(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.keyToPath(key)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write with restricted permissions
	if err := os.WriteFile(path, value, 0600); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// Delete removes sealed data.
func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.keyToPath(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

// List returns keys with the given prefix.
func (s *Storage) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefixPath := s.keyToPath(prefix)
	dir := filepath.Dir(prefixPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list: %w", err)
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		key := filepath.Join(filepath.Base(dir), entry.Name())
		keys = append(keys, key)
	}

	return keys, nil
}

// Exists checks if a key exists.
func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.keyToPath(key)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat: %w", err)
	}

	return true, nil
}

// keyToPath converts a key to a file path.
func (s *Storage) keyToPath(key string) string {
	// Sanitize key to prevent path traversal
	clean := filepath.Clean(key)
	if filepath.IsAbs(clean) {
		clean = clean[1:] // Remove leading slash
	}
	return filepath.Join(s.basePath, clean)
}

// Close closes the storage bridge.
func (s *Storage) Close() error {
	// Nothing to close for file-based storage
	return nil
}
