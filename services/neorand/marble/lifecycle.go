package neorand

import "context"

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the VRF service and background workers.
func (s *Service) Start(ctx context.Context) error {
	return s.BaseService.Start(ctx)
}

// Stop stops the VRF service.
func (s *Service) Stop() error {
	return s.BaseService.Stop()
}
