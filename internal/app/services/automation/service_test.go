package automation

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

func TestService_CreateAndUpdateJob(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	fn, err := store.CreateFunction(context.Background(), function.Definition{AccountID: acct.ID, Name: "fn", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	svc := New(store, store, store, nil)
	job, err := svc.CreateJob(context.Background(), acct.ID, fn.ID, "daily", "0 0 * * *", "desc")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if !job.Enabled {
		t.Fatalf("job should be enabled by default")
	}

	if _, err := svc.CreateJob(context.Background(), acct.ID, fn.ID, "daily", "@hourly", "dup"); err == nil {
		t.Fatalf("expected duplicate name error")
	}

	newName := "nightly"
	newSchedule := "0 0 * * 1"
	newDesc := "updated"
	next := time.Now().Add(24 * time.Hour)
	updated, err := svc.UpdateJob(context.Background(), job.ID, &newName, &newSchedule, &newDesc, &next)
	if err != nil {
		t.Fatalf("update job: %v", err)
	}
	if updated.Name != newName || updated.Schedule != newSchedule || updated.Description != newDesc {
		t.Fatalf("update not applied: %#v", updated)
	}
	if !updated.NextRun.Equal(next.UTC()) {
		t.Fatalf("next run mismatch: %v", updated.NextRun)
	}

	disabled, err := svc.SetEnabled(context.Background(), job.ID, false)
	if err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("job should be disabled")
	}

	runTime := time.Now()
	nextTime := runTime.Add(time.Hour)
	recorded, err := svc.RecordExecution(context.Background(), job.ID, runTime, nextTime)
	if err != nil {
		t.Fatalf("record execution: %v", err)
	}
	if !recorded.LastRun.Equal(runTime.UTC()) || !recorded.NextRun.Equal(nextTime.UTC()) {
		t.Fatalf("execution timestamps not updated")
	}

	list, err := svc.ListJobs(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one job, got %d", len(list))
	}
}
