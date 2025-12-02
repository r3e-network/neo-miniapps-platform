package automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Ensure Scheduler implements lifecycle contract.
var _ interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
	Ready(context.Context) error
} = (*Scheduler)(nil)

// Scheduler polls the automation store and prepares jobs for execution.
// The dispatcher is responsible for forwarding jobs into the runtime.
type Scheduler struct {
	framework.ServiceBase
	service  *Service
	log      *logger.Logger
	interval time.Duration

	mu         sync.Mutex
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	dispatcher JobDispatcher
	tracer     core.Tracer
}

// JobDispatcher consumes scheduled automation jobs.
type JobDispatcher interface {
	DispatchJob(ctx context.Context, job Job) error
}

// JobDispatcherFunc adapts a function to the JobDispatcher interface.
type JobDispatcherFunc func(ctx context.Context, job Job) error

func (f JobDispatcherFunc) DispatchJob(ctx context.Context, job Job) error {
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
	s := &Scheduler{
		service:  service,
		log:      log,
		interval: 1 * time.Second,
		tracer:   core.NoopTracer,
	}
	s.SetName(s.Name())
	return s
}

// WithDispatcher registers a job dispatcher invoked for enabled jobs.
func (s *Scheduler) WithDispatcher(dispatcher JobDispatcher) {
	s.mu.Lock()
	s.dispatcher = dispatcher
	tracer := s.tracer
	s.mu.Unlock()
	if traced, ok := dispatcher.(interface{ WithTracer(core.Tracer) }); ok {
		traced.WithTracer(tracer)
	}
}

// WithTracer configures a tracer for job dispatch spans.
func (s *Scheduler) WithTracer(tracer core.Tracer) {
	s.mu.Lock()
	if tracer == nil {
		s.tracer = core.NoopTracer
	} else {
		s.tracer = tracer
	}
	dispatcher := s.dispatcher
	s.mu.Unlock()
	if traced, ok := dispatcher.(interface{ WithTracer(core.Tracer) }); ok {
		traced.WithTracer(s.tracer)
	}
}

// Name returns the service identifier.
func (s *Scheduler) Name() string { return "automation-scheduler" }

// Domain reports the service domain for engine grouping.
func (s *Scheduler) Domain() string { return "automation" }

// Descriptor advertises the scheduler's architectural placement for orchestration.
func (s *Scheduler) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "runner-automation",
		Domain:       "automation",
		Layer:        core.LayerRunner,
		Capabilities: []string{"schedule", "dispatch"},
	}
}

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

	// Kick off an immediate tick so new jobs don't wait for the first interval.
	go s.tick(runCtx)

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
	s.MarkReady(true)
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
	s.MarkReady(false)
	return nil
}

// Ready reports readiness based on running state.
func (s *Scheduler) Ready(ctx context.Context) error {
	if err := s.ServiceBase.Ready(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return fmt.Errorf("automation scheduler not running")
	}
	return nil
}

func (s *Scheduler) tick(ctx context.Context) {
	if s.service == nil {
		return
	}
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	jobs, err := s.service.ListJobs(listCtx, "")
	cancel()
	if err != nil {
		s.log.WithError(err).Warn("automation scheduler tick failed")
		return
	}

	s.mu.Lock()
	dispatcher := s.dispatcher
	tracer := s.tracer
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
		go func(job Job) {
			defer wg.Done()
			attrs := map[string]string{"job_id": job.ID}
			if job.FunctionID != "" {
				attrs["function_id"] = job.FunctionID
			}
			spanCtx, finishSpan := tracer.StartSpan(ctx, "automation.dispatch", attrs)
			err := dispatcher.DispatchJob(spanCtx, job)
			if err != nil {
				s.log.WithError(err).
					WithField("job_id", job.ID).
					WithField("function_id", job.FunctionID).
					Warn("dispatch automation job failed")
			}
			finishSpan(err)
		}(job)
	}
	wg.Wait()
}
