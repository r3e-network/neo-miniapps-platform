package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	core "github.com/R3E-Network/service_layer/internal/services/core"
)

type countingDispatcher struct {
	count int
}

func (d *countingDispatcher) DispatchJob(ctx context.Context, job domain.Job) error {
	d.count++
	return nil
}

type errorDispatcher struct{}

func (d *errorDispatcher) DispatchJob(ctx context.Context, job domain.Job) error {
	return errors.New("dispatch error")
}

type tracedDispatcher struct {
	tracer core.Tracer
}

func (d *tracedDispatcher) DispatchJob(ctx context.Context, job domain.Job) error {
	return nil
}

func (d *tracedDispatcher) WithTracer(t core.Tracer) {
	d.tracer = t
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

func TestScheduler_NameAndDomain(t *testing.T) {
	scheduler := NewScheduler(nil, nil)
	if scheduler.Name() != "automation-scheduler" {
		t.Errorf("expected name automation-scheduler, got %s", scheduler.Name())
	}
	if scheduler.Domain() != "automation" {
		t.Errorf("expected domain automation, got %s", scheduler.Domain())
	}
}

func TestScheduler_Descriptor(t *testing.T) {
	scheduler := NewScheduler(nil, nil)
	desc := scheduler.Descriptor()
	if desc.Name != "runner-automation" {
		t.Errorf("expected descriptor name runner-automation, got %s", desc.Name)
	}
	if desc.Domain != "automation" {
		t.Errorf("expected descriptor domain automation, got %s", desc.Domain)
	}
	if desc.Layer != core.LayerRunner {
		t.Errorf("expected layer runner, got %s", desc.Layer)
	}
	if len(desc.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(desc.Capabilities))
	}
}

func TestScheduler_StartStop(t *testing.T) {
	store := memory.New()
	svc := New(store, store, store, nil)
	scheduler := NewScheduler(svc, nil)

	// Start the scheduler
	ctx := context.Background()
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Should be ready
	if err := scheduler.Ready(ctx); err != nil {
		t.Errorf("expected ready after start, got: %v", err)
	}

	// Double start should be no-op
	if err := scheduler.Start(ctx); err != nil {
		t.Errorf("double start should not error: %v", err)
	}

	// Stop the scheduler
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := scheduler.Stop(stopCtx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	// Double stop should be no-op
	if err := scheduler.Stop(ctx); err != nil {
		t.Errorf("double stop should not error: %v", err)
	}

	// Ready should fail after stop
	if err := scheduler.Ready(ctx); err == nil {
		t.Error("expected ready to fail after stop")
	}
}

func TestScheduler_WithDispatcher(t *testing.T) {
	scheduler := NewScheduler(nil, nil)
	dispatcher := &countingDispatcher{}
	scheduler.WithDispatcher(dispatcher)

	// No panic when WithDispatcher is called
	scheduler.WithDispatcher(nil)
}

func TestScheduler_WithTracer(t *testing.T) {
	scheduler := NewScheduler(nil, nil)

	// Set a traced dispatcher first
	traced := &tracedDispatcher{}
	scheduler.WithDispatcher(traced)

	// WithTracer should propagate to dispatcher
	scheduler.WithTracer(core.NoopTracer)
	if traced.tracer == nil {
		t.Error("expected tracer to be set on dispatcher")
	}

	// nil tracer should use NoopTracer
	scheduler.WithTracer(nil)
}

func TestScheduler_TickWithNilService(t *testing.T) {
	scheduler := NewScheduler(nil, nil)
	// Should not panic with nil service
	scheduler.tick(context.Background())
}

func TestScheduler_TickWithNilDispatcher(t *testing.T) {
	store := memory.New()
	svc := New(store, store, store, nil)
	scheduler := NewScheduler(svc, nil)
	// Should not panic without dispatcher
	scheduler.tick(context.Background())
}

func TestScheduler_TickWithDisabledJob(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	fn, _ := store.CreateFunction(context.Background(), function.Definition{
		AccountID: acct.ID,
		Name:      "fn",
		Source:    "() => 1",
	})

	svc := New(store, store, store, nil)
	job, err := svc.CreateJob(context.Background(), acct.ID, fn.ID, "disabled-job", "@daily", "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Disable the job
	enabled := false
	if _, err := svc.SetEnabled(context.Background(), job.ID, enabled); err != nil {
		t.Fatalf("disable job: %v", err)
	}

	scheduler := NewScheduler(svc, nil)
	dispatcher := &countingDispatcher{}
	scheduler.WithDispatcher(dispatcher)

	scheduler.tick(context.Background())
	if dispatcher.count != 0 {
		t.Errorf("expected no dispatch for disabled job, got %d", dispatcher.count)
	}
}

func TestScheduler_TickWithDispatchError(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	fn, _ := store.CreateFunction(context.Background(), function.Definition{
		AccountID: acct.ID,
		Name:      "fn",
		Source:    "() => 1",
	})

	svc := New(store, store, store, nil)
	job, _ := svc.CreateJob(context.Background(), acct.ID, fn.ID, "error-job", "@daily", "")

	// Set past next run
	past := time.Now().Add(-time.Minute)
	svc.UpdateJob(context.Background(), job.ID, nil, nil, nil, &past)

	scheduler := NewScheduler(svc, nil)
	scheduler.WithDispatcher(&errorDispatcher{})

	// Should not panic on dispatch error
	scheduler.tick(context.Background())
}

func TestJobDispatcherFunc_Nil(t *testing.T) {
	var fn JobDispatcherFunc
	if err := fn.DispatchJob(context.Background(), domain.Job{}); err != nil {
		t.Errorf("nil JobDispatcherFunc should return nil, got: %v", err)
	}
}

func TestJobDispatcherFunc_Valid(t *testing.T) {
	called := false
	fn := JobDispatcherFunc(func(ctx context.Context, job domain.Job) error {
		called = true
		return nil
	})

	if err := fn.DispatchJob(context.Background(), domain.Job{ID: "test"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected function to be called")
	}
}
