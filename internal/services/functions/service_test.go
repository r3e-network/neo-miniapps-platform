package functions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	datalinkdomain "github.com/R3E-Network/service_layer/internal/app/domain/datalink"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	automationsvc "github.com/R3E-Network/service_layer/internal/services/automation"
	datalinksvc "github.com/R3E-Network/service_layer/internal/services/datalink"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/services/gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/internal/services/oracle"
	pricefeedsvc "github.com/R3E-Network/service_layer/internal/services/pricefeed"
	randomsvc "github.com/R3E-Network/service_layer/internal/services/random"
	"github.com/R3E-Network/service_layer/internal/services/triggers"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
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
	svc.AttachDependencies(nil, nil, nil, nil, nil, nil, nil, gasService, nil)

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
	triggerService := triggers.New(store, store, store, nil)
	automationService := automationsvc.New(store, store, store, nil)
	oracleService := oraclesvc.New(store, store, nil)

	svc.AttachDependencies(triggerService, automationService, nil, nil, nil, nil, oracleService, gasService, nil)

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
	if len(result.Actions) != 4 {
		t.Fatalf("expected 4 action results, got %d", len(result.Actions))
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

	triggersList, _ := store.ListTriggers(context.Background(), acct.ID)
	if len(triggersList) != 1 {
		t.Fatalf("expected trigger stored, got %d", len(triggersList))
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

func TestService_ProcessPriceFeedSnapshotAction(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	priceSvc := pricefeedsvc.New(store, store, nil)
	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, priceSvc, nil, nil, nil, nil, nil, nil)

	randomSvc := randomsvc.New(store, nil)
	fnSvc.AttachDependencies(nil, nil, priceSvc, nil, nil, nil, nil, nil, randomSvc)

	fn, err := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "price", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}

	feed, err := priceSvc.CreateFeed(context.Background(), acct.ID, "NEO", "USD", "@every 1m", "@every 1h", 0.5)
	if err != nil {
		t.Fatalf("create feed: %v", err)
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
					ID:   "1",
					Type: function.ActionTypePriceFeedSnapshot,
					Params: map[string]any{
						"feedId":      feed.ID,
						"price":       12.34,
						"source":      "oracle",
						"collectedAt": now.Format(time.RFC3339),
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
	if action.Result == nil || action.Result["snapshot"] == nil {
		t.Fatalf("expected snapshot result, got %v", action.Result)
	}
}

func TestService_ProcessRandomGenerateAction(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	randomSvc := randomsvc.New(store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, nil, nil, nil, randomSvc)

	fn, err := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "random", Source: "() => 1"})
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
					ID:     "rand",
					Type:   function.ActionTypeRandomGenerate,
					Params: map[string]any{"length": 8, "requestId": "req-123"},
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
	if action.Result == nil || action.Result["value"] == nil {
		t.Fatalf("expected random result, got %v", action.Result)
	}
}

func TestService_ProcessDataLinkDeliveryAction(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	dlSvc := datalinksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, dlSvc, nil, nil, nil)

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
				ID:   "trigger",
				Type: function.ActionTypeTriggerRegister,
				Params: map[string]any{
					"type":   "cron",
					"rule":   "0 * * * *",
					"config": map[string]any{"timezone": "UTC"},
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

func TestService_RegisterTrigger(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	triggerSvc := triggers.New(store, store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(triggerSvc, nil, nil, nil, nil, nil, nil, nil, nil)

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "triggered", Source: "() => 1"})

	trg := trigger.Trigger{
		AccountID:  acct.ID,
		FunctionID: fn.ID,
		Type:       "cron",
		Rule:       "0 * * * *",
	}
	created, err := svc.RegisterTrigger(context.Background(), trg)
	if err != nil {
		t.Fatalf("register trigger: %v", err)
	}
	if created.FunctionID != fn.ID {
		t.Fatalf("expected trigger linked to function")
	}
}

func TestService_ScheduleAutomationJob(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	automationSvc := automationsvc.New(store, store, store, nil)

	svc := New(store, store, nil)
	svc.AttachDependencies(nil, automationSvc, nil, nil, nil, nil, nil, nil, nil)

	fn, _ := svc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "scheduled", Source: "() => 1"})

	job, err := svc.ScheduleAutomationJob(context.Background(), acct.ID, fn.ID, "hourly", "0 * * * *", "hourly run")
	if err != nil {
		t.Fatalf("schedule job: %v", err)
	}
	if job.FunctionID != fn.ID {
		t.Fatalf("expected job linked to function")
	}
}
