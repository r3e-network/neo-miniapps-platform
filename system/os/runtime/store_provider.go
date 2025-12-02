// Package pkg provides the StoreProvider implementation.
package pkg

import (
	"context"
	"database/sql"
	"errors"
)

// storeProvider implements StoreProvider interface.
// It provides generic database access to service packages.
type storeProvider struct {
	db *sql.DB
}

// StoreProviderConfig contains configuration for creating a StoreProvider.
type StoreProviderConfig struct {
	// Database is the generic database connection for service packages.
	Database *sql.DB
}

// NewStoreProvider creates a StoreProvider from the given configuration.
func NewStoreProvider(cfg StoreProviderConfig) StoreProvider {
	return &storeProvider{
		db: cfg.Database,
	}
}

// Database returns the underlying database connection.
// Service packages use this to create their own typed stores.
func (s *storeProvider) Database() any {
	return s.db
}

// AccountExists checks if an account exists by ID.
// Returns nil if account exists, error otherwise.
func (s *storeProvider) AccountExists(ctx context.Context, accountID string) error {
	if s.db == nil {
		return errors.New("database not available")
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM app_accounts WHERE id = $1)
	`, accountID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("account not found")
	}
	return nil
}

// AccountTenant returns the tenant for an account (empty if none).
func (s *storeProvider) AccountTenant(ctx context.Context, accountID string) string {
	if s.db == nil {
		return ""
	}
	var tenant sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT tenant FROM app_accounts WHERE id = $1
	`, accountID).Scan(&tenant)
	if tenant.Valid {
		return tenant.String
	}
	return ""
}

// NilStoreProvider returns a StoreProvider with nil database.
// This is useful for testing or when database is not yet available.
func NilStoreProvider() StoreProvider {
	return &storeProvider{}
}
