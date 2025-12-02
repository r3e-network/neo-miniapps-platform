package oracle

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/pkg/logger"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// APIRequest alias for tests
type APIRequest = core.APIRequest

func TestOracleService_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	// Setup
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")
	accounts.AddAccount("other-account")

	svc := New(accounts, store, log)

	// Test 1: Create data source
	var sourceID string
	t.Run("CreateSource", func(t *testing.T) {
		src, err := svc.CreateSource(ctx, "test-account", "price-feed", "https://api.example.com/price", "GET", "Price feed source", nil, "")
		if err != nil {
			t.Fatalf("CreateSource failed: %v", err)
		}
		sourceID = src.ID
		if src.ID == "" {
			t.Error("Source ID should not be empty")
		}
		if src.Name != "price-feed" {
			t.Errorf("Expected name 'price-feed', got %s", src.Name)
		}
		if !src.Enabled {
			t.Error("Source should be enabled by default")
		}
	})

	// Test 2: Create duplicate source name
	t.Run("CreateSource_DuplicateName", func(t *testing.T) {
		_, err := svc.CreateSource(ctx, "test-account", "price-feed", "https://api.example.com/other", "GET", "", nil, "")
		if err == nil {
			t.Error("Expected error for duplicate source name")
		}
	})

	// Test 3: Create source with invalid account
	t.Run("CreateSource_InvalidAccount", func(t *testing.T) {
		_, err := svc.CreateSource(ctx, "nonexistent", "test", "https://api.example.com", "GET", "", nil, "")
		if err == nil {
			t.Error("Expected error for invalid account")
		}
	})

	// Test 4: Get source
	t.Run("GetSource", func(t *testing.T) {
		src, err := svc.GetSource(ctx, sourceID)
		if err != nil {
			t.Fatalf("GetSource failed: %v", err)
		}
		if src.ID != sourceID {
			t.Errorf("Expected source ID %s, got %s", sourceID, src.ID)
		}
	})

	// Test 5: List sources
	t.Run("ListSources", func(t *testing.T) {
		sources, err := svc.ListSources(ctx, "test-account")
		if err != nil {
			t.Fatalf("ListSources failed: %v", err)
		}
		if len(sources) != 1 {
			t.Errorf("Expected 1 source, got %d", len(sources))
		}
	})

	// Test 6: Create request
	var requestID string
	t.Run("CreateRequest", func(t *testing.T) {
		req, err := svc.CreateRequest(ctx, "test-account", sourceID, `{"symbol":"BTC"}`)
		if err != nil {
			t.Fatalf("CreateRequest failed: %v", err)
		}
		requestID = req.ID
		if req.ID == "" {
			t.Error("Request ID should not be empty")
		}
		if req.Status != StatusPending {
			t.Errorf("Expected status %s, got %s", StatusPending, req.Status)
		}
		if req.DataSourceID != sourceID {
			t.Errorf("Expected source ID %s, got %s", sourceID, req.DataSourceID)
		}
	})

	// Test 7: Get request
	t.Run("GetRequest", func(t *testing.T) {
		req, err := svc.GetRequest(ctx, requestID)
		if err != nil {
			t.Fatalf("GetRequest failed: %v", err)
		}
		if req.ID != requestID {
			t.Errorf("Expected request ID %s, got %s", requestID, req.ID)
		}
	})

	// Test 8: List requests
	t.Run("ListRequests", func(t *testing.T) {
		requests, err := svc.ListRequests(ctx, "test-account", 10, "")
		if err != nil {
			t.Fatalf("ListRequests failed: %v", err)
		}
		if len(requests) != 1 {
			t.Errorf("Expected 1 request, got %d", len(requests))
		}
	})

	// Test 9: List pending requests
	t.Run("ListPending", func(t *testing.T) {
		requests, err := svc.ListPending(ctx)
		if err != nil {
			t.Fatalf("ListPending failed: %v", err)
		}
		if len(requests) != 1 {
			t.Errorf("Expected 1 pending request, got %d", len(requests))
		}
	})

	// Test 10: Complete request (must mark running first)
	t.Run("CompleteRequest", func(t *testing.T) {
		// Mark as running first - oracle requires running status before completion
		_, err := svc.MarkRunning(ctx, requestID)
		if err != nil {
			t.Fatalf("MarkRunning failed: %v", err)
		}
		req, err := svc.CompleteRequest(ctx, requestID, `{"price":50000}`)
		if err != nil {
			t.Fatalf("CompleteRequest failed: %v", err)
		}
		if req.Status != StatusSucceeded {
			t.Errorf("Expected status %s, got %s", StatusSucceeded, req.Status)
		}
		if req.Result != `{"price":50000}` {
			t.Errorf("Expected result %s, got %s", `{"price":50000}`, req.Result)
		}
	})

	// Test 11: Verify no more pending requests
	t.Run("NoPendingAfterComplete", func(t *testing.T) {
		requests, err := svc.ListPending(ctx)
		if err != nil {
			t.Fatalf("ListPending failed: %v", err)
		}
		if len(requests) != 0 {
			t.Errorf("Expected 0 pending requests, got %d", len(requests))
		}
	})

	// Test 12: Update source
	t.Run("UpdateSource", func(t *testing.T) {
		newURL := "https://api.example.com/v2/price"
		src, err := svc.UpdateSource(ctx, sourceID, nil, &newURL, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdateSource failed: %v", err)
		}
		if src.URL != newURL {
			t.Errorf("Expected URL %s, got %s", newURL, src.URL)
		}
	})

	// Test 13: Disable source
	t.Run("DisableSource", func(t *testing.T) {
		src, err := svc.SetSourceEnabled(ctx, sourceID, false)
		if err != nil {
			t.Fatalf("SetSourceEnabled failed: %v", err)
		}
		if src.Enabled {
			t.Error("Source should be disabled")
		}
	})

	// Test 14: Create request for disabled source
	t.Run("CreateRequest_DisabledSource", func(t *testing.T) {
		_, err := svc.CreateRequest(ctx, "test-account", sourceID, `{}`)
		if err == nil {
			t.Error("Expected error for disabled source")
		}
	})
}

func TestOracleService_Validation(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")

	svc := New(accounts, store, log)

	testCases := []struct {
		name      string
		accountID string
		srcName   string
		url       string
		wantErr   bool
	}{
		{"valid", "test-account", "test", "https://api.example.com", false},
		{"empty account", "", "test", "https://api.example.com", true},
		{"empty name", "test-account", "", "https://api.example.com", true},
		{"empty url", "test-account", "test2", "", true},
		{"invalid account", "nonexistent", "test3", "https://api.example.com", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateSource(ctx, tc.accountID, tc.srcName, tc.url, "GET", "", nil, "")
			if (err != nil) != tc.wantErr {
				t.Errorf("CreateSource() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestOracleService_FailedRequest(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")

	svc := New(accounts, store, log)

	// Create source
	src, err := svc.CreateSource(ctx, "test-account", "test-source", "https://api.example.com", "GET", "", nil, "")
	if err != nil {
		t.Fatalf("CreateSource failed: %v", err)
	}

	// Create request
	req, err := svc.CreateRequest(ctx, "test-account", src.ID, `{}`)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	// Complete with error using FailRequest
	req, err = svc.FailRequest(ctx, req.ID, "connection timeout")
	if err != nil {
		t.Fatalf("FailRequest failed: %v", err)
	}

	if req.Status != StatusFailed {
		t.Errorf("Expected status %s, got %s", StatusFailed, req.Status)
	}
	if req.Error != "connection timeout" {
		t.Errorf("Expected error 'connection timeout', got %s", req.Error)
	}
}

func TestOracleService_Publish(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")

	svc := New(accounts, store, log)

	// Create source first
	src, err := svc.CreateSource(ctx, "test-account", "test-source", "https://api.example.com", "GET", "", nil, "")
	if err != nil {
		t.Fatalf("CreateSource failed: %v", err)
	}

	// Test Publish
	t.Run("Publish_Valid", func(t *testing.T) {
		err := svc.Publish(ctx, "request", map[string]any{
			"account_id": "test-account",
			"source_id":  src.ID,
			"payload":    `{"test":true}`,
		})
		if err != nil {
			t.Errorf("Publish failed: %v", err)
		}
	})

	t.Run("Publish_InvalidEvent", func(t *testing.T) {
		err := svc.Publish(ctx, "invalid", map[string]any{})
		if err == nil {
			t.Error("Expected error for invalid event")
		}
	})

	t.Run("Publish_MissingFields", func(t *testing.T) {
		err := svc.Publish(ctx, "request", map[string]any{
			"account_id": "test-account",
		})
		if err == nil {
			t.Error("Expected error for missing source_id")
		}
	})
}

func TestOracleService_HTTPHandlers(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")

	svc := New(accounts, store, log)

	// Create a source for testing
	src, _ := svc.CreateSource(ctx, "test-account", "http-test", "https://api.example.com", "GET", "", nil, "")

	t.Run("HTTPGetSources", func(t *testing.T) {
		result, err := svc.HTTPGetSources(ctx, apiRequest("test-account", nil, nil))
		if err != nil {
			t.Fatalf("HTTPGetSources failed: %v", err)
		}
		sources, ok := result.([]DataSource)
		if !ok {
			t.Fatal("Expected []DataSource result")
		}
		if len(sources) != 1 {
			t.Errorf("Expected 1 source, got %d", len(sources))
		}
	})

	t.Run("HTTPGetSourcesById", func(t *testing.T) {
		result, err := svc.HTTPGetSourcesById(ctx, apiRequest("test-account", map[string]string{"id": src.ID}, nil))
		if err != nil {
			t.Fatalf("HTTPGetSourcesById failed: %v", err)
		}
		source, ok := result.(DataSource)
		if !ok {
			t.Fatal("Expected DataSource result")
		}
		if source.ID != src.ID {
			t.Errorf("Expected source ID %s, got %s", src.ID, source.ID)
		}
	})

	t.Run("HTTPPostRequests", func(t *testing.T) {
		result, err := svc.HTTPPostRequests(ctx, apiRequest("test-account", nil, map[string]any{
			"data_source_id": src.ID,
			"payload":        `{"test":true}`,
		}))
		if err != nil {
			t.Fatalf("HTTPPostRequests failed: %v", err)
		}
		req, ok := result.(Request)
		if !ok {
			t.Fatal("Expected Request result")
		}
		if req.DataSourceID != src.ID {
			t.Errorf("Expected source ID %s, got %s", src.ID, req.DataSourceID)
		}
	})
}

func TestOracleService_ServiceMetadata(t *testing.T) {
	log := logger.NewDefault("oracle-test")
	store := NewMemoryStore()
	accounts := NewMockAccountChecker()

	svc := New(accounts, store, log)

	t.Run("Manifest", func(t *testing.T) {
		manifest := svc.Manifest()
		if manifest.Name != "oracle" {
			t.Errorf("Expected name 'oracle', got %s", manifest.Name)
		}
	})

	t.Run("Domain", func(t *testing.T) {
		domain := svc.Domain()
		if domain != "oracle" {
			t.Errorf("Expected domain 'oracle', got %s", domain)
		}
	})
}

func TestOracleService_MarkRunning(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")

	svc := New(accounts, store, log)

	// Create source and request
	src, _ := svc.CreateSource(ctx, "test-account", "test", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(ctx, "test-account", src.ID, `{}`)

	// Mark as running
	req, err := svc.MarkRunning(ctx, req.ID)
	if err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}
	if req.Status != StatusRunning {
		t.Errorf("Expected status %s, got %s", StatusRunning, req.Status)
	}
}

func TestOracleService_IncrementAttempts(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")

	svc := New(accounts, store, log)

	// Create source and request
	src, _ := svc.CreateSource(ctx, "test-account", "test", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(ctx, "test-account", src.ID, `{}`)

	// Increment attempts
	req, err := svc.IncrementAttempts(ctx, req.ID)
	if err != nil {
		t.Fatalf("IncrementAttempts failed: %v", err)
	}
	if req.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", req.Attempts)
	}

	// Increment again
	req, err = svc.IncrementAttempts(ctx, req.ID)
	if err != nil {
		t.Fatalf("IncrementAttempts failed: %v", err)
	}
	if req.Attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", req.Attempts)
	}
}

func TestOracleService_RetryRequest(t *testing.T) {
	ctx := context.Background()
	log := logger.NewDefault("oracle-test")

	store := NewMemoryStore()
	accounts := NewMockAccountChecker()
	accounts.AddAccount("test-account")

	svc := New(accounts, store, log)

	// Create source and request
	src, _ := svc.CreateSource(ctx, "test-account", "test", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(ctx, "test-account", src.ID, `{}`)

	// Fail the request
	req, _ = svc.FailRequest(ctx, req.ID, "timeout")

	// Retry
	req, err := svc.RetryRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("RetryRequest failed: %v", err)
	}
	if req.Status != StatusPending {
		t.Errorf("Expected status %s, got %s", StatusPending, req.Status)
	}
}

// Helper function to create APIRequest
func apiRequest(accountID string, pathParams map[string]string, body map[string]any) core.APIRequest {
	if pathParams == nil {
		pathParams = make(map[string]string)
	}
	if body == nil {
		body = make(map[string]any)
	}
	return core.APIRequest{
		AccountID:  accountID,
		PathParams: pathParams,
		Query:      make(map[string]string),
		Body:       body,
	}
}
