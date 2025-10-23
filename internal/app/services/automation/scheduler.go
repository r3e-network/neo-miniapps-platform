package automation

import (
	"context"
	"sync"
	"time"

	domain "github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Ensure Scheduler implements system.Service.
var _ system.Service = (*Scheduler)(nil)

// Scheduler polls the automation store and prepares jobs for execution.
// The dispatcher is responsible for forwarding jobs into the runtime.
type Scheduler struct {
	service  *Service
	log      *logger.Logger
	interval time.Duration

	mu         sync.Mutex
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	dispatcher JobDispatcher
}

// JobDispatcher consumes scheduled automation jobs.
type JobDispatcher interface {
	DispatchJob(ctx context.Context, job domain.Job) error
}

// JobDispatcherFunc adapts a function to the JobDispatcher interface.
type JobDispatcherFunc func(ctx context.Context, job domain.Job) error

func (f JobDispatcherFunc) DispatchJob(ctx context.Context, job domain.Job) error {
	if f == nil {
		return nil
	}
	return f(ctx, job)
}

// NewScheduler creates a lifecycle-managed automation scheduler.
func NewScheduler(service *Service, log *logger.Logger) *Scheduler {
	if log == nil {
		log = logger.NewDefault("automation-runner")
	}
	return &Scheduler{
		service:  service,
		log:      log,
		interval: 5 * time.Second,
	}
}

// WithDispatcher registers a job dispatcher invoked for enabled jobs.
func (s *Scheduler) WithDispatcher(dispatcher JobDispatcher) {
	s.mu.Lock()
	s.dispatcher = dispatcher
	s.mu.Unlock()
}

// Name returns the service identifier.
func (s *Scheduler) Name() string { return "automation-scheduler" }

// Start begins the background polling loop.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				s.tick(runCtx)
			}
		}
	}()

	s.log.Info("automation scheduler started")
	return nil
}

// Stop halts the polling loop.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	s.running = false
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.log.Info("automation scheduler stopped")
	return nil
}

func (s *Scheduler) tick(ctx context.Context) {
	if s.service == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	jobs, err := s.service.ListJobs(ctx, "")
	if err != nil {
		s.log.WithError(err).Warn("automation scheduler tick failed")
		return
	}

	s.mu.Lock()
	dispatcher := s.dispatcher
	s.mu.Unlock()

	if dispatcher == nil {
		return
	}

	now := time.Now()
	var wg sync.WaitGroup
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if !job.NextRun.IsZero() && job.NextRun.After(now) {
			continue
		}
		wg.Add(1)
		go func(job domain.Job) {
			defer wg.Done()
			if err := dispatcher.DispatchJob(ctx, job); err != nil {
				s.log.WithError(err).
					WithField("job_id", job.ID).
					WithField("function_id", job.FunctionID).
					Warn("dispatch automation job failed")
			}
		}(job)
	}
	wg.Wait()
}
