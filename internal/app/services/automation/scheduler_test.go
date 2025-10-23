package automation

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

type countingDispatcher struct {
	count int
}

func (d *countingDispatcher) DispatchJob(ctx context.Context, job domain.Job) error {
	d.count++
	return nil
}

func TestScheduler_RespectsNextRun(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	fn, err := store.CreateFunction(context.Background(), function.Definition{
		AccountID: acct.ID,
		Name:      "fn",
		Source:    "() => 1",
	})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	svc := New(store, store, store, nil)
	job, err := svc.CreateJob(context.Background(), acct.ID, fn.ID, "daily", "@daily", "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	future := time.Now().Add(2 * time.Hour)
	if _, err := svc.UpdateJob(context.Background(), job.ID, nil, nil, nil, &future); err != nil {
		t.Fatalf("set future next run: %v", err)
	}

	scheduler := NewScheduler(svc, nil)
	dispatcher := &countingDispatcher{}
	scheduler.WithDispatcher(dispatcher)

	scheduler.tick(context.Background())
	if dispatcher.count != 0 {
		t.Fatalf("expected no dispatch before next run, got %d", dispatcher.count)
	}

	past := time.Now().Add(-time.Minute)
	if _, err := svc.UpdateJob(context.Background(), job.ID, nil, nil, nil, &past); err != nil {
		t.Fatalf("set past next run: %v", err)
	}

	scheduler.tick(context.Background())
	if dispatcher.count != 1 {
		t.Fatalf("expected job to dispatch after next run reached, got %d", dispatcher.count)
	}
}
