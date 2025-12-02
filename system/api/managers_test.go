package api

import (
	"testing"
	"time"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Unit tests for helper functions and data types.
// Database integration tests require PostgreSQL and are in a separate file.

func TestNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantValid bool
	}{
		{"empty string", "", false},
		{"non-empty string", "test", true},
		{"whitespace", " ", true},
		{"special chars", "!@#$%", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := core.ToNullString(tt.input)
			if ns.Valid != tt.wantValid {
				t.Errorf("core.ToNullString(%q).Valid = %v, want %v", tt.input, ns.Valid, tt.wantValid)
			}
			if tt.wantValid && ns.String != tt.input {
				t.Errorf("core.ToNullString(%q).String = %q, want %q", tt.input, ns.String, tt.input)
			}
		})
	}
}

func TestGenerateAccountID(t *testing.T) {
	id1 := generateAccountID("owner1")
	id2 := generateAccountID("owner2")

	// Check prefix
	if len(id1) < 4 || id1[:4] != "acc_" {
		t.Errorf("expected ID to start with 'acc_', got %s", id1)
	}

	// Check uniqueness (different owners)
	if id1 == id2 {
		t.Error("expected different IDs for different owners")
	}

	// Check length (acc_ + 16 hex chars = 20)
	if len(id1) != 20 {
		t.Errorf("expected ID length 20, got %d", len(id1))
	}
}

func TestGenerateAccountID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateAccountID("same-owner")
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true
		time.Sleep(time.Microsecond) // Ensure time difference
	}
}

func TestGenerateContractID(t *testing.T) {
	id := generateContractID("acc_123", "0xabcd")

	// Check prefix
	if len(id) < 4 || id[:4] != "ctr_" {
		t.Errorf("expected ID to start with 'ctr_', got %s", id)
	}

	// Check length (ctr_ + 16 hex chars = 20)
	if len(id) != 20 {
		t.Errorf("expected ID length 20, got %d", len(id))
	}
}

func TestGenerateFunctionID(t *testing.T) {
	id := generateFunctionID("acc_123", "myFunction")

	// Check prefix
	if len(id) < 3 || id[:3] != "fn_" {
		t.Errorf("expected ID to start with 'fn_', got %s", id)
	}

	// Check length (fn_ + 16 hex chars = 19)
	if len(id) != 19 {
		t.Errorf("expected ID length 19, got %d", len(id))
	}
}

func TestGenerateTriggerID(t *testing.T) {
	id := generateTriggerID("fn_123", "cron")

	// Check prefix
	if len(id) < 4 || id[:4] != "trg_" {
		t.Errorf("expected ID to start with 'trg_', got %s", id)
	}

	// Check length (trg_ + 16 hex chars = 20)
	if len(id) != 20 {
		t.Errorf("expected ID length 20, got %d", len(id))
	}
}

// Data type tests

func TestAccount_Fields(t *testing.T) {
	now := time.Now()
	account := Account{
		ID:        "acc_123",
		Owner:     "NeoAddress",
		Metadata:  map[string]string{"key": "value"},
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if account.ID != "acc_123" {
		t.Errorf("expected ID acc_123, got %s", account.ID)
	}
	if account.Owner != "NeoAddress" {
		t.Errorf("expected Owner NeoAddress, got %s", account.Owner)
	}
	if account.Metadata["key"] != "value" {
		t.Errorf("expected Metadata[key] = value, got %s", account.Metadata["key"])
	}
}

func TestWallet_Fields(t *testing.T) {
	now := time.Now()
	wallet := Wallet{
		Address:   "NeoWallet123",
		AccountID: "acc_123",
		Status:    "active",
		LinkedAt:  now,
	}

	if wallet.Address != "NeoWallet123" {
		t.Errorf("expected Address NeoWallet123, got %s", wallet.Address)
	}
	if wallet.AccountID != "acc_123" {
		t.Errorf("expected AccountID acc_123, got %s", wallet.AccountID)
	}
}

func TestSecretInfo_Fields(t *testing.T) {
	now := time.Now()
	secret := SecretInfo{
		Name:      "api_key",
		Encrypted: true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if secret.Name != "api_key" {
		t.Errorf("expected Name api_key, got %s", secret.Name)
	}
	if !secret.Encrypted {
		t.Error("expected Encrypted to be true")
	}
}

func TestContractSpec_Fields(t *testing.T) {
	spec := ContractSpec{
		Name:         "MyContract",
		Description:  "Test contract",
		ScriptHash:   "0x1234567890abcdef",
		Capabilities: []string{"oracle", "vrf"},
		Metadata:     map[string]string{"version": "1.0"},
	}

	if spec.Name != "MyContract" {
		t.Errorf("expected Name MyContract, got %s", spec.Name)
	}
	if len(spec.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(spec.Capabilities))
	}
}

func TestContract_Fields(t *testing.T) {
	now := time.Now()
	contract := Contract{
		ID:           "ctr_123",
		AccountID:    "acc_123",
		Name:         "MyContract",
		ScriptHash:   "0x1234",
		Capabilities: []string{"oracle"},
		Status:       "active",
		Paused:       false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if contract.ID != "ctr_123" {
		t.Errorf("expected ID ctr_123, got %s", contract.ID)
	}
	if contract.Paused {
		t.Error("expected Paused to be false")
	}
}

func TestFunctionSpec_Defaults(t *testing.T) {
	spec := FunctionSpec{
		Name:    "myFunc",
		Runtime: "javascript",
		Code:    "console.log('hello')",
	}

	// Check that optional fields have zero values
	if spec.Timeout != 0 {
		t.Errorf("expected Timeout 0, got %d", spec.Timeout)
	}
	if spec.Memory != 0 {
		t.Errorf("expected Memory 0, got %d", spec.Memory)
	}
	if spec.EntryPoint != "" {
		t.Errorf("expected empty EntryPoint, got %s", spec.EntryPoint)
	}
}

func TestFunction_Fields(t *testing.T) {
	now := time.Now()
	function := Function{
		ID:         "fn_123",
		AccountID:  "acc_123",
		Name:       "myFunc",
		Runtime:    "javascript",
		CodeHash:   "sha256:abc123",
		EntryPoint: "main",
		Timeout:    30,
		Memory:     128,
		Enabled:    true,
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
		LastRunAt:  &now,
	}

	if function.ID != "fn_123" {
		t.Errorf("expected ID fn_123, got %s", function.ID)
	}
	if !function.Enabled {
		t.Error("expected Enabled to be true")
	}
	if function.LastRunAt == nil {
		t.Error("expected LastRunAt to be set")
	}
}

func TestTriggerSpec_Types(t *testing.T) {
	tests := []struct {
		name       string
		triggerType string
		schedule   string
		eventType  string
	}{
		{"cron trigger", "cron", "0 * * * *", ""},
		{"event trigger", "event", "", "OracleRequested"},
		{"webhook trigger", "webhook", "", ""},
		{"manual trigger", "manual", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := TriggerSpec{
				Type:      tt.triggerType,
				Schedule:  tt.schedule,
				EventType: tt.eventType,
				Enabled:   true,
			}

			if spec.Type != tt.triggerType {
				t.Errorf("expected Type %s, got %s", tt.triggerType, spec.Type)
			}
		})
	}
}

func TestTrigger_Fields(t *testing.T) {
	now := time.Now()
	trigger := Trigger{
		ID:          "trg_123",
		FunctionID:  "fn_123",
		Type:        "cron",
		Schedule:    "0 * * * *",
		Enabled:     true,
		Status:      "active",
		Config:      map[string]any{"timezone": "UTC"},
		CreatedAt:   now,
		UpdatedAt:   now,
		LastFiredAt: &now,
	}

	if trigger.ID != "trg_123" {
		t.Errorf("expected ID trg_123, got %s", trigger.ID)
	}
	if trigger.Config["timezone"] != "UTC" {
		t.Errorf("expected Config[timezone] = UTC, got %v", trigger.Config["timezone"])
	}
}

func TestBalance_Fields(t *testing.T) {
	now := time.Now()
	balance := Balance{
		AccountID:      "acc_123",
		Available:      1000000,
		Reserved:       100000,
		TotalDeposited: 2000000,
		TotalWithdrawn: 500000,
		TotalFeesPaid:  400000,
		UpdatedAt:      now,
	}

	if balance.AccountID != "acc_123" {
		t.Errorf("expected AccountID acc_123, got %s", balance.AccountID)
	}
	if balance.Available != 1000000 {
		t.Errorf("expected Available 1000000, got %d", balance.Available)
	}
	// Verify balance equation: Available + Reserved + Withdrawn + Fees = Deposited
	total := balance.Available + balance.Reserved + balance.TotalWithdrawn + balance.TotalFeesPaid
	if total != balance.TotalDeposited {
		t.Errorf("balance equation failed: %d + %d + %d + %d = %d, expected %d",
			balance.Available, balance.Reserved, balance.TotalWithdrawn, balance.TotalFeesPaid,
			total, balance.TotalDeposited)
	}
}

func TestTransaction_Types(t *testing.T) {
	types := []string{"deposit", "withdrawal", "fee", "refund"}
	for _, txType := range types {
		tx := Transaction{
			ID:        "tx_123",
			AccountID: "acc_123",
			Type:      txType,
			Amount:    100000,
		}
		if tx.Type != txType {
			t.Errorf("expected Type %s, got %s", txType, tx.Type)
		}
	}
}

// Benchmark tests

func BenchmarkGenerateAccountID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateAccountID("owner")
	}
}

func BenchmarkGenerateContractID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateContractID("acc_123", "0xabcd")
	}
}

func BenchmarkGenerateFunctionID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateFunctionID("acc_123", "myFunc")
	}
}

func BenchmarkGenerateTriggerID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateTriggerID("fn_123", "cron")
	}
}

func BenchmarkNullString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		core.ToNullString("test-value")
	}
}
