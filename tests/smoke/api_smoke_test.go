package smoke

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/pkg/logger"
	core "github.com/R3E-Network/service_layer/system/framework/core"

	// Service packages with HTTP handlers
	automation "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation/service"
	lottery "github.com/R3E-Network/service_layer/packages/com.r3e.services.lottery/service"
	oracle "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle/service"
)

// apiRequest creates a test APIRequest with the given parameters.
func apiRequest(accountID string, pathParams map[string]string, query map[string]string, body map[string]any) core.APIRequest {
	if pathParams == nil {
		pathParams = make(map[string]string)
	}
	if query == nil {
		query = make(map[string]string)
	}
	if body == nil {
		body = make(map[string]any)
	}
	return core.APIRequest{
		AccountID:  accountID,
		PathParams: pathParams,
		Query:      query,
		Body:       body,
	}
}

// TestOracleHTTPEndpoints tests Oracle service HTTP handlers.
func TestOracleHTTPEndpoints(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-smoke")

	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("test-account")

	store := oracle.NewMemoryStore()
	svc := oracle.New(mockAccounts, store, log)

	t.Run("HTTPGetSources_Empty", func(t *testing.T) {
		result, err := svc.HTTPGetSources(ctx, apiRequest("test-account", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetSources failed: %v", err)
		}
		sources, ok := result.([]oracle.DataSource)
		if !ok {
			t.Fatal("Expected []DataSource result")
		}
		if len(sources) != 0 {
			t.Errorf("Expected 0 sources, got %d", len(sources))
		}
	})

	t.Run("HTTPPostSources", func(t *testing.T) {
		result, err := svc.HTTPPostSources(ctx, apiRequest("test-account", nil, nil, map[string]any{
			"name":        "test-source",
			"url":         "https://api.example.com/data",
			"method":      "GET",
			"description": "Test data source",
		}))
		if err != nil {
			t.Fatalf("HTTPPostSources failed: %v", err)
		}
		source, ok := result.(oracle.DataSource)
		if !ok {
			t.Fatal("Expected DataSource result")
		}
		if source.Name != "test-source" {
			t.Errorf("Expected name 'test-source', got %s", source.Name)
		}
	})

	t.Run("HTTPGetSources_WithData", func(t *testing.T) {
		result, err := svc.HTTPGetSources(ctx, apiRequest("test-account", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetSources failed: %v", err)
		}
		sources, ok := result.([]oracle.DataSource)
		if !ok {
			t.Fatal("Expected []DataSource result")
		}
		if len(sources) != 1 {
			t.Errorf("Expected 1 source, got %d", len(sources))
		}
	})
}

// TestAutomationHTTPEndpoints tests Automation service HTTP handlers.
func TestAutomationHTTPEndpoints(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("automation-smoke")

	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("test-account")

	store := automation.NewMemoryStore()
	svc := automation.New(mockAccounts, store, log)

	t.Run("HTTPGetJobs_Empty", func(t *testing.T) {
		result, err := svc.HTTPGetJobs(ctx, apiRequest("test-account", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetJobs failed: %v", err)
		}
		jobs, ok := result.([]automation.Job)
		if !ok {
			t.Fatal("Expected []Job result")
		}
		if len(jobs) != 0 {
			t.Errorf("Expected 0 jobs, got %d", len(jobs))
		}
	})

	t.Run("HTTPPostJobs", func(t *testing.T) {
		result, err := svc.HTTPPostJobs(ctx, apiRequest("test-account", nil, nil, map[string]any{
			"function_id": "test-function",
			"name":        "test-job",
			"schedule":    "0 * * * *",
			"description": "Test automation job",
		}))
		if err != nil {
			t.Fatalf("HTTPPostJobs failed: %v", err)
		}
		job, ok := result.(automation.Job)
		if !ok {
			t.Fatal("Expected Job result")
		}
		if job.Name != "test-job" {
			t.Errorf("Expected name 'test-job', got %s", job.Name)
		}
	})

	t.Run("HTTPGetJobs_WithData", func(t *testing.T) {
		result, err := svc.HTTPGetJobs(ctx, apiRequest("test-account", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetJobs failed: %v", err)
		}
		jobs, ok := result.([]automation.Job)
		if !ok {
			t.Fatal("Expected []Job result")
		}
		if len(jobs) != 1 {
			t.Errorf("Expected 1 job, got %d", len(jobs))
		}
	})
}

// TestLotteryHTTPEndpoints tests Lottery service HTTP handlers.
func TestLotteryHTTPEndpoints(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("lottery-smoke")

	mockAccounts := lottery.NewMockAccountChecker()
	mockAccounts.AddAccount("operator")
	mockAccounts.AddAccount("player")

	store := lottery.NewMemoryStore()
	svc := lottery.New(mockAccounts, store, log)
	mockVRF := lottery.NewMockVRFService()
	svc.WithVRF(mockVRF)

	// First create config
	_, err := svc.CreateConfig(ctx, "operator", lottery.LotteryConfig{
		Name:        "Smoke Test Lottery",
		Description: "Lottery for smoke testing",
		VRFKeyID:    "test-vrf-key",
	})
	if err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	t.Run("HTTPGetRounds_Empty", func(t *testing.T) {
		result, err := svc.HTTPGetRounds(ctx, apiRequest("operator", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetRounds failed: %v", err)
		}
		rounds, ok := result.([]lottery.Round)
		if !ok {
			t.Fatal("Expected []Round result")
		}
		if len(rounds) != 0 {
			t.Errorf("Expected 0 rounds, got %d", len(rounds))
		}
	})

	var roundID string
	t.Run("HTTPPostRounds", func(t *testing.T) {
		result, err := svc.HTTPPostRounds(ctx, apiRequest("operator", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPPostRounds failed: %v", err)
		}
		round, ok := result.(lottery.Round)
		if !ok {
			t.Fatal("Expected Round result")
		}
		roundID = round.ID
		if round.Status != lottery.RoundStatusActive {
			t.Errorf("Expected status %s, got %s", lottery.RoundStatusActive, round.Status)
		}
	})

	t.Run("HTTPGetRoundsActive", func(t *testing.T) {
		result, err := svc.HTTPGetRoundsActive(ctx, apiRequest("operator", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetRoundsActive failed: %v", err)
		}
		round, ok := result.(lottery.Round)
		if !ok {
			t.Fatal("Expected Round result")
		}
		if round.ID != roundID {
			t.Errorf("Expected round ID %s, got %s", roundID, round.ID)
		}
	})

	t.Run("HTTPPostTickets", func(t *testing.T) {
		// Numbers must be float64 (JSON number type) for proper type assertion
		result, err := svc.HTTPPostTickets(ctx, apiRequest("player", nil, nil, map[string]any{
			"round_id":    roundID,
			"numbers":     []any{float64(5), float64(12), float64(23), float64(45), float64(67)},
			"mega_number": float64(15),
		}))
		if err != nil {
			t.Fatalf("HTTPPostTickets failed: %v", err)
		}
		ticket, ok := result.(lottery.Ticket)
		if !ok {
			t.Fatal("Expected Ticket result")
		}
		if ticket.AccountID != "player" {
			t.Errorf("Expected account 'player', got %s", ticket.AccountID)
		}
	})

	t.Run("HTTPGetTickets", func(t *testing.T) {
		result, err := svc.HTTPGetTickets(ctx, apiRequest("player", nil, map[string]string{"round_id": roundID}, nil))
		if err != nil {
			t.Fatalf("HTTPGetTickets failed: %v", err)
		}
		tickets, ok := result.([]lottery.Ticket)
		if !ok {
			t.Fatal("Expected []Ticket result")
		}
		if len(tickets) != 1 {
			t.Errorf("Expected 1 ticket, got %d", len(tickets))
		}
	})

	t.Run("HTTPGetStats", func(t *testing.T) {
		result, err := svc.HTTPGetStats(ctx, apiRequest("operator", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetStats failed: %v", err)
		}
		stats, ok := result.(lottery.LotteryStats)
		if !ok {
			t.Fatal("Expected LotteryStats result")
		}
		if stats.TotalRounds != 1 {
			t.Errorf("Expected 1 round, got %d", stats.TotalRounds)
		}
	})
}

// TestHTTPEndpointErrorHandling tests error handling in HTTP endpoints.
func TestHTTPEndpointErrorHandling(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("error-smoke")

	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("valid-account")

	t.Run("OracleInvalidAccount", func(t *testing.T) {
		store := oracle.NewMemoryStore()
		svc := oracle.New(mockAccounts, store, log)

		_, err := svc.HTTPPostSources(ctx, apiRequest("invalid-account", nil, nil, map[string]any{
			"name":   "test",
			"url":    "https://api.example.com",
			"method": "GET",
		}))
		if err == nil {
			t.Error("Expected error for invalid account")
		}
	})

	t.Run("OracleMissingRequiredFields", func(t *testing.T) {
		store := oracle.NewMemoryStore()
		svc := oracle.New(mockAccounts, store, log)

		_, err := svc.HTTPPostSources(ctx, apiRequest("valid-account", nil, nil, map[string]any{
			"name": "test",
			// Missing url and method
		}))
		if err == nil {
			t.Error("Expected error for missing required fields")
		}
	})

	t.Run("AutomationInvalidAccount", func(t *testing.T) {
		store := automation.NewMemoryStore()
		svc := automation.New(mockAccounts, store, log)

		_, err := svc.HTTPPostJobs(ctx, apiRequest("invalid-account", nil, nil, map[string]any{
			"function_id": "test",
			"name":        "test",
			"schedule":    "* * * * *",
		}))
		if err == nil {
			t.Error("Expected error for invalid account")
		}
	})

	t.Run("LotteryInvalidAccount", func(t *testing.T) {
		lotteryAccounts := lottery.NewMockAccountChecker()
		lotteryAccounts.AddAccount("valid-account")
		store := lottery.NewMemoryStore()
		svc := lottery.New(lotteryAccounts, store, log)

		_, err := svc.HTTPPostRounds(ctx, apiRequest("invalid-account", nil, nil, nil))
		if err == nil {
			t.Error("Expected error for invalid account")
		}
	})
}

// TestHTTPEndpointResponseTypes verifies that HTTP endpoints return correct types.
func TestHTTPEndpointResponseTypes(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("type-smoke")

	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("test-account")

	t.Run("OracleListReturnsSlice", func(t *testing.T) {
		store := oracle.NewMemoryStore()
		svc := oracle.New(mockAccounts, store, log)

		result, err := svc.HTTPGetSources(ctx, apiRequest("test-account", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetSources failed: %v", err)
		}

		// Verify it's a slice type
		switch result.(type) {
		case []oracle.DataSource:
			// Expected type
		default:
			t.Errorf("Expected []DataSource, got %T", result)
		}
	})

	t.Run("AutomationListReturnsSlice", func(t *testing.T) {
		store := automation.NewMemoryStore()
		svc := automation.New(mockAccounts, store, log)

		result, err := svc.HTTPGetJobs(ctx, apiRequest("test-account", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetJobs failed: %v", err)
		}

		// Verify it's a slice type
		switch result.(type) {
		case []automation.Job:
			// Expected type
		default:
			t.Errorf("Expected []Job, got %T", result)
		}
	})

	t.Run("LotteryListReturnsSlice", func(t *testing.T) {
		lotteryAccounts := lottery.NewMockAccountChecker()
		lotteryAccounts.AddAccount("test-account")
		store := lottery.NewMemoryStore()
		svc := lottery.New(lotteryAccounts, store, log)

		// Create config first
		svc.CreateConfig(ctx, "test-account", lottery.LotteryConfig{
			Name:     "Test",
			VRFKeyID: "test-key",
		})

		result, err := svc.HTTPGetRounds(ctx, apiRequest("test-account", nil, nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetRounds failed: %v", err)
		}

		// Verify it's a slice type
		switch result.(type) {
		case []lottery.Round:
			// Expected type
		default:
			t.Errorf("Expected []Round, got %T", result)
		}
	})
}
