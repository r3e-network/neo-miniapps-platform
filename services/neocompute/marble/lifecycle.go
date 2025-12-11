// Package neocompute provides lifecycle management for the neocompute service.
package neocomputemarble

import "context"

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the NeoCompute service.
// Background workers (cleanup) are managed by BaseService.
func (s *Service) Start(ctx context.Context) error {
	return s.BaseService.Start(ctx)
}

// Stop stops the NeoCompute service.
// Background workers are automatically stopped by BaseService.
func (s *Service) Stop() error {
	return s.BaseService.Stop()
}
