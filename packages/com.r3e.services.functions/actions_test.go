package functions

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/applications/storage/memory"
	"github.com/R3E-Network/service_layer/domain/account"
	datafeeddomain "github.com/R3E-Network/service_layer/domain/datafeeds"
	datalinkdomain "github.com/R3E-Network/service_layer/domain/datalink"
	datastreamsdomain "github.com/R3E-Network/service_layer/domain/datastreams"
	"github.com/R3E-Network/service_layer/domain/function"
	automationsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation"
	datafeedsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datafeeds"
	datalinksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink"
	datastreamsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams"
	gasbanksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
)

func TestAction_GasBankWithdraw(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasSvc := gasbanksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, gasSvc, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "withdraw", Source: "() => 1"})

	// Ensure gas account with funds
	gasAcct, _ := gasSvc.EnsureAccount(context.Background(), acct.ID, "NWALLET")
	_, _, _ = gasSvc.Deposit(context.Background(), gasAcct.ID, 100.0, "test-deposit", "from", "to")

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
					ID:   "withdraw",
					Type: function.ActionTypeGasBankWithdraw,
					Params: map[string]any{
						"wallet": "NWALLET",
						"amount": 10.0,
						"to":     "NDestination",
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}

func TestAction_GasBankWithdraw_ByGasAccountID(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasSvc := gasbanksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, gasSvc, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "withdraw-by-id", Source: "() => 1"})

	// Ensure gas account with funds
	gasAcct, _ := gasSvc.EnsureAccount(context.Background(), acct.ID, "NWALLET")
	_, _, _ = gasSvc.Deposit(context.Background(), gasAcct.ID, 100.0, "test-deposit", "from", "to")

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
					ID:   "withdraw",
					Type: function.ActionTypeGasBankWithdraw,
					Params: map[string]any{
						"gasAccountId": gasAcct.ID,
						"amount":       10.0,
						"to":           "NDestination",
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}

func TestAction_GasBankWithdraw_MissingParams(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasSvc := gasbanksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, gasSvc, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "withdraw-missing", Source: "() => 1"})

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
					ID:     "withdraw",
					Type:   function.ActionTypeGasBankWithdraw,
					Params: map[string]any{},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure")
	}
}

func TestAction_GasBankBalance(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasSvc := gasbanksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, gasSvc, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "balance", Source: "() => 1"})

	// Ensure gas account
	_, _ = gasSvc.EnsureAccount(context.Background(), acct.ID, "NWALLET")

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
					ID:   "balance",
					Type: function.ActionTypeGasBankBalance,
					Params: map[string]any{
						"wallet": "NWALLET",
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s", result.Actions[0].Status)
	}
}

func TestAction_GasBankBalance_ByGasAccountID(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasSvc := gasbanksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, gasSvc, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "balance-id", Source: "() => 1"})
	gasAcct, _ := gasSvc.EnsureAccount(context.Background(), acct.ID, "NWALLET")

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
					ID:   "balance",
					Type: function.ActionTypeGasBankBalance,
					Params: map[string]any{
						"gasAccountId": gasAcct.ID,
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s", result.Actions[0].Status)
	}
}

func TestAction_GasBankListTransactions(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	gasSvc := gasbanksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, nil, gasSvc, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "list-tx", Source: "() => 1"})
	gasAcct, _ := gasSvc.EnsureAccount(context.Background(), acct.ID, "NWALLET")
	_, _, _ = gasSvc.Deposit(context.Background(), gasAcct.ID, 100.0, "test-deposit", "from", "to")

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
					ID:   "list",
					Type: function.ActionTypeGasBankListTx,
					Params: map[string]any{
						"wallet": "NWALLET",
						"limit":  10,
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}

func TestAction_DataFeedSubmit(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	dfSvc := datafeedsvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, dfSvc, nil, nil, nil, nil, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "datafeed", Source: "() => 1"})

	// Create data feed
	feed, _ := dfSvc.CreateFeed(context.Background(), datafeeddomain.Feed{
		AccountID:   acct.ID,
		Pair:        "NEO/USD",
		Description: "test",
		Decimals:    8,
		Aggregation: "median",
		SignerSet:   []string{"signer1"},
	})

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
					ID:   "datafeed",
					Type: function.ActionTypeDataFeedSubmit,
					Params: map[string]any{
						"feedId":    feed.ID,
						"roundId":   1,
						"price":     "100.50",
						"signer":    "signer1",
						"signature": "sig",
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}

func TestAction_DatastreamPublish(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	dsSvc := datastreamsvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, dsSvc, nil, nil, nil, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "datastream", Source: "() => 1"})

	// Create data stream
	stream, _ := dsSvc.CreateStream(context.Background(), datastreamsdomain.Stream{
		AccountID:   acct.ID,
		Name:        "test-stream",
		Symbol:      "TEST",
		Description: "test",
	})

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
					ID:   "publish",
					Type: function.ActionTypeDatastreamPublish,
					Params: map[string]any{
						"streamId": stream.ID,
						"sequence": 1,
						"payload":  map[string]any{"value": 123},
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}

func TestAction_OracleCreateRequest_WithAlternateSources(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	oracleSvc := oraclesvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, nil, oracleSvc, nil, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "oracle", Source: "() => 1"})

	src1, _ := oracleSvc.CreateSource(context.Background(), acct.ID, "primary", "https://example.com", "GET", "", nil, "")
	src2, _ := oracleSvc.CreateSource(context.Background(), acct.ID, "backup", "https://backup.com", "GET", "", nil, "")

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
					ID:   "oracle",
					Type: function.ActionTypeOracleCreateRequest,
					Params: map[string]any{
						"dataSourceId":       src1.ID,
						"payload":            `{"pair":"NEO/USD"}`,
						"alternateSourceIds": []string{src2.ID},
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}

func TestAction_AutomationSchedule_WithEnabled(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	autoSvc := automationsvc.New(store, store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(autoSvc, nil, nil, nil, nil, nil, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "automation", Source: "() => 1"})

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
					ID:   "automation",
					Type: function.ActionTypeAutomationSchedule,
					Params: map[string]any{
						"name":        "job",
						"schedule":    "0 * * * *",
						"description": "test job",
						"enabled":     false,
					},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, err := fnSvc.Execute(context.Background(), fn.ID, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}

func TestAction_DatalinkDelivery_MissingChannel(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	dlSvc := datalinksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, dlSvc, nil, nil, nil)

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "datalink", Source: "() => 1"})

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
					ID:     "dl",
					Type:   function.ActionTypeDatalinkDeliver,
					Params: map[string]any{},
				},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure")
	}
}

func TestAction_NoDependency_GasBankWithdraw(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeGasBankWithdraw, Params: map[string]any{"wallet": "w"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_GasBankBalance(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeGasBankBalance, Params: map[string]any{"wallet": "w"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_GasBankListTx(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeGasBankListTx, Params: map[string]any{"wallet": "w"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_OracleCreateRequest(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeOracleCreateRequest, Params: map[string]any{"dataSourceId": "src"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_PriceFeedSnapshot(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypePriceFeedSnapshot, Params: map[string]any{"feedId": "feed", "price": 10.0}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_DataFeedSubmit(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeDataFeedSubmit, Params: map[string]any{"feedId": "feed", "roundId": 1, "price": "100"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_RandomGenerate(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeRandomGenerate, Params: map[string]any{"length": 32}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_DatastreamPublish(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeDatastreamPublish, Params: map[string]any{"streamId": "s", "sequence": 1}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_DatalinkDeliver(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeDatalinkDeliver, Params: map[string]any{"channelId": "ch"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_TriggerRegister(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeTriggerRegister, Params: map[string]any{"type": "cron"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

func TestAction_NoDependency_AutomationSchedule(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})

	fnSvc := New(store, store, nil)
	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "no-dep", Source: "() => 1"})

	now := time.Now().UTC()
	exec := &staticExecutor{
		result: function.ExecutionResult{
			FunctionID:  fn.ID,
			Output:      map[string]any{},
			Status:      function.ExecutionStatusSucceeded,
			StartedAt:   now,
			CompletedAt: now,
			Actions: []function.Action{
				{ID: "1", Type: function.ActionTypeAutomationSchedule, Params: map[string]any{"name": "job", "schedule": "0 * * * *"}},
			},
		},
	}
	fnSvc.AttachExecutor(exec)

	result, _ := fnSvc.Execute(context.Background(), fn.ID, nil)
	if result.Actions[0].Status != function.ActionStatusFailed {
		t.Fatalf("expected action failure without dependency")
	}
}

// Datalinkdelivery with a valid channel was already covered in service_test.go
// This file tests additional edge cases.

func TestAction_DatalinkDelivery_ValidChannel(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	dlSvc := datalinksvc.New(store, store, nil)

	fnSvc := New(store, store, nil)
	fnSvc.AttachDependencies(nil, nil, nil, dlSvc, nil, nil, nil)

	channel, _ := dlSvc.CreateChannel(context.Background(), datalinkdomain.Channel{
		AccountID: acct.ID,
		Name:      "orders",
		Endpoint:  "https://example.com",
		AuthToken: "token",
		Status:    datalinkdomain.ChannelStatusActive,
		SignerSet: []string{"nwallet"},
	})

	fn, _ := fnSvc.Create(context.Background(), function.Definition{AccountID: acct.ID, Name: "datalink", Source: "() => 1"})

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
						"channel_id": channel.ID,
						"payload":    map[string]any{"value": "abc"},
						"metadata":   map[string]string{"trace": "1"},
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
	if result.Actions[0].Status != function.ActionStatusSucceeded {
		t.Fatalf("expected action success, got %s: %s", result.Actions[0].Status, result.Actions[0].Error)
	}
}
