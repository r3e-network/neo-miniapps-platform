// Package datafeeds provides lifecycle management for the price feed aggregation service.
package datafeedsmarble

import "context"

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the DataFeeds service including the chain push loop.
func (s *Service) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	if s.enableChainPush && s.teeFulfiller != nil && s.dataFeedsHash != "" {
		go s.runChainPushLoop(ctx)
	}

	return nil
}

// Stop stops the DataFeeds service.
func (s *Service) Stop() error {
	close(s.stopCh)
	return s.Service.Stop()
}
