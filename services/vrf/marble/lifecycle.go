package vrfmarble

import (
	"context"
)

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the VRF service and background workers.
func (s *Service) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	// Hydrate pending requests from DB
	if s.repo != nil {
		if pending, err := s.repo.ListByStatus(ctx, StatusPending); err == nil {
			for i := range pending {
				req := vrfReqFromRecord(&pending[i])
				s.requests[req.RequestID] = req
				select {
				case s.pendingRequests <- req:
				default:
				}
			}
		}
	}

	// Start background workers
	go s.runEventListener(ctx)
	go s.runRequestFulfiller(ctx)

	return nil
}

// Stop stops the VRF service.
func (s *Service) Stop() error {
	close(s.stopCh)
	return s.Service.Stop()
}
