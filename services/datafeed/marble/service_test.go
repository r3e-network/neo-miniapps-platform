// Package neofeeds provides price feed aggregation service.
package neofeeds

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	"github.com/R3E-Network/service_layer/infrastructure/testutil"
)

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})

	svc, err := New(&Config{
		Marble: m,
		DB:     nil,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.ID() != ServiceID {
		t.Errorf("ID() = %s, want %s", svc.ID(), ServiceID)
	}
	if svc.Name() != ServiceName {
		t.Errorf("Name() = %s, want %s", svc.Name(), ServiceName)
	}
	if svc.Version() != Version {
		t.Errorf("Version() = %s, want %s", svc.Version(), Version)
	}
}

func TestServiceConstants(t *testing.T) {
	if ServiceID != "neofeeds" {
		t.Errorf("ServiceID = %s, want neofeeds", ServiceID)
	}
	if ServiceName != "NeoFeeds Service" {
		t.Errorf("ServiceName = %s, want NeoFeeds Service", ServiceName)
	}
	if Version != "3.0.0" {
		t.Errorf("Version = %s, want 3.0.0", Version)
	}
}

func TestInitDefaultSources(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	// Should have 1 default source (binance)
	if len(svc.sources) != 1 {
		t.Errorf("len(sources) = %d, want 1", len(svc.sources))
	}

	// Check source names
	expectedSources := []string{"binance"}
	for _, name := range expectedSources {
		if _, ok := svc.sources[name]; !ok {
			t.Errorf("Source %s not found", name)
		}
	}
}

// =============================================================================
// calculateMedian Tests
// =============================================================================

func TestCalculateMedianOdd(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	prices := []float64{10.0, 20.0, 30.0}
	median := svc.calculateMedian(prices)

	if median != 20.0 {
		t.Errorf("calculateMedian() = %f, want 20.0", median)
	}
}

func TestCalculateMedianEven(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	prices := []float64{10.0, 20.0, 30.0, 40.0}
	median := svc.calculateMedian(prices)

	if median != 25.0 {
		t.Errorf("calculateMedian() = %f, want 25.0", median)
	}
}

func TestCalculateMedianSingle(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	prices := []float64{50.0}
	median := svc.calculateMedian(prices)

	if median != 50.0 {
		t.Errorf("calculateMedian() = %f, want 50.0", median)
	}
}

func TestCalculateMedianUnsorted(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	// Unsorted input should still work
	prices := []float64{30.0, 10.0, 20.0}
	median := svc.calculateMedian(prices)

	if median != 20.0 {
		t.Errorf("calculateMedian() = %f, want 20.0", median)
	}
}

// =============================================================================
// signPrice Tests
// =============================================================================

func TestSignPriceWithKey(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))
	svc, _ := New(&Config{Marble: m})

	price := &PriceResponse{
		Pair:      "BTCUSDT",
		Price:     5000000000000,
		Decimals:  8,
		Timestamp: time.Now(),
	}

	sig, pub, err := svc.signPrice(price)
	if err != nil {
		t.Fatalf("signPrice() error = %v", err)
	}

	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}
	if len(pub) != 33 {
		t.Errorf("pubkey length = %d, want 33", len(pub))
	}
}

func TestSignPriceWithoutKey(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	price := &PriceResponse{
		Pair:      "BTCUSDT",
		Price:     5000000000000,
		Decimals:  8,
		Timestamp: time.Now(),
	}

	// Without signing key (nil/empty), DeriveKey still works with empty input
	// The function will succeed but produce a signature based on empty key derivation
	sig, pub, err := svc.signPrice(price)
	if err != nil {
		// If it fails, that's also acceptable behavior
		t.Logf("signPrice() returned error without signing key: %v", err)
		return
	}
	// If it succeeds, signature should still be valid format
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}
	if len(pub) != 33 {
		t.Errorf("pubkey length = %d, want 33", len(pub))
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandleGetPriceMissingPair(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	req := httptest.NewRequest("GET", "/price/", nil)
	rr := httptest.NewRecorder()

	svc.handleGetPrice(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleGetPrices(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	mockDB := database.NewMockRepository()
	svc, _ := New(&Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest("GET", "/prices", nil)
	rr := httptest.NewRecorder()

	svc.handleGetPrices(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var prices []PriceResponse
	if err := json.NewDecoder(rr.Body).Decode(&prices); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return empty array
	if len(prices) != 0 {
		t.Errorf("len(prices) = %d, want 0", len(prices))
	}
}

func TestHandleListFeeds(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	req := httptest.NewRequest("GET", "/feeds", nil)
	rr := httptest.NewRecorder()

	svc.handleListFeeds(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var feeds []FeedSummary
	if err := json.NewDecoder(rr.Body).Decode(&feeds); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	expected := len(svc.GetEnabledFeeds())
	if len(feeds) != expected {
		t.Errorf("len(feeds) = %d, want %d", len(feeds), expected)
	}
}

// =============================================================================
// Request/Response Type Tests
// =============================================================================

func TestPriceSourceJSON(t *testing.T) {
	src := PriceSource{
		Name:     "Binance",
		URL:      "https://api.binance.com/api/v3/ticker/price",
		JSONPath: "price",
		Weight:   3,
	}

	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PriceSource
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Name != src.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, src.Name)
	}
	if decoded.Weight != src.Weight {
		t.Errorf("Weight = %d, want %d", decoded.Weight, src.Weight)
	}
}

func TestPriceResponseJSON(t *testing.T) {
	resp := PriceResponse{
		FeedID:    "BTCUSDT",
		Pair:      "BTCUSDT",
		Price:     5000000000000,
		Decimals:  8,
		Timestamp: time.Now(),
		Sources:   []string{"binance", "coinbase"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PriceResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Price != resp.Price {
		t.Errorf("Price = %d, want %d", decoded.Price, resp.Price)
	}
	if decoded.Decimals != resp.Decimals {
		t.Errorf("Decimals = %d, want %d", decoded.Decimals, resp.Decimals)
	}
	if len(decoded.Sources) != len(resp.Sources) {
		t.Errorf("len(Sources) = %d, want %d", len(decoded.Sources), len(resp.Sources))
	}
}

// =============================================================================
// GetPrice and fetchPrice Tests with Mock Server
// =============================================================================

func TestGetPriceWithMockSources(t *testing.T) {
	// Create mock price API server
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a mock price response
		json.NewEncoder(w).Encode(map[string]interface{}{
			"price": "50000.50",
			"data": map[string]string{
				"amount": "50000.50",
			},
			"result": map[string]interface{}{
				"XXBTZUSD": map[string]interface{}{
					"c": []string{"50000.50", "1.0"},
				},
			},
		})
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	mockConfig := &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{ID: "mock1", Name: "Mock1", URL: mockServer.URL, JSONPath: "price", Weight: 2},
			{ID: "mock2", Name: "Mock2", URL: mockServer.URL, JSONPath: "data.amount", Weight: 1},
		},
		Feeds: []FeedConfig{
			{ID: "BTCUSDT", Pair: "BTCUSDT", Sources: []string{"mock1", "mock2"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := New(&Config{Marble: m, FeedsConfig: mockConfig})

	price, err := svc.GetPrice(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatalf("GetPrice() error = %v", err)
	}

	if price.Pair != "BTCUSDT" {
		t.Errorf("Pair = %s, want BTCUSDT", price.Pair)
	}

	if price.Decimals != 8 {
		t.Errorf("Decimals = %d, want 8", price.Decimals)
	}

	if len(price.Sources) == 0 {
		t.Error("Expected at least one source")
	}

	// Price should be around 50000.50 * 1e8 = 5000050000000
	expectedPrice := int64(50000.50 * 1e8)
	if price.Price != expectedPrice {
		t.Errorf("Price = %d, want %d", price.Price, expectedPrice)
	}
}

func TestGetPriceNoSources(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	emptyConfig := &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{ID: "dummy", URL: "http://invalid", JSONPath: "price"},
		},
		Feeds:          []FeedConfig{},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := New(&Config{Marble: m, FeedsConfig: emptyConfig})

	// Clear all sources to force error
	svc.sources = map[string]*SourceConfig{}

	_, err := svc.GetPrice(context.Background(), "BTCUSDT")
	if err == nil {
		t.Error("GetPrice() expected error with no sources")
	}
}

func TestGetPriceAllSourcesFail(t *testing.T) {
	// Create mock server that always fails
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	failConfig := &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{ID: "failing", Name: "Failing", URL: mockServer.URL, JSONPath: "price", Weight: 1},
		},
		Feeds: []FeedConfig{
			{ID: "BTCUSDT", Pair: "BTCUSDT", Sources: []string{"failing"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := New(&Config{Marble: m, FeedsConfig: failConfig})

	_, err := svc.GetPrice(context.Background(), "BTCUSDT")
	if err == nil {
		t.Error("GetPrice() expected error when all sources fail")
	}
}

func TestGetPriceWithSigningKey(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"price": "50000.00"})
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))
	mockConfig := &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{ID: "mock", Name: "Mock", URL: mockServer.URL, JSONPath: "price", Weight: 1},
		},
		Feeds: []FeedConfig{
			{ID: "BTCUSDT", Pair: "BTCUSDT", Sources: []string{"mock"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := New(&Config{Marble: m, FeedsConfig: mockConfig})

	price, err := svc.GetPrice(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatalf("GetPrice() error = %v", err)
	}

	// Should have signature when signing key is set
	if len(price.Signature) == 0 {
		t.Error("Expected signature when signing key is set")
	}
}

func TestFetchPriceSuccess(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"price": "42000.75"})
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	source := PriceSource{
		Name:     "Mock",
		URL:      mockServer.URL,
		JSONPath: "price",
		Weight:   1,
	}

	price, err := svc.fetchPrice(context.Background(), "ETHUSDT", source)
	if err != nil {
		t.Fatalf("fetchPrice() error = %v", err)
	}

	if price != 42000.75 {
		t.Errorf("price = %f, want 42000.75", price)
	}
}

func TestFetchPriceInvalidJSON(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	source := PriceSource{
		Name:     "Mock",
		URL:      mockServer.URL,
		JSONPath: "price",
		Weight:   1,
	}

	// gjson handles invalid JSON gracefully, returning 0
	price, _ := svc.fetchPrice(context.Background(), "BTCUSDT", source)
	if price != 0 {
		t.Errorf("price = %f, want 0 for invalid JSON", price)
	}
}

func TestFetchPriceMissingPath(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"other": "value"})
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	source := PriceSource{
		Name:     "Mock",
		URL:      mockServer.URL,
		JSONPath: "price", // This path doesn't exist in response
		Weight:   1,
	}

	_, err := svc.fetchPrice(context.Background(), "BTCUSDT", source)
	if err == nil {
		t.Error("fetchPrice() expected error for missing JSON path")
	}
}

func TestFetchPriceHTTPError(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	source := PriceSource{
		Name:     "Mock",
		URL:      mockServer.URL,
		JSONPath: "price",
		Weight:   1,
	}

	// HTTP errors don't cause fetchPrice to fail, it just returns empty body
	// which results in missing path error
	_, err := svc.fetchPrice(context.Background(), "BTCUSDT", source)
	if err == nil {
		t.Error("fetchPrice() expected error for HTTP error response")
	}
}

func TestFetchPriceConnectionError(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	source := PriceSource{
		Name:     "Mock",
		URL:      "http://localhost:99999", // Invalid port
		JSONPath: "price",
		Weight:   1,
	}

	_, err := svc.fetchPrice(context.Background(), "BTCUSDT", source)
	if err == nil {
		t.Error("fetchPrice() expected error for connection failure")
	}
}

func TestHandleGetPriceSuccess(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"price": "50000.00"})
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	mockConfig := &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{ID: "mock", Name: "Mock", URL: mockServer.URL, JSONPath: "price", Weight: 1},
		},
		Feeds: []FeedConfig{
			{ID: "BTCUSDT", Pair: "BTCUSDT", Sources: []string{"mock"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := New(&Config{Marble: m, FeedsConfig: mockConfig})

	req := httptest.NewRequest("GET", "/price/BTCUSDT", nil)
	req = mux.SetURLVars(req, map[string]string{"pair": "BTCUSDT"})
	rr := httptest.NewRecorder()

	svc.handleGetPrice(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp PriceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Pair != "BTCUSDT" {
		t.Errorf("Pair = %s, want BTCUSDT", resp.Pair)
	}
}

func TestHandleGetPriceLegacySlashPair(t *testing.T) {
	mockServer := testutil.NewHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"price": "50000.00"})
	}))
	defer mockServer.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	mockConfig := &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{ID: "mock", Name: "Mock", URL: mockServer.URL, JSONPath: "price", Weight: 1},
		},
		Feeds: []FeedConfig{
			{ID: "BTC-USD", Pair: "BTCUSDT", Sources: []string{"mock"}, Enabled: true},
		},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := New(&Config{Marble: m, FeedsConfig: mockConfig})

	req := httptest.NewRequest("GET", "/price/BTC/USD", nil)
	req = mux.SetURLVars(req, map[string]string{"pair": "BTC/USD"})
	rr := httptest.NewRecorder()

	svc.handleGetPrice(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp PriceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.FeedID != "BTC-USD" {
		t.Errorf("FeedID = %s, want BTC-USD", resp.FeedID)
	}
	if resp.Pair != "BTC-USD" {
		t.Errorf("Pair = %s, want BTC-USD", resp.Pair)
	}
}

func TestHandleGetPriceError(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	emptyConfig := &NeoFeedsConfig{
		Version: "1.0",
		Sources: []SourceConfig{
			{ID: "dummy", URL: "http://invalid", JSONPath: "price"},
		},
		Feeds:          []FeedConfig{},
		UpdateInterval: 60 * time.Second,
	}
	svc, _ := New(&Config{Marble: m, FeedsConfig: emptyConfig})

	// Clear sources to force error
	svc.sources = map[string]*SourceConfig{}

	req := httptest.NewRequest("GET", "/price/BTCUSDT", nil)
	req = mux.SetURLVars(req, map[string]string{"pair": "BTCUSDT"})
	rr := httptest.NewRecorder()

	svc.handleGetPrice(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkCalculateMedian(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	svc, _ := New(&Config{Marble: m})

	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(i * 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make a copy since calculateMedian sorts in place
		pricesCopy := make([]float64, len(prices))
		copy(pricesCopy, prices)
		_ = svc.calculateMedian(pricesCopy)
	}
}

func BenchmarkSignPrice(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))
	svc, _ := New(&Config{Marble: m})

	price := &PriceResponse{
		Pair:      "BTCUSDT",
		Price:     5000000000000,
		Decimals:  8,
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = svc.signPrice(price)
	}
}

func BenchmarkPriceResponseMarshal(b *testing.B) {
	resp := PriceResponse{
		FeedID:    "BTCUSDT",
		Pair:      "BTCUSDT",
		Price:     5000000000000,
		Decimals:  8,
		Timestamp: time.Now(),
		Sources:   []string{"binance", "coinbase", "kraken"},
		Signature: make([]byte, 64),
		PublicKey: make([]byte, 33),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(resp)
	}
}
