// Package neoaccounts provides lifecycle management for the neoaccounts service.
package neoaccountsmarble

import "context"

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the neoaccounts service and background workers.
func (s *Service) Start(ctx context.Context) error {
	return s.BaseService.Start(ctx)
}

// Stop stops the neoaccounts service.
func (s *Service) Stop() error {
	return s.BaseService.Stop()
}
