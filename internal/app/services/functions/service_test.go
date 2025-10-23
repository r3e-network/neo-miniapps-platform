package functions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	automationsvc "github.com/R3E-Network/service_layer/internal/app/services/automation"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/app/services/gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/internal/app/services/oracle"
	"github.com/R3E-Network/service_layer/internal/app/services/triggers"
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
	svc.AttachDependencies(nil, nil, nil, nil, gasService)

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

	svc.AttachDependencies(triggerService, automationService, nil, oracleService, gasService)

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

	requests, _ := store.ListRequests(context.Background(), acct.ID)
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
