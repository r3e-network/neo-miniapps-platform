package integration

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/pkg/logger"

	// Service packages with testing utilities
	automation "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation/service"
	lottery "github.com/R3E-Network/service_layer/packages/com.r3e.services.lottery/service"
	oracle "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle/service"
)

// TestServiceRegistry tests that services can be instantiated with their test utilities.
func TestServiceRegistry(t *testing.T) {
	log := logger.NewDefault("integration-test")

	// Create mock account checker from oracle package (shared pattern)
	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("test-account")

	t.Run("OracleServiceInstantiate", func(t *testing.T) {
		store := oracle.NewMemoryStore()
		svc := oracle.New(mockAccounts, store, log)
		if svc == nil {
			t.Error("Oracle service is nil")
		}
		if svc.Domain() != "oracle" {
			t.Errorf("Expected domain 'oracle', got %s", svc.Domain())
		}
	})

	t.Run("AutomationServiceInstantiate", func(t *testing.T) {
		store := automation.NewMemoryStore()
		svc := automation.New(mockAccounts, store, log)
		if svc == nil {
			t.Error("Automation service is nil")
		}
		if svc.Domain() != "automation" {
			t.Errorf("Expected domain 'automation', got %s", svc.Domain())
		}
	})

	t.Run("LotteryServiceInstantiate", func(t *testing.T) {
		lotteryAccounts := lottery.NewMockAccountChecker()
		lotteryAccounts.AddAccount("test-account")
		store := lottery.NewMemoryStore()
		svc := lottery.New(lotteryAccounts, store, log)
		if svc == nil {
			t.Error("Lottery service is nil")
		}
		if svc.Domain() != "lottery" {
			t.Errorf("Expected domain 'lottery', got %s", svc.Domain())
		}
	})
}

// TestLotteryVRFIntegration tests the integration between Lottery and VRF services.
func TestLotteryVRFIntegration(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("lottery-vrf-test")

	// Setup accounts
	mockAccounts := lottery.NewMockAccountChecker()
	mockAccounts.AddAccount("operator")
	mockAccounts.AddAccount("player-1")
	mockAccounts.AddAccount("player-2")

	// Setup Lottery service with mock VRF
	lotteryStore := lottery.NewMemoryStore()
	lotterySvc := lottery.New(mockAccounts, lotteryStore, log)
	mockVRF := lottery.NewMockVRFService()
	lotterySvc.WithVRF(mockVRF)

	t.Run("LotteryWithVRFDraw", func(t *testing.T) {
		// Create lottery config
		config, err := lotterySvc.CreateConfig(ctx, "operator", lottery.LotteryConfig{
			Name:        "VRF Lottery",
			Description: "Lottery with VRF integration",
			VRFKeyID:    "test-vrf-key",
		})
		if err != nil {
			t.Fatalf("CreateConfig failed: %v", err)
		}
		t.Logf("Created lottery config: %s", config.ID)

		// Start round
		round, err := lotterySvc.StartRound(ctx, "operator")
		if err != nil {
			t.Fatalf("StartRound failed: %v", err)
		}

		// Buy tickets
		for i := 0; i < 12; i++ {
			_, err := lotterySvc.QuickPick(ctx, "player-1", round.ID)
			if err != nil {
				t.Fatalf("QuickPick failed: %v", err)
			}
		}

		// Execute draw with VRF
		result, err := lotterySvc.ExecuteAutomatedDraw(ctx, round.ID)
		if err != nil {
			t.Fatalf("ExecuteAutomatedDraw failed: %v", err)
		}

		if len(result.WinningNumbers) != 5 {
			t.Errorf("Expected 5 winning numbers, got %d", len(result.WinningNumbers))
		}
		t.Logf("Draw result: numbers=%v, mega=%d, winners=%d",
			result.WinningNumbers, result.MegaNumber, len(result.Winners))
	})
}

// TestOracleAutomationIntegration tests the integration between Oracle and Automation services.
func TestOracleAutomationIntegration(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-automation-test")

	// Setup accounts
	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("operator")

	// Setup Oracle service
	oracleStore := oracle.NewMemoryStore()
	oracleSvc := oracle.New(mockAccounts, oracleStore, log)

	// Setup Automation service
	autoStore := automation.NewMemoryStore()
	autoSvc := automation.New(mockAccounts, autoStore, log)

	t.Run("CreateOracleSourceForAutomation", func(t *testing.T) {
		// Create oracle data source
		source, err := oracleSvc.CreateSource(ctx, "operator", "price-feed",
			"https://api.example.com/price", "GET", "Price feed for automation", nil, "")
		if err != nil {
			t.Fatalf("CreateSource failed: %v", err)
		}
		t.Logf("Created oracle source: %s", source.ID)

		// Create automation job that uses oracle
		// CreateJob(ctx, accountID, functionID, name, schedule, description)
		job, err := autoSvc.CreateJob(ctx, "operator", source.ID, "price-update", "*/5 * * * *", "Update price from oracle")
		if err != nil {
			t.Fatalf("CreateJob failed: %v", err)
		}
		t.Logf("Created automation job: %s", job.ID)

		// Verify job was created
		jobs, err := autoSvc.ListJobs(ctx, "operator")
		if err != nil {
			t.Fatalf("ListJobs failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Errorf("Expected 1 job, got %d", len(jobs))
		}
	})
}

// TestOracleRequestLifecycle tests the full oracle request lifecycle.
func TestOracleRequestLifecycle(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-lifecycle-test")

	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("operator")

	oracleStore := oracle.NewMemoryStore()
	oracleSvc := oracle.New(mockAccounts, oracleStore, log)

	t.Run("FullRequestLifecycle", func(t *testing.T) {
		// Create source
		source, err := oracleSvc.CreateSource(ctx, "operator", "test-source",
			"https://api.example.com/data", "GET", "", nil, "")
		if err != nil {
			t.Fatalf("CreateSource failed: %v", err)
		}

		// Create request
		req, err := oracleSvc.CreateRequest(ctx, "operator", source.ID, `{"query":"test"}`)
		if err != nil {
			t.Fatalf("CreateRequest failed: %v", err)
		}
		if req.Status != oracle.StatusPending {
			t.Errorf("Expected status %s, got %s", oracle.StatusPending, req.Status)
		}

		// Mark running
		req, err = oracleSvc.MarkRunning(ctx, req.ID)
		if err != nil {
			t.Fatalf("MarkRunning failed: %v", err)
		}
		if req.Status != oracle.StatusRunning {
			t.Errorf("Expected status %s, got %s", oracle.StatusRunning, req.Status)
		}

		// Complete request
		req, err = oracleSvc.CompleteRequest(ctx, req.ID, `{"result":"success"}`)
		if err != nil {
			t.Fatalf("CompleteRequest failed: %v", err)
		}
		if req.Status != oracle.StatusSucceeded {
			t.Errorf("Expected status %s, got %s", oracle.StatusSucceeded, req.Status)
		}

		t.Logf("Request lifecycle completed: %s -> %s", req.ID, req.Status)
	})
}

// TestCrossServiceAccountValidation tests that services properly validate accounts.
func TestCrossServiceAccountValidation(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("account-validation-test")

	// Setup mock accounts - only "valid-account" exists
	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("valid-account")

	invalidAccountID := "nonexistent-account"

	t.Run("OracleRejectsInvalidAccount", func(t *testing.T) {
		oracleStore := oracle.NewMemoryStore()
		oracleSvc := oracle.New(mockAccounts, oracleStore, log)

		_, err := oracleSvc.CreateSource(ctx, invalidAccountID, "test", "https://api.example.com", "GET", "", nil, "")
		if err == nil {
			t.Error("Expected error for invalid account")
		}
	})

	t.Run("AutomationRejectsInvalidAccount", func(t *testing.T) {
		autoStore := automation.NewMemoryStore()
		autoSvc := automation.New(mockAccounts, autoStore, log)

		_, err := autoSvc.CreateJob(ctx, invalidAccountID, "func-id", "test", "* * * * *", "")
		if err == nil {
			t.Error("Expected error for invalid account")
		}
	})

	t.Run("LotteryRejectsInvalidAccount", func(t *testing.T) {
		lotteryAccounts := lottery.NewMockAccountChecker()
		lotteryAccounts.AddAccount("valid-account")
		lotteryStore := lottery.NewMemoryStore()
		lotterySvc := lottery.New(lotteryAccounts, lotteryStore, log)

		_, err := lotterySvc.CreateConfig(ctx, invalidAccountID, lottery.LotteryConfig{
			Name: "test",
		})
		if err == nil {
			t.Error("Expected error for invalid account")
		}
	})
}

// TestAutomationJobLifecycle tests the full automation job lifecycle.
func TestAutomationJobLifecycle(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("automation-lifecycle-test")

	mockAccounts := oracle.NewMockAccountChecker()
	mockAccounts.AddAccount("operator")

	autoStore := automation.NewMemoryStore()
	autoSvc := automation.New(mockAccounts, autoStore, log)

	t.Run("CreateAndManageJob", func(t *testing.T) {
		// Create job: CreateJob(ctx, accountID, functionID, name, schedule, description)
		job, err := autoSvc.CreateJob(ctx, "operator", "test-function", "test-job", "0 * * * *", "Test automation job")
		if err != nil {
			t.Fatalf("CreateJob failed: %v", err)
		}
		t.Logf("Created job: %s", job.ID)

		// Get job
		retrieved, err := autoSvc.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob failed: %v", err)
		}
		if retrieved.Name != "test-job" {
			t.Errorf("Expected name 'test-job', got %s", retrieved.Name)
		}

		// Disable job using SetEnabled
		disabled, err := autoSvc.SetEnabled(ctx, job.ID, false)
		if err != nil {
			t.Fatalf("SetEnabled failed: %v", err)
		}
		if disabled.Enabled {
			t.Error("Job should be disabled")
		}

		// List jobs
		jobs, err := autoSvc.ListJobs(ctx, "operator")
		if err != nil {
			t.Fatalf("ListJobs failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Errorf("Expected 1 job, got %d", len(jobs))
		}
	})
}
