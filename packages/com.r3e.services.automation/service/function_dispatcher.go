package automation

import (
	"context"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/metrics"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// FunctionRunner executes functions referenced by automation jobs.
type FunctionRunner interface {
	Execute(ctx context.Context, functionID string, payload map[string]any) (FunctionExecution, error)
}

// FunctionRunnerFunc adapts a function to the FunctionRunner interface.
type FunctionRunnerFunc func(ctx context.Context, functionID string, payload map[string]any) (FunctionExecution, error)

func (f FunctionRunnerFunc) Execute(ctx context.Context, functionID string, payload map[string]any) (FunctionExecution, error) {
	if f == nil {
		return FunctionExecution{}, nil
	}
	return f(ctx, functionID, payload)
}

// FunctionDispatcher dispatches automation jobs by invoking the provided runner.
type FunctionDispatcher struct {
	runner  FunctionRunner
	service *Service
	log     *logger.Logger
	tracer  core.Tracer
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
		tracer:  core.NoopTracer,
	}
}

// WithTracer configures tracing spans for the check/perform phases.
func (d *FunctionDispatcher) WithTracer(tracer core.Tracer) {
	if tracer == nil {
		d.tracer = core.NoopTracer
		return
	}
	d.tracer = tracer
}

func (d *FunctionDispatcher) DispatchJob(ctx context.Context, job Job) error {
	if d.runner == nil {
		return nil
	}
	runAt := time.Now().UTC()
	attrs := map[string]string{"job_id": job.ID}
	if job.FunctionID != "" {
		attrs["function_id"] = job.FunctionID
	}

	checkPayload := map[string]any{
		"automation_job": job.ID,
		"phase":          "check",
	}
	checkStart := time.Now()
	checkCtx, finishCheck := d.tracer.StartSpan(ctx, "automation.job.check", attrs)
	exec, err := d.runner.Execute(checkCtx, job.FunctionID, checkPayload)
	if err != nil {
		d.log.WithError(err).
			WithField("job_id", job.ID).
			WithField("function_id", job.FunctionID).
			Warn("automation job check failed")
		metrics.RecordAutomationExecution(job.ID+":check", time.Since(checkStart), false)
		finishCheck(err)
		return err
	}
	finishCheck(nil)
	metrics.RecordAutomationExecution(job.ID+":check", time.Since(checkStart), true)

	shouldPerform, performPayload := interpretCheck(exec.Output)
	if !shouldPerform {
		d.log.WithField("job_id", job.ID).
			WithField("function_id", job.FunctionID).
			Debug("automation job check skipped perform")
		if d.service != nil {
			if _, err := d.service.RecordExecution(ctx, job.ID, runAt); err != nil {
				d.log.WithError(err).
					WithField("job_id", job.ID).
					Warn("record automation job execution failed")
			}
		}
		return nil
	}

	performPayload["automation_job"] = job.ID
	performPayload["phase"] = "perform"

	performStart := time.Now()
	performCtx, finishPerform := d.tracer.StartSpan(ctx, "automation.job.perform", attrs)
	_, err = d.runner.Execute(performCtx, job.FunctionID, performPayload)
	if err != nil {
		d.log.WithError(err).
			WithField("job_id", job.ID).
			WithField("function_id", job.FunctionID).
			Warn("automation job perform failed")
		metrics.RecordAutomationExecution(job.ID+":perform", time.Since(performStart), false)
		finishPerform(err)
		return err
	}
	finishPerform(nil)
	metrics.RecordAutomationExecution(job.ID+":perform", time.Since(performStart), true)

	if d.service != nil {
		if _, err := d.service.RecordExecution(ctx, job.ID, runAt); err != nil {
			d.log.WithError(err).
				WithField("job_id", job.ID).
				Warn("record automation job execution failed")
		}
	}
	return nil
}

func interpretCheck(output map[string]any) (bool, map[string]any) {
	if len(output) == 0 {
		return true, map[string]any{}
	}
	perform := true
	if value, ok := output["shouldPerform"]; ok {
		switch v := value.(type) {
		case bool:
			perform = v
		case string:
			perform = strings.EqualFold(strings.TrimSpace(v), "true")
		case float64:
			perform = v != 0
		}
	}
	payload := map[string]any{}
	if value, ok := output["performPayload"]; ok {
		if m, ok := value.(map[string]any); ok {
			for k, v := range m {
				payload[k] = v
			}
		}
	}
	return perform, payload
}
