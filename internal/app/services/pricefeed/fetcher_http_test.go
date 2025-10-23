package pricefeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
)

func TestHTTPFetcher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("base") != "NEO" || r.URL.Query().Get("quote") != "USD" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("expected auth header, got %q", got)
		}
		w.Write([]byte(`{"price": 10.5, "source": "test"}`))
	}))
	defer server.Close()

	fetcher, err := NewHTTPFetcher(server.Client(), server.URL, "token", nil)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	price, source, err := fetcher.Fetch(context.Background(), domain.Feed{BaseAsset: "NEO", QuoteAsset: "USD"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if price != 10.5 || source != "test" {
		t.Fatalf("unexpected result price=%v source=%s", price, source)
	}
}
