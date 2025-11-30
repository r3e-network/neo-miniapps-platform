package automation

import (
	"context"
	"testing"
)

type scriptStep struct {
	output FunctionExecution
	err    error
}

type scriptedRunner struct {
	script  []scriptStep
	payload []map[string]any
}

func (r *scriptedRunner) Execute(ctx context.Context, functionID string, payload map[string]any) (FunctionExecution, error) {
	r.payload = append(r.payload, cloneMap(payload))
	if len(r.script) == 0 {
		return FunctionExecution{}, nil
	}
	step := r.script[0]
	r.script = r.script[1:]
	return step.output, step.err
}

func TestFunctionDispatcher_PerformFlow(t *testing.T) {
	checkOutput := FunctionExecution{
		Output: map[string]any{
			"shouldPerform": true,
			"performPayload": map[string]any{
				"foo": "bar",
			},
		},
	}
	runner := &scriptedRunner{
		script: []scriptStep{
			{output: checkOutput},
			{output: FunctionExecution{}},
		},
	}
	dispatcher := NewFunctionDispatcher(runner, nil, nil)
	job := Job{ID: "job-1", FunctionID: "fn-1", Enabled: true}

	if err := dispatcher.DispatchJob(context.Background(), job); err != nil {
		t.Fatalf("dispatch job: %v", err)
	}
	if len(runner.payload) != 2 {
		t.Fatalf("expected two executions, got %d", len(runner.payload))
	}
	if phase := runner.payload[0]["phase"]; phase != "check" {
		t.Fatalf("expected first phase to be check, got %v", phase)
	}
	if phase := runner.payload[1]["phase"]; phase != "perform" {
		t.Fatalf("expected second phase to be perform, got %v", phase)
	}
	if value := runner.payload[1]["foo"]; value != "bar" {
		t.Fatalf("perform payload missing: %v", runner.payload[1])
	}
}

func TestFunctionDispatcher_CheckSkip(t *testing.T) {
	runner := &scriptedRunner{
		script: []scriptStep{
			{output: FunctionExecution{
				Output: map[string]any{
					"shouldPerform": false,
				},
			}},
		},
	}
	dispatcher := NewFunctionDispatcher(runner, nil, nil)
	job := Job{ID: "job-2", FunctionID: "fn-2", Enabled: true}

	if err := dispatcher.DispatchJob(context.Background(), job); err != nil {
		t.Fatalf("dispatch job: %v", err)
	}
	if len(runner.payload) != 1 {
		t.Fatalf("expected single execution, got %d", len(runner.payload))
	}
	if phase := runner.payload[0]["phase"]; phase != "check" {
		t.Fatalf("expected check phase, got %v", phase)
	}
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
