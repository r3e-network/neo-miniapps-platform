package oracle

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

func TestService_SourceAndRequestLifecycle(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := New(store, store, nil)
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

	if _, err := svc.MarkRunning(context.Background(), req.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if _, err := svc.CompleteRequest(context.Background(), req.ID, `{"price":12.3}`); err != nil {
		t.Fatalf("complete request: %v", err)
	}

	if _, err := svc.FailRequest(context.Background(), req.ID, "should not overwrite success"); err != nil {
		t.Fatalf("fail request: %v", err)
	}

	requests, err := svc.ListRequests(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].Status != domain.StatusFailed {
		t.Fatalf("expected failed status, got %s", requests[0].Status)
	}
}

func ExampleService_CreateRequest() {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "oracle-user"})
	log := logger.NewDefault("example-oracle")
	log.SetOutput(io.Discard)
	svc := New(store, store, log)
	src, _ := svc.CreateSource(context.Background(), acct.ID, "prices", "https://api.example.com", "GET", "", nil, "")
	req, _ := svc.CreateRequest(context.Background(), acct.ID, src.ID, "{}")
	fmt.Println(req.Status)
	// Output:
	// pending
}
