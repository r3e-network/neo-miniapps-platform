package automation

import (
	"testing"
	"time"
)

func TestJobFields(t *testing.T) {
	now := time.Now()
	job := Job{
		ID:         "job-1",
		AccountID:  "acct-1",
		FunctionID: "fn-1",
		Name:       "Hourly",
		Schedule:   "0 * * * *",
		Enabled:    true,
		LastRun:    now.Add(-time.Hour),
		NextRun:    now,
	}

	if job.Schedule == "" || job.Name == "" {
		t.Fatalf("expected job to retain schedule and name")
	}
	if !job.Enabled {
		t.Fatalf("expected job to be enabled")
	}
	if job.NextRun.IsZero() {
		t.Fatalf("expected next run to be set")
	}
}
