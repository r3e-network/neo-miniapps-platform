package oracle

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/storage/memory"
)

func TestService_SourceAndRequestLifecycle(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, NewStoreAdapter(store), nil)
	src, err := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "get", "desc", map[string]string{"X-API": "key"}, "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	if _, err := svc.CreateSource(context.Background(), acct.ID, "prices", "https://other", "GET", "", nil, ""); err == nil {
		t.Fatalf("expected duplicate source error")
	}

	newName := "prices-v2"
	newURL := "https://api2.example.com"
	if _, err := svc.UpdateSource(context.Background(), src.ID, &newName, &newURL, nil, nil, nil, nil); err != nil {
		t.Fatalf("update source: %v", err)
	}

	req, err := svc.CreateRequest(context.Background(), acct.ID, src.ID, `{"pair":"NEO/USD"}`)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	if _, err := svc.CompleteRequest(context.Background(), req.ID, `{"price":12.3}`); err == nil {
		t.Fatalf("expected error completing request that was not running")
	}

	if _, err := svc.MarkRunning(context.Background(), req.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if _, err := svc.CompleteRequest(context.Background(), req.ID, `{"price":12.3}`); err != nil {
		t.Fatalf("complete request: %v", err)
	}

	if _, err := svc.FailRequest(context.Background(), req.ID, "should not overwrite success"); err == nil {
		t.Fatalf("expected error failing completed request")
	}

	requests, err := svc.ListRequests(context.Background(), acct.ID, 10, "")
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %s", requests[0].Status)
	}

	if _, err := svc.SetSourceEnabled(context.Background(), src.ID, false); err != nil {
		t.Fatalf("disable source: %v", err)
	}
	if _, err := svc.CreateRequest(context.Background(), acct.ID, src.ID, `{}`); err == nil {
		t.Fatalf("expected error creating request with disabled source")
	}
}

func TestService_RequestStatusValidation(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, NewStoreAdapter(store), nil)
	src, err := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "get", "desc", nil, "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	req, err := svc.CreateRequest(context.Background(), acct.ID, src.ID, `{}`)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	if _, err := svc.MarkRunning(context.Background(), req.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if _, err := svc.MarkRunning(context.Background(), req.ID); err == nil {
		t.Fatalf("expected error re-marking running request")
	}

	if _, err := svc.FailRequest(context.Background(), req.ID, "boom"); err != nil {
		t.Fatalf("fail request: %v", err)
	}

	if _, err := svc.CompleteRequest(context.Background(), req.ID, `{}`); err == nil {
		t.Fatalf("expected error completing failed request")
	}

	// retry should only work from failed -> pending
	if _, err := svc.RetryRequest(context.Background(), req.ID); err != nil {
		t.Fatalf("retry failed request: %v", err)
	}
	retried, _ := svc.GetRequest(context.Background(), req.ID)
	if retried.Status != StatusPending || retried.Attempts != 0 || retried.Error != "" {
		t.Fatalf("expected reset pending state, got %+v", retried)
	}
	if _, err := svc.MarkRunning(context.Background(), req.ID); err != nil {
		t.Fatalf("mark running after retry: %v", err)
	}
	if _, err := svc.RetryRequest(context.Background(), req.ID); err == nil {
		t.Fatalf("expected retry to fail when status=running")
	}
}

func ExampleService_CreateRequest() {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "oracle-user"})
	log := logger.NewDefault("example-oracle")
	log.SetOutput(io.Discard)
	svc := New(store, NewStoreAdapter(store), log)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")
	fmt.Println(req.Status)
	// Output:
	// pending
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "oracle" {
		t.Fatalf("expected name oracle, got %s", m.Name)
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "oracle" {
		t.Fatalf("expected name oracle, got %s", d.Name)
	}
}

func TestService_Domain(t *testing.T) {
	svc := New(nil, nil, nil)
	if svc.Domain() != "oracle" {
		t.Fatalf("expected domain oracle")
	}
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

func TestService_ListSources(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	svc.CreateSource(context.Background(), acct.ID, "src1", "https://example.com", "GET", "", nil, "")

	sources, err := svc.ListSources(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
}

func TestService_ListPending(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")
	svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	pending, err := svc.ListPending(context.Background())
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
}

func TestService_IncrementAttempts(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	updated, err := svc.IncrementAttempts(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("increment attempts: %v", err)
	}
	if updated.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", updated.Attempts)
	}
}

func TestService_Publish_UnsupportedEvent(t *testing.T) {
	svc := New(nil, nil, nil)
	err := svc.Publish(context.Background(), "unknown", nil)
	if err == nil {
		t.Fatalf("expected error for unsupported event")
	}
}

func TestService_Publish_Request(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")

	payload := map[string]any{
		"account_id": acct.ID,
		"source_id":  src.ID,
		"payload":    "{}",
	}
	err := svc.Publish(context.Background(), "request", payload)
	if err != nil {
		t.Fatalf("publish request: %v", err)
	}
}

func TestService_WithFeeCollector(t *testing.T) {
	svc := New(nil, nil, nil, WithFeeCollector(nil))
	if svc == nil {
		t.Fatalf("expected service")
	}
}

func TestService_WithDefaultFee(t *testing.T) {
	svc := New(nil, nil, nil, WithDefaultFee(100))
	if svc == nil {
		t.Fatalf("expected service")
	}
}

func TestService_CreateSource_MissingAccount(t *testing.T) {
	store := memory.New()
	svc := New(store, NewStoreAdapter(store), nil)
	_, err := svc.CreateSource(context.Background(), "nonexistent", "src", "https://example.com", "GET", "", nil, "")
	if err == nil {
		t.Fatalf("expected account error")
	}
}

func TestService_CreateRequest_MissingAccount(t *testing.T) {
	store := memory.New()
	svc := New(store, NewStoreAdapter(store), nil)
	_, err := svc.CreateRequest(context.Background(), "nonexistent", "src", "{}")
	if err == nil {
		t.Fatalf("expected account error")
	}
}

func TestService_CreateRequest_SourceNotFound(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	_, err := svc.CreateRequest(context.Background(), acct.ID, "nonexistent", "{}")
	if err == nil {
		t.Fatalf("expected source not found error")
	}
}

func TestService_ListSources_MissingAccount(t *testing.T) {
	store := memory.New()
	svc := New(store, NewStoreAdapter(store), nil)
	_, err := svc.ListSources(context.Background(), "nonexistent")
	if err == nil {
		t.Fatalf("expected account error")
	}
}

func TestService_ListRequests_Empty(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	reqs, err := svc.ListRequests(context.Background(), acct.ID, 10, "")
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(reqs) != 0 {
		t.Fatalf("expected empty list")
	}
}

func TestService_CreateRequestWithOptions(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")

	fee := int64(100)
	req, err := svc.CreateRequestWithOptions(context.Background(), acct.ID, src.ID, "{}", CreateRequestOptions{
		Fee: &fee,
	})
	if err != nil {
		t.Fatalf("create request with options: %v", err)
	}
	if req.ID == "" {
		t.Fatalf("expected request ID")
	}
}

func TestService_FailRequestWithOptions(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")
	svc.MarkRunning(context.Background(), req.ID)

	_, err := svc.FailRequestWithOptions(context.Background(), req.ID, "test failure", FailRequestOptions{
		RefundFee: true,
	})
	if err != nil {
		t.Fatalf("fail request with options: %v", err)
	}
}

func TestRequestResolverFunc_Nil(t *testing.T) {
	var f RequestResolverFunc = nil
	done, success, result, errMsg, retryAfter, err := f.Resolve(context.Background(), Request{})
	if done || success || result != "" || errMsg != "" || retryAfter != 0 || err != nil {
		t.Fatalf("expected zero values for nil resolver")
	}
}

func TestTimeoutResolver_New(t *testing.T) {
	// Test default timeout
	r := NewTimeoutResolver(0)
	if r == nil {
		t.Fatalf("expected resolver")
	}
	// Test custom timeout
	r2 := NewTimeoutResolver(5 * time.Minute)
	if r2 == nil {
		t.Fatalf("expected resolver")
	}
}

func TestTimeoutResolver_Resolve(t *testing.T) {
	r := NewTimeoutResolver(50 * time.Millisecond)

	// Test already succeeded
	done, success, _, _, _, _ := r.Resolve(context.Background(), Request{Status: StatusSucceeded, Result: "ok"})
	if !done || !success {
		t.Fatalf("expected done and success for succeeded request")
	}

	// Test already failed
	done, success, _, _, _, _ = r.Resolve(context.Background(), Request{Status: StatusFailed, Error: "err"})
	if !done || success {
		t.Fatalf("expected done and not success for failed request")
	}

	// Test pending - first call stores it
	req := Request{ID: "test-req", Status: StatusPending}
	done, _, _, _, retryAfter, _ := r.Resolve(context.Background(), req)
	if done {
		t.Fatalf("expected not done on first call")
	}
	if retryAfter == 0 {
		t.Fatalf("expected retry after")
	}

	// Test pending - before timeout
	done, _, _, _, _, _ = r.Resolve(context.Background(), req)
	if done {
		t.Fatalf("expected not done before timeout")
	}

	// Wait for timeout and test again
	time.Sleep(60 * time.Millisecond)
	done, success, _, errMsg, _, _ := r.Resolve(context.Background(), req)
	if !done {
		t.Fatalf("expected done after timeout")
	}
	if success {
		t.Fatalf("expected not success after timeout")
	}
	if errMsg == "" {
		t.Fatalf("expected error message")
	}
}

func TestService_UpdateSource_Validation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")

	// Test empty name
	emptyName := "  "
	if _, err := svc.UpdateSource(context.Background(), src.ID, &emptyName, nil, nil, nil, nil, nil); err == nil {
		t.Fatalf("expected error for empty name")
	}

	// Test empty URL
	emptyURL := ""
	if _, err := svc.UpdateSource(context.Background(), src.ID, nil, &emptyURL, nil, nil, nil, nil); err == nil {
		t.Fatalf("expected error for empty url")
	}

	// Test empty method
	emptyMethod := "  "
	if _, err := svc.UpdateSource(context.Background(), src.ID, nil, nil, &emptyMethod, nil, nil, nil); err == nil {
		t.Fatalf("expected error for empty method")
	}

	// Test duplicate name
	svc.CreateSource(context.Background(), acct.ID, "other-source", "https://other.com", "GET", "", nil, "")
	dupName := "other-source"
	if _, err := svc.UpdateSource(context.Background(), src.ID, &dupName, nil, nil, nil, nil, nil); err == nil {
		t.Fatalf("expected error for duplicate name")
	}

	// Test nonexistent source
	if _, err := svc.UpdateSource(context.Background(), "nonexistent", nil, nil, nil, nil, nil, nil); err == nil {
		t.Fatalf("expected error for nonexistent source")
	}

	// Test description, headers, body updates
	newDesc := "new description"
	newBody := "{\"key\": \"value\"}"
	newHeaders := map[string]string{"X-Custom": "header"}
	updated, err := svc.UpdateSource(context.Background(), src.ID, nil, nil, nil, &newDesc, newHeaders, &newBody)
	if err != nil {
		t.Fatalf("update source: %v", err)
	}
	if updated.Description != newDesc {
		t.Fatalf("expected updated description")
	}
	if updated.Body != newBody {
		t.Fatalf("expected updated body")
	}
	if updated.Headers["X-Custom"] != "header" {
		t.Fatalf("expected updated headers")
	}
}

func TestService_SetSourceEnabled_AlreadyEnabled(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")

	// Source is enabled by default, try to enable again
	result, err := svc.SetSourceEnabled(context.Background(), src.ID, true)
	if err != nil {
		t.Fatalf("set source enabled: %v", err)
	}
	// Should return unchanged source
	if result.ID != src.ID {
		t.Fatalf("expected same source")
	}
}

func TestService_Publish_InvalidPayload(t *testing.T) {
	svc := New(nil, nil, nil)

	// Test invalid payload type
	err := svc.Publish(context.Background(), "request", "not a map")
	if err == nil {
		t.Fatalf("expected error for invalid payload type")
	}

	// Test missing account_id/source_id
	err = svc.Publish(context.Background(), "request", map[string]any{})
	if err == nil {
		t.Fatalf("expected error for missing fields")
	}

	// Test payload as map
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc2 := New(store, NewStoreAdapter(store), nil)
	src, _ := svc2.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")
	err = svc2.Publish(context.Background(), "request", map[string]any{
		"account_id": acct.ID,
		"source_id":  src.ID,
		"payload":    map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("publish with map payload: %v", err)
	}
}

func TestService_CompleteRequest_AlreadyCompleted(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	svc.MarkRunning(context.Background(), req.ID)
	svc.CompleteRequest(context.Background(), req.ID, "result")

	// Try to complete again
	_, err := svc.CompleteRequest(context.Background(), req.ID, "result2")
	if err == nil {
		t.Fatalf("expected error completing already completed request")
	}
}

func TestService_FailRequest_AlreadyFailed(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	svc.MarkRunning(context.Background(), req.ID)
	svc.FailRequest(context.Background(), req.ID, "error")

	// Try to fail again
	_, err := svc.FailRequest(context.Background(), req.ID, "error2")
	if err == nil {
		t.Fatalf("expected error failing already failed request")
	}
}

func TestService_RetryRequest_FromRunning(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	svc.MarkRunning(context.Background(), req.ID)

	// Try to retry from running status
	_, err := svc.RetryRequest(context.Background(), req.ID)
	if err == nil {
		t.Fatalf("expected error retrying from running status")
	}
}

func TestService_CreateSource_Validation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)

	// Test empty account_id
	_, err := svc.CreateSource(context.Background(), "", "name", "https://example.com", "GET", "", nil, "")
	if err == nil {
		t.Fatalf("expected error for empty account_id")
	}

	// Test empty name
	_, err = svc.CreateSource(context.Background(), acct.ID, "", "https://example.com", "GET", "", nil, "")
	if err == nil {
		t.Fatalf("expected error for empty name")
	}

	// Test empty url
	_, err = svc.CreateSource(context.Background(), acct.ID, "name", "", "GET", "", nil, "")
	if err == nil {
		t.Fatalf("expected error for empty url")
	}

	// Test empty method defaults to GET (not an error)
	src, err := svc.CreateSource(context.Background(), acct.ID, "name-default", "https://example.com", "", "", nil, "")
	if err != nil {
		t.Fatalf("empty method should default to GET: %v", err)
	}
	if src.Method != "GET" {
		t.Fatalf("expected method GET, got %s", src.Method)
	}
}

func TestService_FailRequestWithOptions_RefundFee(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	// Create service with a mock fee collector that tracks refunds
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "fee-src", "https://example.com", "GET", "", nil, "")

	// Create request with a fee
	fee := int64(100)
	req, _ := svc.CreateRequestWithOptions(context.Background(), acct.ID, src.ID, "{}", CreateRequestOptions{
		Fee: &fee,
	})
	svc.MarkRunning(context.Background(), req.ID)

	// Fail with refund - without fee collector set, just covers the code path
	_, err := svc.FailRequestWithOptions(context.Background(), req.ID, "test failure", FailRequestOptions{
		RefundFee: true,
	})
	if err != nil {
		t.Fatalf("fail request with refund: %v", err)
	}
}

func TestService_FailRequest_FromPending(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "pending-fail-src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	// Fail directly from pending (allowed transition)
	_, err := svc.FailRequest(context.Background(), req.ID, "direct failure from pending")
	if err != nil {
		t.Fatalf("fail from pending: %v", err)
	}
}

func TestService_GetSource(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "get-src", "https://example.com", "GET", "", nil, "")

	retrieved, err := svc.GetSource(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if retrieved.Name != "get-src" {
		t.Fatalf("expected name get-src, got %s", retrieved.Name)
	}

	// Test nonexistent source
	_, err = svc.GetSource(context.Background(), "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent source")
	}
}

func TestService_CompleteRequest_InvalidStatus(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, NewStoreAdapter(store), nil)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "invalid-status-src", "https://example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")

	// Try to complete from pending (not allowed)
	_, err := svc.CompleteRequest(context.Background(), req.ID, "result")
	if err == nil {
		t.Fatalf("expected error completing from pending status")
	}
}
