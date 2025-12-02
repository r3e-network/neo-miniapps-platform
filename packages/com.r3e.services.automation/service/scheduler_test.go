package automation

import (
	"context"
	"errors"
	"testing"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

type countingDispatcher struct {
	count int
}

func (d *countingDispatcher) DispatchJob(ctx context.Context, job Job) error {
	d.count++
	return nil
}

type errorDispatcher struct{}

func (d *errorDispatcher) DispatchJob(ctx context.Context, job Job) error {
	return errors.New("dispatch error")
}

type tracedDispatcher struct {
	tracer core.Tracer
}

func (d *tracedDispatcher) DispatchJob(ctx context.Context, job Job) error {
	return nil
}

func (d *tracedDispatcher) WithTracer(t core.Tracer) {
	d.tracer = t
}

func TestScheduler_RespectsNextRun(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestScheduler_DispatchesEnabledJobs(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestScheduler_SkipsDisabledJobs(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestScheduler_HandlesDispatchError(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestScheduler_TracerIntegration(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}
