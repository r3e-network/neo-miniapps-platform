package gasbank

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
)

func TestHTTPWithdrawalResolver(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("transaction_id") != "tx" {
			t.Fatalf("unexpected query %s", r.URL.RawQuery)
		}
		switch calls {
		case 1:
			w.Write([]byte(`{"done": false, "retry_after_seconds": 0.1}`))
		case 2:
			w.Write([]byte(`{"done": true, "success": false, "message": "failed"}`))
		default:
			t.Fatalf("too many calls")
		}
	}))
	defer server.Close()

	resolver, err := NewHTTPWithdrawalResolver(server.Client(), server.URL, "", nil)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	tx := gasbank.Transaction{ID: "tx"}

	done, success, msg, retry, err := resolver.Resolve(context.Background(), tx)
	if err != nil || done || success || msg != "" || retry <= 0 {
		t.Fatalf("unexpected first resolve: done=%v success=%v msg=%q err=%v retry=%v", done, success, msg, err, retry)
	}

	time.Sleep(50 * time.Millisecond)

	done, success, msg, _, err = resolver.Resolve(context.Background(), tx)
	if err != nil || !done || success || msg != "failed" {
		t.Fatalf("unexpected second resolve: done=%v success=%v msg=%q err=%v", done, success, msg, err)
	}
}
