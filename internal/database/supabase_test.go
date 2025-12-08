// Package database provides Supabase database integration.
package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// =============================================================================
// Client Tests
// =============================================================================

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				URL:        "https://test.supabase.co",
				ServiceKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			cfg: Config{
				URL:        "",
				ServiceKey: "test-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestClientRequest(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("apikey") != "test-key" {
			t.Errorf("apikey header = %s, want test-key", r.Header.Get("apikey"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header = %s, want application/json", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]string{{"id": "123"}})
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		URL:        server.URL,
		ServiceKey: "test-key",
	})

	ctx := context.Background()
	data, err := client.request(ctx, "GET", "test_table", nil, "")
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("request() returned empty data")
	}
}

func TestClientRequestWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode body: %v", err)
		}

		if body["name"] != "test" {
			t.Errorf("body[name] = %s, want test", body["name"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		URL:        server.URL,
		ServiceKey: "test-key",
	})

	ctx := context.Background()
	_, err := client.request(ctx, "POST", "test_table", map[string]string{"name": "test"}, "")
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
}

func TestClientRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		URL:        server.URL,
		ServiceKey: "test-key",
	})

	ctx := context.Background()
	_, err := client.request(ctx, "GET", "test_table", nil, "")
	if err == nil {
		t.Error("request() should return error for 400 status")
	}
}

// =============================================================================
// Domain Model Tests
// =============================================================================

func TestUserJSON(t *testing.T) {
	user := User{
		ID:        "user-123",
		Address:   "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != user.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, user.ID)
	}
	if decoded.Address != user.Address {
		t.Errorf("Address = %s, want %s", decoded.Address, user.Address)
	}
}

func TestAPIKeyJSON(t *testing.T) {
	apiKey := APIKey{
		ID:      "key-123",
		UserID:  "user-456",
		Name:    "Test Key",
		KeyHash: "hash123",
		Prefix:  "sk_test",
		Scopes:  []string{"read", "write"},
	}

	data, err := json.Marshal(apiKey)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded APIKey
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Name != apiKey.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, apiKey.Name)
	}
	if len(decoded.Scopes) != len(apiKey.Scopes) {
		t.Errorf("Scopes length = %d, want %d", len(decoded.Scopes), len(apiKey.Scopes))
	}
}

func TestSecretJSON(t *testing.T) {
	secret := Secret{
		ID:             "secret-123",
		UserID:         "user-456",
		Name:           "API_KEY",
		EncryptedValue: []byte("encrypted-data"),
		Version:        1,
	}

	data, err := json.Marshal(secret)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Secret
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Name != secret.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, secret.Name)
	}
}

func TestServiceRequestJSON(t *testing.T) {
	req := ServiceRequest{
		ID:          "req-123",
		UserID:      "user-456",
		ServiceType: "vrf",
		Status:      "pending",
		Payload:     json.RawMessage(`{"pair":"BTC/USD"}`),
		GasUsed:     1000,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ServiceRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ServiceType != req.ServiceType {
		t.Errorf("ServiceType = %s, want %s", decoded.ServiceType, req.ServiceType)
	}
	if decoded.GasUsed != req.GasUsed {
		t.Errorf("GasUsed = %d, want %d", decoded.GasUsed, req.GasUsed)
	}
}

func TestPriceFeedJSON(t *testing.T) {
	feed := PriceFeed{
		ID:        "feed-123",
		FeedID:    "BTC-USD",
		Pair:      "BTC/USD",
		Price:     5000000000000, // $50,000 with 8 decimals
		Decimals:  8,
		Timestamp: time.Now(),
		Sources:   []string{"binance", "coinbase"},
	}

	data, err := json.Marshal(feed)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PriceFeed
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Price != feed.Price {
		t.Errorf("Price = %d, want %d", decoded.Price, feed.Price)
	}
	if len(decoded.Sources) != len(feed.Sources) {
		t.Errorf("Sources length = %d, want %d", len(decoded.Sources), len(feed.Sources))
	}
}

func TestGasBankAccountJSON(t *testing.T) {
	account := GasBankAccount{
		ID:       "account-123",
		UserID:   "user-456",
		Balance:  1000000,
		Reserved: 100000,
	}

	data, err := json.Marshal(account)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded GasBankAccount
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Balance != account.Balance {
		t.Errorf("Balance = %d, want %d", decoded.Balance, account.Balance)
	}
	if decoded.Reserved != account.Reserved {
		t.Errorf("Reserved = %d, want %d", decoded.Reserved, account.Reserved)
	}
}

// =============================================================================
// Repository Tests with Mock Server
// =============================================================================

func TestRepositoryGetUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v1/users" {
			t.Errorf("Path = %s, want /rest/v1/users", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]User{{
			ID:      "user-123",
			Address: "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR",
		}})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	user, err := repo.GetUser(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("ID = %s, want user-123", user.ID)
	}
}

func TestRepositoryGetUserNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]User{})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	_, err := repo.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("GetUser() should return error for nonexistent user")
	}
}

func TestRepositoryGetUserByAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]User{{
			ID:      "user-123",
			Address: "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR",
		}})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	user, err := repo.GetUserByAddress(ctx, "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR")
	if err != nil {
		t.Fatalf("GetUserByAddress() error = %v", err)
	}

	if user.Address != "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR" {
		t.Errorf("Address = %s, want NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR", user.Address)
	}
}

func TestRepositoryCreateUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]User{})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	err := repo.CreateUser(ctx, &User{
		ID:      "user-123",
		Address: "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
}

func TestRepositoryGetSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Secret{
			{ID: "secret-1", Name: "API_KEY"},
			{ID: "secret-2", Name: "DB_PASSWORD"},
		})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	secrets, err := repo.GetSecrets(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetSecrets() error = %v", err)
	}

	if len(secrets) != 2 {
		t.Errorf("len(secrets) = %d, want 2", len(secrets))
	}
}

func TestRepositoryGetServiceRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ServiceRequest{
			{ID: "req-1", ServiceType: "vrf"},
			{ID: "req-2", ServiceType: "mixer"},
		})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	requests, err := repo.GetServiceRequests(ctx, "user-123", 10)
	if err != nil {
		t.Fatalf("GetServiceRequests() error = %v", err)
	}

	if len(requests) != 2 {
		t.Errorf("len(requests) = %d, want 2", len(requests))
	}
}

func TestRepositoryGetLatestPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]PriceFeed{{
			ID:     "feed-123",
			FeedID: "BTC-USD",
			Price:  5000000000000,
		}})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	feed, err := repo.GetLatestPrice(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("GetLatestPrice() error = %v", err)
	}

	if feed.FeedID != "BTC-USD" {
		t.Errorf("FeedID = %s, want BTC-USD", feed.FeedID)
	}
}

func TestRepositoryGetGasBankAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]GasBankAccount{{
			ID:       "account-123",
			UserID:   "user-456",
			Balance:  1000000,
			Reserved: 100000,
		}})
	}))
	defer server.Close()

	client, _ := NewClient(Config{URL: server.URL, ServiceKey: "test-key"})
	repo := NewRepository(client)

	ctx := context.Background()
	account, err := repo.GetGasBankAccount(ctx, "user-456")
	if err != nil {
		t.Fatalf("GetGasBankAccount() error = %v", err)
	}

	if account.Balance != 1000000 {
		t.Errorf("Balance = %d, want 1000000", account.Balance)
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewClient(Config{
			URL:        "https://test.supabase.co",
			ServiceKey: "test-key",
		})
	}
}

func BenchmarkUserMarshal(b *testing.B) {
	user := User{
		ID:        "user-123",
		Address:   "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(user)
	}
}

func BenchmarkUserUnmarshal(b *testing.B) {
	data := []byte(`{"id":"user-123","address":"NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR","email":"test@example.com"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var user User
		_ = json.Unmarshal(data, &user)
	}
}
