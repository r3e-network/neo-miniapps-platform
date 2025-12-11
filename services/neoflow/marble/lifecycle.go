// Package neoflow provides lifecycle management for the task neoflow service.
package neoflowmarble

import (
	"context"
	"os"
	"time"
)

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the neoflow scheduler.
func (s *Service) Start(ctx context.Context) error {
	if err := s.BaseService.Start(ctx); err != nil {
		return err
	}

	stopCh := s.ensureSchedulerStopCh()

	s.hydrateSchedulerCache(ctx)

	// Setup event listeners for on-chain triggers
	s.SetupEventTriggerListener()

	// Start background workers
	go s.runScheduler(ctx, stopCh)
	go s.runChainTriggerChecker(ctx, stopCh)

	return nil
}

// Stop stops the neoflow service.
func (s *Service) Stop() error {
	s.stopSchedulerWorkers()
	return s.BaseService.Stop()
}

func (s *Service) ensureSchedulerStopCh() chan struct{} {
	s.scheduler.mu.Lock()
	defer s.scheduler.mu.Unlock()

	if s.scheduler.stopCh == nil {
		s.scheduler.stopCh = make(chan struct{})
	}
	return s.scheduler.stopCh
}

func (s *Service) stopSchedulerWorkers() {
	s.scheduler.mu.Lock()
	if s.scheduler.stopCh != nil {
		close(s.scheduler.stopCh)
		s.scheduler.stopCh = nil
	}
	s.scheduler.mu.Unlock()
}

func (s *Service) hydrateSchedulerCache(ctx context.Context) {
	if s.repo == nil {
		return
	}

	userID := s.schedulerHydrationUserID()
	if userID == "" {
		return
	}

	triggers, err := s.repo.GetTriggers(ctx, userID)
	if err != nil {
		return
	}

	s.scheduler.mu.Lock()
	for i := range triggers {
		t := triggers[i]
		if t.Enabled {
			s.scheduler.triggers[t.ID] = &t
		} else {
			delete(s.scheduler.triggers, t.ID)
		}
	}
	s.scheduler.mu.Unlock()
}

func (s *Service) schedulerHydrationUserID() string {
	if s == nil {
		return ""
	}

	if marbleInstance := s.Marble(); marbleInstance != nil {
		if value, ok := marbleInstance.Secret("NEOFLOW_SCHEDULER_USER_ID"); ok && len(value) > 0 {
			return string(value)
		}
	}

	return os.Getenv("NEOFLOW_SCHEDULER_USER_ID")
}

// runScheduler handles time-based triggers (cron).
func (s *Service) runScheduler(ctx context.Context, stopCh <-chan struct{}) {
	ticker := time.NewTicker(SchedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-ticker.C:
			s.checkAndExecuteTriggers(ctx)
		}
	}
}

// runChainTriggerChecker handles on-chain triggers (price, threshold).
func (s *Service) runChainTriggerChecker(ctx context.Context, stopCh <-chan struct{}) {
	ticker := time.NewTicker(ChainTriggerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-ticker.C:
			s.checkChainTriggers(ctx)
		}
	}
}
