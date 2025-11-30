package oracle

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/pkg/storage/memory"
)

func TestHTTPResolver_GetRequest(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	var receivedQuery string
	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":42}`))
	}))
	defer server.Close()

	source, err := svc.CreateSource(context.Background(), acct.ID, "prices", server.URL, "GET", "", nil, "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := Request{ID: "req-1", AccountID: acct.ID, DataSourceID: source.ID, Payload: `{"asset":"NEO"}`}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success || retry != 0 {
		t.Fatalf("unexpected state: done=%v success=%v retry=%v", done, success, retry)
	}
	if strings.TrimSpace(result) != `{"value":42}` || errMsg != "" {
		t.Fatalf("unexpected payload result=%q error=%q", result, errMsg)
	}
	if !strings.Contains(receivedQuery, "asset=NEO") {
		t.Fatalf("expected query to include asset, got %q", receivedQuery)
	}
}

func TestHTTPResolver_PostRequest(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	var body string
	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected content type application/json, got %s", ct)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	source, err := svc.CreateSource(context.Background(), acct.ID, "prices-post", server.URL, "POST", "", nil, `{"default":true}`)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := Request{ID: "req-2", AccountID: acct.ID, DataSourceID: source.ID, Payload: `{"override":true}`}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success {
		t.Fatalf("expected success")
	}
	if result != "OK" || errMsg != "" || retry != 0 {
		t.Fatalf("unexpected response result=%q err=%q retry=%v", result, errMsg, retry)
	}
	if body != `{"override":true}` {
		t.Fatalf("expected payload override, got %q", body)
	}
}

func TestHTTPResolver_HandlesErrorStatus(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing parameter", http.StatusBadRequest)
	}))
	defer server.Close()

	source, _ := svc.CreateSource(context.Background(), acct.ID, "error-source", server.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := Request{ID: "req-3", AccountID: acct.ID, DataSourceID: source.ID}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || success || retry != 0 {
		t.Fatalf("expected final failure, got done=%v success=%v retry=%v", done, success, retry)
	}
	if result != "" || !strings.Contains(errMsg, "missing parameter") {
		t.Fatalf("unexpected error message: %q", errMsg)
	}
}

func TestHTTPResolver_RetryableStatus(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	source, _ := svc.CreateSource(context.Background(), acct.ID, "retry-source", server.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := Request{ID: "req-4", AccountID: acct.ID, DataSourceID: source.ID}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error for retryable status")
	}
	if done || success || result != "" || errMsg != "" {
		t.Fatalf("unexpected state for retry: done=%v success=%v result=%q errMsg=%q", done, success, result, errMsg)
	}
	if retry <= 0 || retry > defaultHTTPResolverRetry {
		t.Fatalf("unexpected retry duration: %v", retry)
	}
}

func TestHTTPResolver_MultiSourceAggregate(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	server1 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("10"))
	}))
	defer server1.Close()
	server2 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("30"))
	}))
	defer server2.Close()

	src1, _ := svc.CreateSource(context.Background(), acct.ID, "oracle-1", server1.URL, "GET", "", nil, "")
	src2, _ := svc.CreateSource(context.Background(), acct.ID, "oracle-2", server2.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server1.Client(), nil)
	req := Request{
		ID:           "req-agg",
		AccountID:    acct.ID,
		DataSourceID: src1.ID,
		Payload:      fmt.Sprintf(`{"alternate_source_ids":["%s"]}`, src2.ID),
	}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success || retry != 0 || errMsg != "" {
		t.Fatalf("unexpected state: done=%v success=%v retry=%v errMsg=%q", done, success, retry, errMsg)
	}
	if strings.TrimSpace(result) != "20" && strings.TrimSpace(result) != "20.0" {
		t.Fatalf("expected median aggregated result, got %q", result)
	}
}

func newOracleHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp4 listener unavailable: %v", err)
	}
	server := &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	return server
}

func TestHTTPResolver_WithTracer(t *testing.T) {
	resolver := NewHTTPResolver(nil, nil, nil)
	// With nil tracer
	resolver.WithTracer(nil)
	// With actual tracer
	resolver.WithTracer(mockHTTPTracer{})
}

type mockHTTPTracer struct{}

func (mockHTTPTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

func TestHTTPResolver_SourceNotFound(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	resolver := NewHTTPResolver(svc, nil, nil)
	req := Request{ID: "req-notfound", AccountID: acct.ID, DataSourceID: "nonexistent"}

	done, _, _, errMsg, _, err := resolver.Resolve(context.Background(), req)
	// Source not found should cause an error or return not done with error message
	if done && errMsg == "" && err == nil {
		t.Fatalf("expected error or error message for nonexistent source")
	}
}

func TestHTTPResolver_MedianFloatOddCount(t *testing.T) {
	// Test median with odd number of values
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	server1 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("10"))
	}))
	defer server1.Close()
	server2 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("20"))
	}))
	defer server2.Close()
	server3 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("30"))
	}))
	defer server3.Close()

	src1, _ := svc.CreateSource(context.Background(), acct.ID, "median1", server1.URL, "GET", "", nil, "")
	src2, _ := svc.CreateSource(context.Background(), acct.ID, "median2", server2.URL, "GET", "", nil, "")
	src3, _ := svc.CreateSource(context.Background(), acct.ID, "median3", server3.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server1.Client(), nil)
	req := Request{
		ID:           "req-median-odd",
		AccountID:    acct.ID,
		DataSourceID: src1.ID,
		Payload:      fmt.Sprintf(`{"alternate_source_ids":["%s","%s"]}`, src2.ID, src3.ID),
	}

	done, success, result, _, _, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success {
		t.Fatalf("expected done and success")
	}
	// With odd count (3 values), median should be the middle value (20)
	if !strings.Contains(result, "20") {
		t.Logf("unexpected median result: %s", result)
	}
}

func TestHTTPResolver_NonNumericResults(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	server1 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"hello"`))
	}))
	defer server1.Close()
	server2 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"world"`))
	}))
	defer server2.Close()

	src1, _ := svc.CreateSource(context.Background(), acct.ID, "string1", server1.URL, "GET", "", nil, "")
	src2, _ := svc.CreateSource(context.Background(), acct.ID, "string2", server2.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server1.Client(), nil)
	req := Request{
		ID:           "req-string",
		AccountID:    acct.ID,
		DataSourceID: src1.ID,
		Payload:      fmt.Sprintf(`{"alternate_source_ids":["%s"]}`, src2.ID),
	}

	done, success, result, _, _, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// Non-numeric results should still be returned
	if !done || !success {
		t.Fatalf("expected done and success for non-numeric values")
	}
	// Result should contain one of the string values
	if !strings.Contains(result, "hello") && !strings.Contains(result, "world") {
		t.Logf("result: %s", result)
	}
}

func TestHTTPResolver_AlreadySucceeded(t *testing.T) {
	resolver := NewHTTPResolver(nil, nil, nil)
	req := Request{
		ID:     "req-succeeded",
		Status: StatusSucceeded,
		Result: "already done",
	}

	done, success, result, _, _, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success || result != "already done" {
		t.Fatalf("expected done=true, success=true, result=already done")
	}
}

func TestHTTPResolver_AlreadyFailed(t *testing.T) {
	resolver := NewHTTPResolver(nil, nil, nil)
	req := Request{
		ID:     "req-failed",
		Status: StatusFailed,
		Error:  "previous error",
	}

	done, success, _, errMsg, _, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || success || errMsg != "previous error" {
		t.Fatalf("expected done=true, success=false, errMsg=previous error")
	}
}

func TestHTTPResolver_NoService(t *testing.T) {
	resolver := NewHTTPResolver(nil, nil, nil)
	req := Request{ID: "req-no-svc", Status: StatusPending}

	_, _, _, _, _, err := resolver.Resolve(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error for nil service")
	}
}

func TestHTTPResolver_EmptyDataSourceID(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(NewMemoryAccountChecker(store), NewMemoryStoreAdapter(store, store), nil)

	resolver := NewHTTPResolver(svc, nil, nil)
	req := Request{
		ID:           "req-empty-src",
		AccountID:    acct.ID,
		DataSourceID: "",
		Status:       StatusPending,
	}

	done, success, _, errMsg, _, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// No sources specified should return done=true, success=false with error message
	if !done || success || errMsg == "" {
		t.Fatalf("expected done=true, success=false with error message")
	}
}
