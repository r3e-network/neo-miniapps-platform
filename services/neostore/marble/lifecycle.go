// Package neostoremarble provides lifecycle management for the neostore service.
package neostoremarble

import "context"

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the NeoStore service.
func (s *Service) Start(ctx context.Context) error {
	return s.BaseService.Start(ctx)
}

// Stop stops the NeoStore service.
func (s *Service) Stop() error {
	return s.BaseService.Stop()
}
