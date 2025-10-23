package automation

import (
	"context"
	"time"

	domain "github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// FunctionRunner executes functions referenced by automation jobs.
type FunctionRunner interface {
	Execute(ctx context.Context, functionID string, payload map[string]any) error
}

// FunctionRunnerFunc adapts a function to the FunctionRunner interface.
type FunctionRunnerFunc func(ctx context.Context, functionID string, payload map[string]any) error

func (f FunctionRunnerFunc) Execute(ctx context.Context, functionID string, payload map[string]any) error {
	if f == nil {
		return nil
	}
	return f(ctx, functionID, payload)
}

// FunctionDispatcher dispatches automation jobs by invoking the provided runner.
type FunctionDispatcher struct {
	runner  FunctionRunner
	service *Service
	log     *logger.Logger
	nextFn  func(domain.Job) time.Time
}

func NewFunctionDispatcher(runner FunctionRunner, svc *Service, log *logger.Logger) *FunctionDispatcher {
	if log == nil {
		log = logger.NewDefault("automation-dispatcher")
	}
	if runner == nil {
		log.Warn("no function runner configured; automation jobs will be skipped")
	}
	return &FunctionDispatcher{
		runner:  runner,
		service: svc,
		log:     log,
		nextFn: func(job domain.Job) time.Time {
			return time.Now().Add(time.Minute)
		},
	}
}

func (d *FunctionDispatcher) DispatchJob(ctx context.Context, job domain.Job) error {
	if d.runner == nil {
		return nil
	}
	payload := map[string]any{"automation_job": job.ID}
	start := time.Now()
	if err := d.runner.Execute(ctx, job.FunctionID, payload); err != nil {
		d.log.WithError(err).
			WithField("job_id", job.ID).
			WithField("function_id", job.FunctionID).
			Warn("automation job execution failed")
		metrics.RecordAutomationExecution(job.ID, time.Since(start), false)
		return err
	}
	metrics.RecordAutomationExecution(job.ID, time.Since(start), true)

	if d.service != nil {
		runAt := time.Now()
		next := d.nextFn(job)
		if _, err := d.service.RecordExecution(ctx, job.ID, runAt, next); err != nil {
			d.log.WithError(err).
				WithField("job_id", job.ID).
				Warn("record automation job execution failed")
		}
	}
	return nil
}
