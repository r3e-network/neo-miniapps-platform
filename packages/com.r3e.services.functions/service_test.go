package functions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/applications/storage"
	"github.com/R3E-Network/service_layer/applications/storage/memory"
	"github.com/R3E-Network/service_layer/domain/account"
	datalinkdomain "github.com/R3E-Network/service_layer/domain/datalink"
	"github.com/R3E-Network/service_layer/domain/function"
	automationsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation"
	datalinksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink"
	gasbanksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

func TestService(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
	fn := function.Definition{AccountID: acct.ID, Name: "hello", Source: "() => 1"}
	created, err := svc.Create(context.Background(), fn)
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	created.Description = "desc"
	updated, err := svc.Update(context.Background(), created)
	if err != nil {
		t.Fatalf("update function: %v", err)
	}
	if updated.Description != "desc" {
		t.Fatalf("expected description update")
	}

	list, err := svc.List(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list functions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 function, got %d", len(list))
	}
}

func TestService_Execute(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.AttachExecutor(&mockExecutor{})

	created, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "echo", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := svc.Execute(context.Background(), created.ID, map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Status != function.ExecutionStatusSucceeded {
		t.Fatalf("expected succeeded status, got %s", result.Status)
	}
	if result.Output["foo"] != "bar" {
		t.Fatalf("unexpected output: %v", result.Output)
	}
	if result.Input["foo"] != "bar" {
		t.Fatalf("input not recorded: %v", result.Input)
	}

	execs, err := svc.ListExecutions(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].ID != result.ID {
		t.Fatalf("expected persisted execution ID %s, got %s", result.ID, execs[0].ID)
	}
}

func TestService_ExecuteFailureRecordsHistory(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.AttachExecutor(&failingExecutor{})

	created, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "fail", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := svc.Execute(context.Background(), created.ID, map[string]any{"foo": "bar"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if result.Status != function.ExecutionStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.Error == "" {
		t.Fatalf("expected error message recorded")
	}

	execs, listErr := svc.ListExecutions(context.Background(), created.ID, 0)
	if listErr != nil {
		t.Fatalf("list executions: %v", listErr)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution record, got %d", len(execs))
	}
	if execs[0].Status != function.ExecutionStatusFailed {
		t.Fatalf("expected persisted status failed, got %s", execs[0].Status)
	}
}

func TestService_ExecuteReturnsExecutorErrorWhenPersistenceFails(t *testing.T) {
	mem := memory.New()
	acct, _ := mem.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	persistErr := errors.New("persist execution failure")
	execErr := errors.New("executor failure")
	store := &failingFunctionStore{FunctionStore: mem, err: persistErr}

	svc := New(mem, store, nil)
	svc.AttachExecutor(&erroringExecutor{err: execErr})

	fn, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "boom", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	_, err = svc.Execute(context.Background(), fn.ID, nil)
	if err == nil {
		t.Fatalf("expected execution error")
	}
	if !errors.Is(err, execErr) {
		t.Fatalf("expected executor error to be preserved, got %v", err)
	}
	if !errors.Is(err, persistErr) {
		t.Fatalf("expected persistence error to be included, got %v", err)
	}
}

func TestService_UpdateValidatesSecrets(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	resolver := &stubSecretResolver{allowed: map[string]bool{"foo": true}}

	svc := New(store, store, nil)
	svc.AttachSecretResolver(resolver)

	created, err := svc.Create(context.Background(), function.Definition{
		AccountID: acct.ID,
		Name:      "secret-fn",
		Source:    "() => 1",
		Secrets:   []string{"foo"},
	})
	if err != nil {
		t.Fatalf("create with secrets: %v", err)
	}

	if _, err := svc.Update(context.Background(), function.Definition{ID: created.ID, Secrets: []string{"missing"}}); err == nil {
		t.Fatalf("expected secret validation error")
	}
	if len(resolver.last) != 1 || resolver.last[0] != "missing" {
		t.Fatalf("expected resolver to be called with new secrets, got %v", resolver.last)
	}

	fetched, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get function: %v", err)
	}
	if len(fetched.Secrets) != 1 || fetched.Secrets[0] != "foo" {
		t.Fatalf("stored secrets should remain unchanged, got %v", fetched.Secrets)
	}

	if _, err := svc.Update(context.Background(), function.Definition{ID: created.ID, Secrets: []string{}}); err != nil {
		t.Fatalf("clear secrets: %v", err)
	}

	fetched, err = svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get function after clearing: %v", err)
	}
	if len(fetched.Secrets) != 0 {
		t.Fatalf("expected secrets to be cleared, got %v", fetched.Secrets)
	}
}

func TestService_ExecutePreservesInputsAndOutputs(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)

	exec := &mutatingExecutor{}
	svc.AttachExecutor(exec)

	created, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "mutator", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	payload := map[string]any{
		"nested": map[string]any{
			"value": "original",
		},
		"list": []any{
			map[string]any{"value": "original"},
		},
	}

	result, err := svc.Execute(context.Background(), created.ID, payload)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := payload["nested"].(map[string]any)["value"]; got != "original" {
		t.Fatalf("expected original payload untouched, got %q", got)
	}
	if got := result.Input["nested"].(map[string]any)["value"]; got != "original" {
		t.Fatalf("expected persisted input to remain original, got %q", got)
	}
	if got := result.Input["list"].([]any)[0].(map[string]any)["value"]; got != "original" {
		t.Fatalf("expected nested slice input copy, got %q", got)
	}

	exec.output["nested"].(map[string]any)["value"] = "changed"
	if got := result.Output["nested"].(map[string]any)["value"]; got != "mutated" {
		t.Fatalf("expected persisted output to remain unchanged, got %q", got)
	}
}

func TestService_ExecuteProcessesActions(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.AttachExecutor(&actionExecutor{})
	gasService := gasbanksvc.New(store, store, nil)
	svc.AttachDependencies(nil, nil, nil, nil, nil, gasService, nil)

	created, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "action", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := svc.Execute(context.Background(), created.ID, map[string]any{"wallet": "NWALLET"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action result, got %d", len(result.Actions))
	}
	action := result.Actions[0]
	if action.Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s", action.Status)
	}
	accountMap, ok := action.Result["account"].(map[string]any)
	if !ok || accountMap["id"] == "" {
		t.Fatalf("expected account data in action result, got %#v", action.Result)
	}
	gasAccounts, err := store.ListGasAccounts(context.Background(), acct.ID)
	if err != nil || len(gasAccounts) != 1 {
		t.Fatalf("expected gas account persisted, err=%v len=%d", err, len(gasAccounts))
	}
}

func TestService_ExecuteProcessesMultipleActions(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	svc := New(store, store, nil)
	multiExec := &multiActionExecutor{}
	svc.AttachExecutor(multiExec)

	gasService := gasbanksvc.New(store, store, nil)
	automationService := automationsvc.New(store, store, store, nil)
	oracleService := oraclesvc.New(store, store, nil)

	svc.AttachDependencies(automationService, nil, nil, nil, oracleService, gasService, nil)

	fn, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "multi", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	src, err := oracleService.CreateSource(context.Background(), acct.ID, "datasource", "https://example.com", "GET", "", nil, "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	multiExec.sourceID = src.ID

	result, err := svc.Execute(context.Background(), fn.ID, map[string]any{"wallet": "NWallet"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Status != function.ExecutionStatusSucceeded {
		t.Fatalf("expected success status, got %s", result.Status)
	}
	if len(result.Actions) != 3 {
		t.Fatalf("expected 3 action results, got %d", len(result.Actions))
	}
	for _, action := range result.Actions {
		if action.Status != function.ActionStatusSucceeded {
			t.Fatalf("expected action success for %s, got %s", action.Type, action.Status)
		}
	}

	gasAccounts, _ := store.ListGasAccounts(context.Background(), acct.ID)
	if len(gasAccounts) != 1 {
		t.Fatalf("expected gas account ensured, got %d", len(gasAccounts))
	}

	jobs, _ := store.ListAutomationJobs(context.Background(), acct.ID)
	if len(jobs) != 1 || !jobs[0].Enabled {
		t.Fatalf("expected automation job stored and enabled, got %#v", jobs)
	}

	requests, _ := store.ListRequests(context.Background(), acct.ID, 10, "")
	if len(requests) != 1 {
		t.Fatalf("expected oracle request stored, got %d", len(requests))
	}
}

func TestService_ExecuteActionFailure(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	svc := New(store, store, nil)
	svc.AttachExecutor(&failingActionExecutor{})

	fn, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "fail-action", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	exec, err := svc.Execute(context.Background(), fn.ID, nil)
	if err == nil {
		t.Fatalf("expected error from unsupported action")
	}
	if exec.Status != function.ExecutionStatusFailed {
		t.Fatalf("expected execution failed status, got %s", exec.Status)
	}
	if len(exec.Actions) != 1 {
		t.Fatalf("expected one action result, got %d", len(exec.Actions))
	}
	if exec.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure, got %s", exec.Actions[0].Status)
	}
}

type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	return function.ExecutionResult{
		FunctionID:  def.ID,
		Output:      map[string]any{"foo": payload["foo"]},
		Status:      function.ExecutionStatusSucceeded,
		StartedAt:   time.Now().UTC(),
		CompletedAt: time.Now().UTC(),
	}, nil
}

type failingExecutor struct{}

func (f *failingExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	return function.ExecutionResult{
		FunctionID: def.ID,
		Output:     map[string]any{},
		StartedAt:  time.Now().UTC(),
	}, errors.New("boom")
}

type failingFunctionStore struct {
	storage.FunctionStore
	err error
}

func (s *failingFunctionStore) CreateExecution(ctx context.Context, exec function.Execution) (function.Execution, error) {
	return function.Execution{}, s.err
}

type erroringExecutor struct {
	err error
}

func (e *erroringExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	return function.ExecutionResult{
		FunctionID:  def.ID,
		Output:      map[string]any{},
		StartedAt:   time.Now().UTC(),
		CompletedAt: time.Now().UTC(),
	}, e.err
}

type stubSecretResolver struct {
	allowed map[string]bool
	last    []string
}

func (r *stubSecretResolver) ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	r.last = append([]string(nil), names...)
	for _, name := range names {
		if !r.allowed[name] {
			return nil, fmt.Errorf("secret %s not found", name)
		}
	}
	return map[string]string{}, nil
}

func ExampleService_Execute() {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "demo"})

	log := logger.NewDefault("example-functions")
	log.SetOutput(io.Discard)
	svc := New(store, store, log)
	svc.AttachExecutor(NewSimpleExecutor())

	fn, _ := svc.Create(context.Background(), function.Definition{
		AccountID: acct.ID,
		Name:      "hello",
		Source:    "() => ({greeting: 'hello world'})",
	})

	result, _ := svc.Execute(context.Background(), fn.ID, map[string]any{"foo": "bar"})
	fmt.Println(result.Status)
	fmt.Println(result.Output["message"])

	// Output:
	// succeeded
	// execution completed
}

func TestService_ProcessDataLinkDeliveryAction(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	dlSvc := datalinksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, dlSvc, nil, nil, nil)

	channel, err := dlSvc.CreateChannel(context.Background(), datalinkdomain.Channel{
		AccountID: acct.ID,
		Name:      "orders",
		Endpoint:  "https://example.com",
		AuthToken: "token",
		Status:    datalinkdomain.ChannelStatusActive,
		SignerSet: []string{"nwallet"},
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	fn, err := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "datalink", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{
					ID:   "dl-1",
					Type: function.ActionTypeDatalinkDeliver,
					Params: map[string]any{
						"channelId": channel.ID,
						"payload":   map[string]any{"value": "abc"},
						"metadata":  map[string]any{"trace": 1},
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute function: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected single action result, got %d", len(result.Actions))
	}
	action := result.Actions[0]
	if action.Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s", action.Status)
	}
	if action.Result == nil || action.Result["delivery"] == nil {
		t.Fatalf("expected delivery result, got %v", action.Result)
	}
	deliveries, err := store.ListDeliveries(context.Background(), acct.ID, 10)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].ChannelID != channel.ID {
		t.Fatalf("expected delivery persisted for channel %s, got %v", channel.ID, deliveries)
	}
}

type mutatingExecutor struct {
	output map[string]any
}

func (m *mutatingExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	if nested, ok := payload["nested"].(map[string]any); ok {
		nested["value"] = "mutated"
	}
	if lst, ok := payload["list"].([]any); ok && len(lst) > 0 {
		if nested, ok := lst[0].(map[string]any); ok {
			nested["value"] = "mutated"
		}
	}
	m.output = map[string]any{
		"nested": map[string]any{"value": "mutated"},
	}
	now := time.Now().UTC()
	return function.ExecutionResult{
		FunctionID:  def.ID,
		Output:      m.output,
		Status:      function.ExecutionStatusSucceeded,
		StartedAt:   now,
		CompletedAt: now,
	}, nil
}

type actionExecutor struct{}

func (a *actionExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	now := time.Now().UTC()
	return function.ExecutionResult{
		FunctionID: def.ID,
		Output: map[string]any{
			"ok": true,
		},
		Status:      function.ExecutionStatusSucceeded,
		StartedAt:   now,
		CompletedAt: now,
		Actions: []function.Action{
			{
				ID:     "ensure",
				Type:   function.ActionTypeGasBankEnsureAccount,
				Params: map[string]any{"wallet": payload["wallet"]},
			},
		},
	}, nil
}

type multiActionExecutor struct {
	sourceID string
}

func (m *multiActionExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	now := time.Now().UTC()
	return function.ExecutionResult{
		FunctionID: def.ID,
		Output: map[string]any{
			"handled": true,
		},
		Status:      function.ExecutionStatusSucceeded,
		StartedAt:   now,
		CompletedAt: now,
		Actions: []function.Action{
			{
				ID:     "gas",
				Type:   function.ActionTypeGasBankEnsureAccount,
				Params: map[string]any{"wallet": payload["wallet"]},
			},
			{
				ID:   "automation",
				Type: function.ActionTypeAutomationSchedule,
				Params: map[string]any{
					"name":        "hourly",
					"schedule":    "0 * * * *",
					"description": "hourly run",
				},
			},
			{
				ID:   "oracle",
				Type: function.ActionTypeOracleCreateRequest,
				Params: map[string]any{
					"dataSourceId": m.sourceID,
					"payload":      map[string]any{"pair": "NEO/USD"},
				},
			},
		},
	}, nil
}

type failingActionExecutor struct{}

func (f *failingActionExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	now := time.Now().UTC()
	return function.ExecutionResult{
		FunctionID:  def.ID,
		Output:      map[string]any{"ok": false},
		Status:      function.ExecutionStatusSucceeded,
		StartedAt:   now,
		CompletedAt: now,
		Actions: []function.Action{
			{ID: "unknown", Type: "unknown.action"},
		},
	}, nil
}

type staticExecutor struct {
	result function.ExecutionResult
}

func (s *staticExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	return s.result, nil
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "functions" {
		t.Fatalf("expected name functions")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "functions" {
		t.Fatalf("expected name functions")
	}
}

func TestService_CreateValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)

	// Missing account_id
	if _, err := svc.Create(context.Background(), function.Definition{Name: "test", Source: "() => 1"}); err == nil {
		t.Fatalf("expected account_id required error")
	}
	// Missing name
	if _, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Source: "() => 1"}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing source
	if _, err := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "test"}); err == nil {
		t.Fatalf("expected source required error")
	}
}

func TestService_Get(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "hello", Source: "() => 1"})

	got, err := svc.Get(context.Background(), fn.ID)
	if err != nil {
		t.Fatalf("get function: %v", err)
	}
	if got.ID != fn.ID {
		t.Fatalf("function mismatch")
	}
}

func TestService_ScheduleAutomationJob(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	automationSvc := automationsvc.New(store, store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(automationSvc, nil, nil, nil, nil, nil, nil)

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "scheduled", Source: "() => 1"})

	job, err := svc.ScheduleAutomationJob(context.Background(), acct.ID, fn.ID, "hourly", "0 * * * *", "hourly run")
	if err != nil {
		t.Fatalf("schedule job: %v", err)
	}
	if job.FunctionID != fn.ID {
		t.Fatalf("expected job linked to function")
	}
}

func TestService_UpdateAutomationJob(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	automationSvc := automationsvc.New(store, store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(automationSvc, nil, nil, nil, nil, nil, nil)

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "update-job", Source: "() => 1"})
	job, _ := svc.ScheduleAutomationJob(context.Background(), acct.ID, fn.ID, "hourly", "0 * * * *", "hourly run")

	newName := "daily"
	updated, err := svc.UpdateAutomationJob(context.Background(), job.ID, &newName, nil, nil)
	if err != nil {
		t.Fatalf("update job: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected name %q, got %q", newName, updated.Name)
	}
}

func TestService_UpdateAutomationJob_NoDependency(t *testing.T) {
	svc := New(nil, nil, nil)
	newName := "test"
	_, err := svc.UpdateAutomationJob(context.Background(), "job-1", &newName, nil, nil)
	if err == nil {
		t.Fatalf("expected dependency unavailable error")
	}
}

func TestService_SetAutomationEnabled(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	automationSvc := automationsvc.New(store, store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(automationSvc, nil, nil, nil, nil, nil, nil)

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "toggle-job", Source: "() => 1"})
	job, _ := svc.ScheduleAutomationJob(context.Background(), acct.ID, fn.ID, "test", "0 * * * *", "")

	updated, err := svc.SetAutomationEnabled(context.Background(), job.ID, false)
	if err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if updated.Enabled {
		t.Fatalf("expected job disabled")
	}
}

func TestService_SetAutomationEnabled_NoDependency(t *testing.T) {
	svc := New(nil, nil, nil)
	_, err := svc.SetAutomationEnabled(context.Background(), "job-1", true)
	if err == nil {
		t.Fatalf("expected dependency unavailable error")
	}
}

func TestService_CreateOracleRequest(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	oracleSvc := oraclesvc.New(store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(nil, nil, nil, nil, oracleSvc, nil, nil)

	src, _ := oracleSvc.CreateSource(context.Background(), acct.ID, "test", "https://example.com", "GET", "", nil, "")

	req, err := svc.CreateOracleRequest(context.Background(), acct.ID, src.ID, `{"key":"value"}`)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if req.DataSourceID != src.ID {
		t.Fatalf("expected source ID match")
	}
}

func TestService_CreateOracleRequest_NoDependency(t *testing.T) {
	svc := New(nil, nil, nil)
	_, err := svc.CreateOracleRequest(context.Background(), "acc-1", "src-1", "")
	if err == nil {
		t.Fatalf("expected dependency unavailable error")
	}
}

func TestService_CompleteOracleRequest(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	oracleSvc := oraclesvc.New(store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(nil, nil, nil, nil, oracleSvc, nil, nil)

	src, _ := oracleSvc.CreateSource(context.Background(), acct.ID, "test", "https://example.com", "GET", "", nil, "")
	req, _ := oracleSvc.CreateRequest(context.Background(), acct.ID, src.ID, "")
	_, _ = oracleSvc.MarkRunning(context.Background(), req.ID)

	completed, err := svc.CompleteOracleRequest(context.Background(), req.ID, `{"result":"ok"}`)
	if err != nil {
		t.Fatalf("complete request: %v", err)
	}
	if completed.Result != `{"result":"ok"}` {
		t.Fatalf("expected result match")
	}
}

func TestService_CompleteOracleRequest_NoDependency(t *testing.T) {
	svc := New(nil, nil, nil)
	_, err := svc.CompleteOracleRequest(context.Background(), "req-1", "result")
	if err == nil {
		t.Fatalf("expected dependency unavailable error")
	}
}

func TestService_EnsureGasAccount(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasSvc := gasbanksvc.New(store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(nil, nil, nil, nil, nil, gasSvc, nil)

	gasAcct, err := svc.EnsureGasAccount(context.Background(), acct.ID, "NWALLET")
	if err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	if gasAcct.WalletAddress != "nwallet" {
		t.Fatalf("expected wallet match, got %q", gasAcct.WalletAddress)
	}
}

func TestService_EnsureGasAccount_NoDependency(t *testing.T) {
	svc := New(nil, nil, nil)
	_, err := svc.EnsureGasAccount(context.Background(), "acc-1", "wallet")
	if err == nil {
		t.Fatalf("expected dependency unavailable error")
	}
}

func TestService_ScheduleAutomationJob_NoDependency(t *testing.T) {
	svc := New(nil, nil, nil)
	_, err := svc.ScheduleAutomationJob(context.Background(), "acc", "fn", "name", "0 * * * *", "")
	if err == nil {
		t.Fatalf("expected dependency unavailable error")
	}
}

func TestService_GetExecution(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.AttachExecutor(&mockExecutor{})

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "exec", Source: "() => 1"})
	exec, _ := svc.Execute(context.Background(), fn.ID, map[string]any{"foo": "bar"})

	fetched, err := svc.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if fetched.ID != exec.ID {
		t.Fatalf("expected execution ID match")
	}
}

func TestService_Invoke(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.AttachExecutor(&mockExecutor{})
	_ = svc.Start(context.Background())

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "invoke", Source: "() => 1"})

	result, err := svc.Invoke(context.Background(), map[string]any{
		"account_id":  acct.ID,
		"function_id": fn.ID,
		"input":       "test",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
}

func TestService_Invoke_WithMapInput(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.AttachExecutor(&mockExecutor{})
	_ = svc.Start(context.Background())

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "invoke-map", Source: "() => 1"})

	result, err := svc.Invoke(context.Background(), map[string]any{
		"account_id":  acct.ID,
		"function_id": fn.ID,
		"input":       map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
}

func TestService_Invoke_NotReady(t *testing.T) {
	svc := New(nil, nil, nil)
	_, err := svc.Invoke(context.Background(), map[string]any{})
	if err == nil {
		t.Fatalf("expected not ready error")
	}
}

func TestService_Invoke_InvalidPayload(t *testing.T) {
	svc := New(nil, nil, nil)
	_ = svc.Start(context.Background())
	_, err := svc.Invoke(context.Background(), "not a map")
	if err == nil {
		t.Fatalf("expected payload type error")
	}
}

func TestService_Invoke_MissingParams(t *testing.T) {
	svc := New(nil, nil, nil)
	_ = svc.Start(context.Background())
	_, err := svc.Invoke(context.Background(), map[string]any{})
	if err == nil {
		t.Fatalf("expected missing params error")
	}
}
