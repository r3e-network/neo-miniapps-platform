package tee

import (
	"context"
	"testing"
)

func TestScriptService_Create(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	def, err := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "test-script",
		Source:    `(function(params) { return { hello: "world" }; })`,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if def.ID == "" {
		t.Error("expected non-empty ID")
	}
	if def.Name != "test-script" {
		t.Errorf("expected name 'test-script', got %s", def.Name)
	}
}

func TestScriptService_CreateValidation(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	// Missing account_id
	_, err := svc.Create(ctx, ScriptDefinition{
		Name:   "test",
		Source: `function() {}`,
	})
	if err == nil {
		t.Error("expected error for missing account_id")
	}

	// Missing name
	_, err = svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Source:    `(function() )`,
	})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Missing source
	_, err = svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "test",
	})
	if err == nil {
		t.Error("expected error for missing source")
	}
}

func TestScriptService_Get(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "test-script",
		Source:    `(function(params) { return {}; })`,
	})

	retrieved, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if retrieved.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, retrieved.ID)
	}
}

func TestScriptService_List(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	// Create scripts for different accounts
	_, _ = svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "script-1",
		Source:    `(function() { return {}; })`,
	})
	_, _ = svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "script-2",
		Source:    `(function() { return {}; })`,
	})
	_, _ = svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-2",
		Name:      "script-3",
		Source:    `(function() { return {}; })`,
	})

	// List for acct-1
	scripts, err := svc.List(ctx, "acct-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scripts) != 2 {
		t.Errorf("expected 2 scripts for acct-1, got %d", len(scripts))
	}

	// List for acct-2
	scripts, err = svc.List(ctx, "acct-2")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scripts) != 1 {
		t.Errorf("expected 1 script for acct-2, got %d", len(scripts))
	}
}

func TestScriptService_Update(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID:   "acct-1",
		Name:        "original-name",
		Description: "original description",
		Source:      `(function() { return { v: 1 }; })`,
	})

	updated, err := svc.Update(ctx, ScriptDefinition{
		ID:          created.ID,
		Name:        "updated-name",
		Description: "updated description",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "updated-name" {
		t.Errorf("expected name 'updated-name', got %s", updated.Name)
	}
	if updated.Description != "updated description" {
		t.Errorf("expected updated description")
	}
	// Source should be preserved
	if updated.Source != created.Source {
		t.Errorf("source should be preserved when not provided")
	}
}

func TestScriptService_Delete(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "to-delete",
		Source:    `(function() { return {}; })`,
	})

	err := svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = svc.Get(ctx, created.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestScriptService_Execute(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "adder",
		Source:    `(function(params) { return { sum: params.a + params.b }; })`,
	})

	run, err := svc.Execute(ctx, created.ID, map[string]any{"a": 10, "b": 20})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if run.Status != ScriptStatusSucceeded {
		t.Errorf("expected succeeded, got %s: %s", run.Status, run.Error)
	}

	// Check output - goja may return int64 or float64
	var sum float64
	switch v := run.Output["sum"].(type) {
	case float64:
		sum = v
	case int64:
		sum = float64(v)
	}
	if sum != 30 {
		t.Errorf("expected sum=30, got %v", run.Output["sum"])
	}
}

func TestScriptService_ExecuteWithLogs(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "logger",
		Source: `(function(params) {
			console.log("Hello from script");
			console.log("Input:", JSON.stringify(params));
			return { logged: true };
		})`,
	})

	run, err := svc.Execute(ctx, created.ID, map[string]any{"test": "value"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(run.Logs) < 1 {
		t.Error("expected at least one log entry")
	}
}

func TestScriptService_ExecuteError(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "error-script",
		Source:    `(function(params) { throw new Error("intentional error"); })`,
	})

	run, _ := svc.Execute(ctx, created.ID, map[string]any{})

	if run.Status != ScriptStatusFailed {
		t.Errorf("expected failed status, got %s", run.Status)
	}
	if run.Error == "" {
		t.Error("expected error message")
	}
}

func TestScriptService_ListRuns(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "multi-run",
		Source:    `(function(params) { return { n: params.n }; })`,
	})

	// Execute multiple times
	for i := 0; i < 5; i++ {
		_, _ = svc.Execute(ctx, created.ID, map[string]any{"n": i})
	}

	runs, err := svc.ListRuns(ctx, created.ID, 10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 5 {
		t.Errorf("expected 5 runs, got %d", len(runs))
	}

	// Test limit
	runs, _ = svc.ListRuns(ctx, created.ID, 3)
	if len(runs) != 3 {
		t.Errorf("expected 3 runs with limit, got %d", len(runs))
	}
}

func TestScriptService_Invoke(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "invoke-test",
		Source:    `(function(params) { return { result: "invoked" }; })`,
	})

	result, err := svc.Invoke(ctx, map[string]any{
		"script_id":  created.ID,
		"account_id": "acct-1",
		"input":      map[string]any{},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	run, ok := result.(ScriptRun)
	if !ok {
		t.Fatalf("expected ScriptRun, got %T", result)
	}
	if run.Status != ScriptStatusSucceeded {
		t.Errorf("expected succeeded, got %s", run.Status)
	}
}

func TestScriptService_InvokeBackwardCompatibility(t *testing.T) {
	engine, _ := NewEngine(EngineConfig{Mode: EnclaveModeSimulation})
	ctx := context.Background()
	_ = engine.Start(ctx)
	defer engine.Stop(ctx)

	store := NewMemoryScriptStore()
	svc := NewScriptService(ScriptServiceConfig{
		Engine: engine,
		Store:  store,
	})
	_ = svc.Start(ctx)

	created, _ := svc.Create(ctx, ScriptDefinition{
		AccountID: "acct-1",
		Name:      "compat-test",
		Source:    `(function(params) { return { ok: true }; })`,
	})

	// Use function_id instead of script_id (backward compatibility)
	result, err := svc.Invoke(ctx, map[string]any{
		"function_id": created.ID,
		"account_id":  "acct-1",
	})
	if err != nil {
		t.Fatalf("Invoke with function_id: %v", err)
	}

	run, ok := result.(ScriptRun)
	if !ok {
		t.Fatalf("expected ScriptRun, got %T", result)
	}
	if run.Status != ScriptStatusSucceeded {
		t.Errorf("expected succeeded, got %s", run.Status)
	}
}
