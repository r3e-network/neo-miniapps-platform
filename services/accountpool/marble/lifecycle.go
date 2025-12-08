// Package accountpool provides lifecycle management for the account pool service.
package accountpoolmarble

import (
	"context"
	"fmt"
)

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the account pool service and background workers.
func (s *Service) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	// Initialize pool accounts
	if err := s.initializePool(ctx); err != nil {
		return fmt.Errorf("initialize pool: %w", err)
	}

	// Start background workers
	go s.runAccountRotation(ctx)
	go s.runLockCleanup(ctx)

	return nil
}

// Stop stops the account pool service.
func (s *Service) Stop() error {
	close(s.stopCh)
	return s.Service.Stop()
}
