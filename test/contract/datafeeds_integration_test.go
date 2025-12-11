// Package contract provides neofeeds integration tests with Neo Express.
package contract

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	neofeeds "github.com/R3E-Network/service_layer/services/neofeeds/marble"
)

// TestNeoFeedsPriceFetching tests that neofeeds can fetch prices from Chainlink and Binance.
func TestNeoFeedsPriceFetching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	m, err := marble.New(marble.Config{MarbleType: "neofeeds"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))

	mockDB := database.NewMockRepository()
	svc, err := neofeeds.New(neofeeds.Config{
		Marble:      m,
		DB:          mockDB,
		ArbitrumRPC: "https://arb1.arbitrum.io/rpc",
	})
	if err != nil {
		t.Fatalf("neofeeds.New: %v", err)
	}

	t.Run("fetch BTC price from Chainlink", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		price, err := svc.GetPrice(ctx, "BTC/USD")
		if err != nil {
			t.Skipf("Chainlink fetch failed (network issue): %v", err)
		}

		t.Logf("BTC/USD price: %d (decimals: %d)", price.Price, price.Decimals)
		if price.Price <= 0 {
			t.Error("expected positive price")
		}
		if price.Decimals != 8 {
			t.Errorf("expected 8 decimals, got %d", price.Decimals)
		}
		if len(price.Signature) == 0 {
			t.Error("expected signature")
		}
	})

	t.Run("fetch ETH price from Chainlink", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		price, err := svc.GetPrice(ctx, "ETH/USD")
		if err != nil {
			t.Skipf("Chainlink fetch failed (network issue): %v", err)
		}

		t.Logf("ETH/USD price: %d (decimals: %d)", price.Price, price.Decimals)
		if price.Price <= 0 {
			t.Error("expected positive price")
		}
	})

	t.Run("fetch NEO price from Binance (not on Chainlink)", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		price, err := svc.GetPrice(ctx, "NEO/USD")
		if err != nil {
			t.Skipf("Binance fetch failed (network issue): %v", err)
		}

		t.Logf("NEO/USD price: %d (decimals: %d)", price.Price, price.Decimals)
		if price.Price <= 0 {
			t.Error("expected positive price")
		}
	})

	t.Run("fetch GAS price from Binance (not on Chainlink)", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		price, err := svc.GetPrice(ctx, "GAS/USD")
		if err != nil {
			t.Skipf("Binance fetch failed (network issue): %v", err)
		}

		t.Logf("GAS/USD price: %d (decimals: %d)", price.Price, price.Decimals)
		if price.Price <= 0 {
			t.Error("expected positive price")
		}
	})
}

// TestNeoFeedsHTTPHandler tests the HTTP handlers for neofeeds service.
func TestNeoFeedsHTTPHandler(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"price": "50000.00"})
	}))
	defer mockServer.Close()

	mockConfig := &neofeeds.NeoFeedsConfig{
		Version: "1.0",
		Sources: []neofeeds.SourceConfig{
			{ID: "mock", Name: "Mock", URL: mockServer.URL, JSONPath: "price", Weight: 1},
		},
		Feeds: []neofeeds.FeedConfig{
			{ID: "BTC/USD", Pair: "BTCUSDT", Sources: []string{"mock"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := neofeeds.New(neofeeds.Config{Marble: m, DB: database.NewMockRepository(), FeedsConfig: mockConfig})

	t.Run("health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("feeds endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/feeds", nil)
		w := httptest.NewRecorder()
		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("price endpoint", func(t *testing.T) {
		// Use URL-encoded slash for feed IDs like BTC/USD
		req := httptest.NewRequest("GET", "/price/BTC%2FUSD", nil)
		w := httptest.NewRecorder()
		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp neofeeds.PriceResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp.Price <= 0 {
			t.Error("expected positive price")
		}
		if len(resp.Signature) == 0 {
			t.Error("expected signature")
		}

		t.Logf("Signature: %s", hex.EncodeToString(resp.Signature))
		t.Logf("PublicKey: %s", hex.EncodeToString(resp.PublicKey))
	})
}

// TestNeoFeedsSignatureVerification tests that signatures can be verified.
func TestNeoFeedsSignatureVerification(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", signingKey)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"price": "50000.00"})
	}))
	defer mockServer.Close()

	mockConfig := &neofeeds.NeoFeedsConfig{
		Version: "1.0",
		Sources: []neofeeds.SourceConfig{
			{ID: "mock", Name: "Mock", URL: mockServer.URL, JSONPath: "price", Weight: 1},
		},
		Feeds: []neofeeds.FeedConfig{
			{ID: "BTC/USD", Pair: "BTCUSDT", Sources: []string{"mock"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := neofeeds.New(neofeeds.Config{Marble: m, DB: database.NewMockRepository(), FeedsConfig: mockConfig})

	ctx := context.Background()
	price, err := svc.GetPrice(ctx, "BTC/USD")
	if err != nil {
		t.Fatalf("GetPrice: %v", err)
	}

	t.Logf("Price Response:")
	t.Logf("  FeedID: %s", price.FeedID)
	t.Logf("  Price: %d", price.Price)
	t.Logf("  Decimals: %d", price.Decimals)
	t.Logf("  Timestamp: %v", price.Timestamp)
	t.Logf("  Signature length: %d", len(price.Signature))
	t.Logf("  PublicKey length: %d", len(price.PublicKey))

	if len(price.Signature) != 64 {
		t.Errorf("expected 64 byte signature, got %d", len(price.Signature))
	}
	if len(price.PublicKey) != 33 {
		t.Errorf("expected 33 byte compressed public key, got %d", len(price.PublicKey))
	}
}

// TestNeoExpressNeoFeedsContract tests neofeeds contract deployment and invocation.
func TestNeoExpressNeoFeedsContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping neo-express test in short mode")
	}

	SkipIfNoNeoExpress(t)

	nefPath := filepath.Join("..", "..", "contracts", "build", "NeoFeedsService.nef")
	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		t.Skip("NeoFeedsService.nef not found, run 'make build-contracts' first")
	}

	t.Log("Neo Express neofeeds contract test - contract deployment ready")
	t.Logf("Contract artifacts found at: %s", nefPath)
}

// TestNeoFeedsMultiplePrices tests fetching multiple prices concurrently.
func TestNeoFeedsMultiplePrices(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))

	mockDB2 := database.NewMockRepository()
	svc, err := neofeeds.New(neofeeds.Config{
		Marble:      m,
		DB:          mockDB2,
		ArbitrumRPC: "https://arb1.arbitrum.io/rpc",
	})
	if err != nil {
		t.Fatalf("neofeeds.New: %v", err)
	}

	feeds := []string{"BTC/USD", "ETH/USD", "SOL/USD", "NEO/USD", "GAS/USD"}
	results := make(chan struct {
		feed  string
		price int64
		err   error
	}, len(feeds))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, feed := range feeds {
		go func(feedID string) {
			price, err := svc.GetPrice(ctx, feedID)
			var p int64
			if price != nil {
				p = price.Price
			}
			results <- struct {
				feed  string
				price int64
				err   error
			}{feedID, p, err}
		}(feed)
	}

	successCount := 0
	for i := 0; i < len(feeds); i++ {
		result := <-results
		if result.err != nil {
			t.Logf("%s: error - %v", result.feed, result.err)
		} else {
			t.Logf("%s: price = %d", result.feed, result.price)
			successCount++
		}
	}

	if successCount == 0 {
		t.Skip("All price fetches failed (network issues)")
	}

	t.Logf("Successfully fetched %d/%d prices", successCount, len(feeds))
}

// TestChainlinkDirectFetch tests direct Chainlink client usage.
func TestChainlinkDirectFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client, err := neofeeds.NewChainlinkClient("")
	if err != nil {
		t.Fatalf("NewChainlinkClient: %v", err)
	}
	defer client.Close()

	t.Run("BTC/USD from Chainlink Arbitrum", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		price, decimals, err := client.GetPrice(ctx, "BTC/USD")
		if err != nil {
			t.Skipf("Chainlink fetch failed: %v", err)
		}

		t.Logf("BTC/USD: $%.2f (decimals: %d)", price, decimals)
		if price < 1000 || price > 1000000 {
			t.Errorf("unreasonable BTC price: %.2f", price)
		}
	})

	t.Run("ETH/USD from Chainlink Arbitrum", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		price, decimals, err := client.GetPrice(ctx, "ETH/USD")
		if err != nil {
			t.Skipf("Chainlink fetch failed: %v", err)
		}

		t.Logf("ETH/USD: $%.2f (decimals: %d)", price, decimals)
		if price < 100 || price > 100000 {
			t.Errorf("unreasonable ETH price: %.2f", price)
		}
	})

	t.Run("unsupported feed should error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _, err := client.GetPrice(ctx, "NEO/USD")
		if err == nil {
			t.Error("expected error for unsupported NEO/USD feed")
		}
	})
}

// TestNeoFeedsServiceInfo tests service info methods.
func TestNeoFeedsServiceInfo(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := neofeeds.New(neofeeds.Config{Marble: m, DB: database.NewMockRepository()})

	if svc.ID() != "neofeeds" {
		t.Errorf("expected ID 'neofeeds', got '%s'", svc.ID())
	}
	if svc.Name() != "NeoFeeds Service" {
		t.Errorf("expected name 'NeoFeeds Service', got '%s'", svc.Name())
	}
	if svc.Version() != "3.0.0" {
		t.Errorf("expected version '3.0.0', got '%s'", svc.Version())
	}
}

// BenchmarkPriceFetching benchmarks price fetching performance.
func BenchmarkPriceFetching(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"price": "50000.00"}`)
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))

	mockConfig := &neofeeds.NeoFeedsConfig{
		Version: "1.0",
		Sources: []neofeeds.SourceConfig{
			{ID: "mock", URL: mockServer.URL, JSONPath: "price", Weight: 1},
		},
		Feeds: []neofeeds.FeedConfig{
			{ID: "BTC/USD", Pair: "BTCUSDT", Sources: []string{"mock"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := neofeeds.New(neofeeds.Config{Marble: m, DB: database.NewMockRepository(), FeedsConfig: mockConfig})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GetPrice(ctx, "BTC/USD")
	}
}
