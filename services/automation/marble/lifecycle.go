// Package automation provides lifecycle management for the task automation service.
package automationmarble

import (
	"context"
	"time"
)

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the automation scheduler.
func (s *Service) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	// Hydrate scheduler cache from DB (enabled triggers)
	if triggers, err := s.repo.GetTriggers(ctx, ""); err == nil {
		s.scheduler.mu.Lock()
		for i := range triggers {
			t := triggers[i]
			if t.Enabled {
				s.scheduler.triggers[t.ID] = &t
			}
		}
		s.scheduler.mu.Unlock()
	}

	// Setup event listeners for on-chain triggers
	s.SetupEventTriggerListener()

	// Start background workers
	go s.runScheduler(ctx)
	go s.runChainTriggerChecker(ctx)

	return nil
}

// Stop stops the automation service.
func (s *Service) Stop() error {
	close(s.scheduler.stopCh)
	return s.Service.Stop()
}

// runScheduler handles time-based triggers (cron).
func (s *Service) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(SchedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.scheduler.stopCh:
			return
		case <-ticker.C:
			s.checkAndExecuteTriggers(ctx)
		}
	}
}

// runChainTriggerChecker handles on-chain triggers (price, threshold).
func (s *Service) runChainTriggerChecker(ctx context.Context) {
	ticker := time.NewTicker(ChainTriggerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.scheduler.stopCh:
			return
		case <-ticker.C:
			s.checkChainTriggers(ctx)
		}
	}
}
