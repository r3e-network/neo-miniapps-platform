package automation

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
)

type fakeRunner struct{ count int }

func (f *fakeRunner) Execute(ctx context.Context, functionID string, payload map[string]any) error {
	f.count++
	return nil
}

func TestFunctionDispatcher_Run(t *testing.T) {
	runner := &fakeRunner{}
	dispatcher := NewFunctionDispatcher(runner, nil, nil)
	job := automation.Job{ID: "job-1", FunctionID: "fn-1", Enabled: true}
	if err := dispatcher.DispatchJob(context.Background(), job); err != nil {
		t.Fatalf("dispatch job: %v", err)
	}
	if runner.count != 1 {
		t.Fatalf("expected runner to be invoked once, got %d", runner.count)
	}
}
