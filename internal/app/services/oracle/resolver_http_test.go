package oracle

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/R3E-Network/service_layer/internal/app/domain/oracle"
)

func TestHTTPResolver(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			w.Write([]byte(`{"done": false, "retry_after_seconds": 0.1}`))
		case 2:
			w.Write([]byte(`{"done": true, "success": true, "result": "payload"}`))
		default:
			t.Fatalf("unexpected call count: %d", calls)
		}
	}))
	defer server.Close()

	resolver, err := NewHTTPResolver(server.Client(), server.URL, "", nil)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	req := domain.Request{ID: "req"}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil || done || success || result != "" || errMsg != "" || retry <= 0 {
		t.Fatalf("unexpected first resolve: done=%v success=%v result=%q err=%v retry=%v", done, success, result, err, retry)
	}

	time.Sleep(50 * time.Millisecond)

	done, success, result, errMsg, _, err = resolver.Resolve(context.Background(), req)
	if err != nil || !done || !success || result != "payload" || errMsg != "" {
		t.Fatalf("unexpected second resolve: done=%v success=%v result=%q err=%v", done, success, result, err)
	}
}
