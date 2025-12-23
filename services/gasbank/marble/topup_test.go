package neogasbank

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
)

func TestTopUpConstants(t *testing.T) {
	if TopUpThreshold != 10000000 {
		t.Errorf("TopUpThreshold = %d, want 10000000", TopUpThreshold)
	}
	if TopUpTargetAmount != 100000000 {
		t.Errorf("TopUpTargetAmount = %d, want 100000000", TopUpTargetAmount)
	}
	if TopUpCheckInterval != 5*time.Minute {
		t.Errorf("TopUpCheckInterval = %v, want 5m", TopUpCheckInterval)
	}
	if TopUpBatchSize != 100 {
		t.Errorf("TopUpBatchSize = %d, want 100", TopUpBatchSize)
	}
}

func TestIsAutoTopUpEnabled(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"enabled with true", "true", true},
		{"enabled with 1", "1", true},
		{"disabled with false", "false", false},
		{"disabled with 0", "0", false},
		{"disabled with empty", "", false},
		{"disabled with invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TOPUP_ENABLED", tt.envValue)
			defer os.Unsetenv("TOPUP_ENABLED")

			got := svc.isAutoTopUpEnabled()
			if got != tt.want {
				t.Errorf("isAutoTopUpEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessAutoTopUpNoChainClient(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	// Should not panic with nil chain client
	ctx := context.Background()
	svc.processAutoTopUp(ctx)
}

func TestProcessAutoTopUpDisabled(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	// Ensure TOPUP_ENABLED is not set
	os.Unsetenv("TOPUP_ENABLED")

	ctx := context.Background()
	svc.processAutoTopUp(ctx)
	// Should return early without error
}

func TestGetAccountPoolClient(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	// Test with default URL
	os.Unsetenv("NEOACCOUNTS_SERVICE_URL")
	client, err := svc.getAccountPoolClient()
	if err != nil {
		t.Fatalf("getAccountPoolClient() error = %v", err)
	}
	if client == nil {
		t.Error("getAccountPoolClient() returned nil client")
	}

	// Test with custom URL
	os.Setenv("NEOACCOUNTS_SERVICE_URL", "http://custom:9090")
	defer os.Unsetenv("NEOACCOUNTS_SERVICE_URL")
	client, err = svc.getAccountPoolClient()
	if err != nil {
		t.Fatalf("getAccountPoolClient() error = %v", err)
	}
	if client == nil {
		t.Error("getAccountPoolClient() returned nil client")
	}
}

func TestTopUpAccountSimulated(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	toAddress := "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq"
	amount := int64(90000000) // 0.9 GAS

	txHash, err := svc.topUpAccount(ctx, toAddress, amount)
	if err != nil {
		t.Fatalf("topUpAccount() error = %v", err)
	}
	if txHash == "" {
		t.Error("topUpAccount() returned empty tx hash")
	}
	// Simulated hash should be all zeros
	expectedHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	if txHash != expectedHash {
		t.Errorf("topUpAccount() txHash = %s, want %s", txHash, expectedHash)
	}
}

func TestStatisticsIncludesTopUp(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	os.Setenv("TOPUP_ENABLED", "true")
	defer os.Unsetenv("TOPUP_ENABLED")

	stats := svc.statistics()

	// Check that top-up related stats are present
	if _, ok := stats["topup_enabled"]; !ok {
		t.Error("statistics() missing topup_enabled")
	}
	if _, ok := stats["topup_check_interval"]; !ok {
		t.Error("statistics() missing topup_check_interval")
	}
	if _, ok := stats["topup_threshold"]; !ok {
		t.Error("statistics() missing topup_threshold")
	}
	if _, ok := stats["topup_target_amount"]; !ok {
		t.Error("statistics() missing topup_target_amount")
	}

	// Verify values
	if stats["topup_enabled"] != true {
		t.Errorf("topup_enabled = %v, want true", stats["topup_enabled"])
	}
	if stats["topup_threshold"] != TopUpThreshold {
		t.Errorf("topup_threshold = %v, want %d", stats["topup_threshold"], TopUpThreshold)
	}
	if stats["topup_target_amount"] != TopUpTargetAmount {
		t.Errorf("topup_target_amount = %v, want %d", stats["topup_target_amount"], TopUpTargetAmount)
	}
}

func TestTopUpThresholdAndTarget(t *testing.T) {
	// Verify the relationship between threshold and target
	if TopUpTargetAmount <= TopUpThreshold {
		t.Errorf("TopUpTargetAmount (%d) should be greater than TopUpThreshold (%d)",
			TopUpTargetAmount, TopUpThreshold)
	}

	// Verify reasonable values
	// 0.1 GAS threshold
	if TopUpThreshold != 10000000 {
		t.Errorf("TopUpThreshold should be 0.1 GAS (10000000), got %d", TopUpThreshold)
	}
	// 1 GAS target
	if TopUpTargetAmount != 100000000 {
		t.Errorf("TopUpTargetAmount should be 1 GAS (100000000), got %d", TopUpTargetAmount)
	}
}

func TestTopUpBatchSizeReasonable(t *testing.T) {
	// Verify batch size is reasonable (not too large to avoid timeouts)
	if TopUpBatchSize > 1000 {
		t.Errorf("TopUpBatchSize (%d) is too large, should be <= 1000", TopUpBatchSize)
	}
	if TopUpBatchSize < 1 {
		t.Errorf("TopUpBatchSize (%d) is too small, should be >= 1", TopUpBatchSize)
	}
}

func TestTopUpCheckIntervalReasonable(t *testing.T) {
	// Verify check interval is reasonable (not too frequent)
	if TopUpCheckInterval < time.Minute {
		t.Errorf("TopUpCheckInterval (%v) is too frequent, should be >= 1 minute", TopUpCheckInterval)
	}
	if TopUpCheckInterval > time.Hour {
		t.Errorf("TopUpCheckInterval (%v) is too infrequent, should be <= 1 hour", TopUpCheckInterval)
	}
}

func TestGetAccountPoolClientWithEmptyURL(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	// Test with whitespace-only URL
	os.Setenv("NEOACCOUNTS_SERVICE_URL", "   ")
	defer os.Unsetenv("NEOACCOUNTS_SERVICE_URL")

	client, err := svc.getAccountPoolClient()
	if err != nil {
		t.Fatalf("getAccountPoolClient() error = %v", err)
	}
	if client == nil {
		t.Error("getAccountPoolClient() should return client even with empty URL")
	}
}

func TestTopUpAccountWithEmptyAddress(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	// Empty address should still work in simulated mode
	txHash, err := svc.topUpAccount(ctx, "", 100000000)
	if err != nil {
		t.Fatalf("topUpAccount() error = %v", err)
	}
	if txHash == "" {
		t.Error("topUpAccount() returned empty tx hash")
	}
}

func TestTopUpAccountWithZeroAmount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	toAddress := "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq"
	// Zero amount should still work in simulated mode
	txHash, err := svc.topUpAccount(ctx, toAddress, 0)
	if err != nil {
		t.Fatalf("topUpAccount() error = %v", err)
	}
	if txHash == "" {
		t.Error("topUpAccount() returned empty tx hash")
	}
}

func TestTopUpAccountWithNegativeAmount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	toAddress := "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq"
	// Negative amount should still work in simulated mode (validation would happen in real implementation)
	txHash, err := svc.topUpAccount(ctx, toAddress, -100)
	if err != nil {
		t.Fatalf("topUpAccount() error = %v", err)
	}
	if txHash == "" {
		t.Error("topUpAccount() returned empty tx hash")
	}
}
