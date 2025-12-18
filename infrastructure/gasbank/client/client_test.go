package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c, err := New(Config{BaseURL: "http://localhost:8080"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %s, want http://localhost:8080", c.baseURL)
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewEmptyBaseURL(t *testing.T) {
	_, err := New(Config{BaseURL: ""})
	if err == nil {
		t.Error("New() expected error for empty base URL")
	}
}

func TestNewCustomHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 30 * time.Second}
	c, err := New(Config{
		BaseURL:    "http://localhost:8080",
		HTTPClient: customClient,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.httpClient != customClient {
		t.Error("httpClient should be the custom client")
	}
}

func TestDeductFeeNilRequest(t *testing.T) {
	c, _ := New(Config{BaseURL: "http://localhost:8080"})
	_, err := c.DeductFee(context.Background(), nil)
	if err == nil {
		t.Error("DeductFee() expected error for nil request")
	}
}

func TestDeductFeeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/deduct" {
			t.Errorf("path = %s, want /deduct", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}

		var req DeductFeeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.UserID != "user1" {
			t.Errorf("UserID = %s, want user1", req.UserID)
		}
		if req.Amount != 100 {
			t.Errorf("Amount = %d, want 100", req.Amount)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeductFeeResponse{
			Success:       true,
			TransactionID: "tx123",
			BalanceAfter:  900,
		})
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	resp, err := c.DeductFee(context.Background(), &DeductFeeRequest{
		UserID:    "user1",
		Amount:    100,
		ServiceID: "neofeeds",
	})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if !resp.Success {
		t.Error("Success should be true")
	}
	if resp.TransactionID != "tx123" {
		t.Errorf("TransactionID = %s, want tx123", resp.TransactionID)
	}
	if resp.BalanceAfter != 900 {
		t.Errorf("BalanceAfter = %d, want 900", resp.BalanceAfter)
	}
}

func TestDeductFeeHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "insufficient balance"})
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	_, err := c.DeductFee(context.Background(), &DeductFeeRequest{
		UserID:    "user1",
		Amount:    100,
		ServiceID: "neofeeds",
	})
	if err == nil {
		t.Error("DeductFee() expected error for HTTP 403")
	}
}

func TestDeductFeeHTTPErrorNoJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	_, err := c.DeductFee(context.Background(), &DeductFeeRequest{
		UserID:    "user1",
		Amount:    100,
		ServiceID: "neofeeds",
	})
	if err == nil {
		t.Error("DeductFee() expected error for HTTP 500")
	}
}

func TestDeductFeeInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	_, err := c.DeductFee(context.Background(), &DeductFeeRequest{
		UserID:    "user1",
		Amount:    100,
		ServiceID: "neofeeds",
	})
	if err == nil {
		t.Error("DeductFee() expected error for invalid JSON")
	}
}

func TestGetAccountSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/account" {
			t.Errorf("path = %s, want /account", r.URL.Path)
		}
		if r.Header.Get("X-User-ID") != "user1" {
			t.Errorf("X-User-ID = %s, want user1", r.Header.Get("X-User-ID"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GetAccountResponse{
			ID:        "acc1",
			UserID:    "user1",
			Balance:   1000,
			Reserved:  100,
			Available: 900,
		})
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	resp, err := c.GetAccount(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if resp.ID != "acc1" {
		t.Errorf("ID = %s, want acc1", resp.ID)
	}
	if resp.Balance != 1000 {
		t.Errorf("Balance = %d, want 1000", resp.Balance)
	}
	if resp.Available != 900 {
		t.Errorf("Available = %d, want 900", resp.Available)
	}
}

func TestGetAccountHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	_, err := c.GetAccount(context.Background(), "user1")
	if err == nil {
		t.Error("GetAccount() expected error for HTTP 404")
	}
}

func TestGetAccountInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	_, err := c.GetAccount(context.Background(), "user1")
	if err == nil {
		t.Error("GetAccount() expected error for invalid JSON")
	}
}

func TestCheckBalanceSufficient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GetAccountResponse{
			ID:        "acc1",
			UserID:    "user1",
			Balance:   1000,
			Reserved:  100,
			Available: 900,
		})
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	sufficient, available, err := c.CheckBalance(context.Background(), "user1", 500)
	if err != nil {
		t.Fatalf("CheckBalance() error = %v", err)
	}
	if !sufficient {
		t.Error("sufficient should be true")
	}
	if available != 900 {
		t.Errorf("available = %d, want 900", available)
	}
}

func TestCheckBalanceInsufficient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GetAccountResponse{
			ID:        "acc1",
			UserID:    "user1",
			Balance:   1000,
			Reserved:  100,
			Available: 900,
		})
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	sufficient, available, err := c.CheckBalance(context.Background(), "user1", 1000)
	if err != nil {
		t.Fatalf("CheckBalance() error = %v", err)
	}
	if sufficient {
		t.Error("sufficient should be false")
	}
	if available != 900 {
		t.Errorf("available = %d, want 900", available)
	}
}

func TestCheckBalanceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c, _ := New(Config{BaseURL: server.URL})
	_, _, err := c.CheckBalance(context.Background(), "user1", 500)
	if err == nil {
		t.Error("CheckBalance() expected error")
	}
}

func TestDeductFeeConnectionError(t *testing.T) {
	c, _ := New(Config{BaseURL: "http://localhost:99999"})
	_, err := c.DeductFee(context.Background(), &DeductFeeRequest{
		UserID:    "user1",
		Amount:    100,
		ServiceID: "neofeeds",
	})
	if err == nil {
		t.Error("DeductFee() expected error for connection failure")
	}
}

func TestGetAccountConnectionError(t *testing.T) {
	c, _ := New(Config{BaseURL: "http://localhost:99999"})
	_, err := c.GetAccount(context.Background(), "user1")
	if err == nil {
		t.Error("GetAccount() expected error for connection failure")
	}
}

func TestDefaultTimeout(t *testing.T) {
	if defaultTimeout != 10*time.Second {
		t.Errorf("defaultTimeout = %v, want 10s", defaultTimeout)
	}
}
