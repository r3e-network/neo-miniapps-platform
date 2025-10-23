package function

import (
	"testing"
	"time"
)

func TestActionStatusValues(t *testing.T) {
	if ExecutionStatusSucceeded != "succeeded" {
		t.Fatalf("unexpected succeeded status: %s", ExecutionStatusSucceeded)
	}
	if ExecutionStatusFailed != "failed" {
		t.Fatalf("unexpected failed status: %s", ExecutionStatusFailed)
	}
}

func TestExecutionCapturesActions(t *testing.T) {
	started := time.Now().Add(-time.Second)
	completed := time.Now()

	exec := Execution{
		ID:          "exec-1",
		AccountID:   "acct-1",
		FunctionID:  "fn-1",
		Input:       map[string]any{"foo": "bar"},
		Output:      map[string]any{"ok": true},
		Logs:        []string{"log"},
		Status:      ExecutionStatusSucceeded,
		StartedAt:   started,
		CompletedAt: completed,
		Duration:    completed.Sub(started),
		Actions: []ActionResult{
			{
				Action: Action{ID: "a1", Type: "test", Params: map[string]any{"k": "v"}},
				Status: ActionStatusSucceeded,
			},
		},
	}

	if exec.Actions[0].Status != ActionStatusSucceeded {
		t.Fatalf("expected action to record status succeeded")
	}
	if exec.Output["ok"] != true {
		t.Fatalf("expected output to persist")
	}
}
