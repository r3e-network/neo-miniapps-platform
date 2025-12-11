// Package neofeeds provides lifecycle management for the price feed aggregation service.
package neofeeds

import "context"

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the NeoFeeds service including chain push workers.
// Background workers are managed by BaseService.
func (s *Service) Start(ctx context.Context) error {
	return s.BaseService.Start(ctx)
}

// Stop stops the NeoFeeds service.
// Background workers are automatically stopped by BaseService.
func (s *Service) Stop() error {
	return s.BaseService.Stop()
}
