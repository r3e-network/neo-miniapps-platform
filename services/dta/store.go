// Package dta provides Data Trust Authority service.
package dta

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages DTA service data using Supabase PostgreSQL.
type Store struct {
	mu           sync.RWMutex
	certificates *base.SupabaseStore[*Certificate]
	ready        bool
}

// NewStore creates a new DTA store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		certificates: base.NewSupabaseStore[*Certificate](config, "oracle_requests"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		certificates: base.NewSupabaseStore[*Certificate](config, "oracle_requests"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.certificates.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize oracle_requests store: %w", err)
	}

	s.ready = true
	return nil
}

// Close closes the store.
func (s *Store) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}

// Shutdown shuts down the store (implements base.Component interface).
func (s *Store) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.certificates.Close(ctx); err != nil {
		return fmt.Errorf("close oracle_requests store: %w", err)
	}

	s.ready = false
	return nil
}

// Health checks store health.
func (s *Store) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.certificates.Health(ctx)
}

// CreateCertificate creates a new certificate.
func (s *Store) CreateCertificate(ctx context.Context, cert *Certificate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	cert.GenerateID()
	cert.SetTimestamps()
	if cert.Status == "" {
		cert.Status = CertStatusValid
	}
	return s.certificates.Create(ctx, cert)
}

// GetCertificate gets a certificate by ID.
func (s *Store) GetCertificate(ctx context.Context, id string) (*Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.certificates.Get(ctx, id)
}

// ListCertificates lists all certificates.
func (s *Store) ListCertificates(ctx context.Context) ([]*Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.certificates.List(ctx)
}
