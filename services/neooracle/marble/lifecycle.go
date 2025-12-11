// Package oraclemarble provides lifecycle management for the neooracle service.
package neooracle

import (
	"context"
)

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the NeoOracle service.
func (s *Service) Start(ctx context.Context) error {
	return s.BaseService.Start(ctx)
}

// Stop stops the NeoOracle service.
func (s *Service) Stop() error {
	return s.BaseService.Stop()
}
