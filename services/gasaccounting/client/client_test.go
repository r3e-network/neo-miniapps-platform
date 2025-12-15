// Package client provides a client SDK for the GasAccounting service.
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
	client := New("http://localhost:8080", "test-service")
	if client == nil {
		t.Fatal("New() returned nil")
	}
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %s, want http://localhost:8080", client.baseURL)
	}
	if client.serviceID != "test-service" {
		t.Errorf("serviceID = %s, want test-service", client.serviceID)
	}
}

func TestGetBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/balance" {
			t.Errorf("Path = %s, want /balance", r.URL.Path)
		}

		resp := BalanceResponse{
			UserID:           1,
			AvailableBalance: 50000,
			ReservedBalance:  10000,
			TotalBalance:     60000,
			AsOf:             time.Now(),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, "test")
	resp, err := client.GetBalance(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetBalance() error = %v", err)
	}
	if resp.AvailableBalance != 50000 {
		t.Errorf("AvailableBalance = %d, want 50000", resp.AvailableBalance)
	}
}

func TestGetBalanceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.GetBalance(context.Background(), 1)
	if err == nil {
		t.Error("GetBalance() expected error")
	}
}

func TestDeposit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/deposit" {
			t.Errorf("Path = %s, want /deposit", r.URL.Path)
		}

		var req DepositRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := DepositResponse{
			EntryID:     1,
			NewBalance:  req.Amount,
			DepositedAt: time.Now(),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, "test")
	resp, err := client.Deposit(context.Background(), &DepositRequest{
		UserID: 1,
		Amount: 100000,
		TxHash: "0xabc123",
	})
	if err != nil {
		t.Fatalf("Deposit() error = %v", err)
	}
	if resp.NewBalance != 100000 {
		t.Errorf("NewBalance = %d, want 100000", resp.NewBalance)
	}
}

func TestConsume(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		resp := ConsumeResponse{
			EntryID:    1,
			NewBalance: 95000,
			ConsumedAt: time.Now(),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, "test-service")
	resp, err := client.Consume(context.Background(), 1, 5000, "req-001", "VRF request")
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if resp.NewBalance != 95000 {
		t.Errorf("NewBalance = %d, want 95000", resp.NewBalance)
	}
}

func TestReserve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		resp := ReserveResponse{
			ReservationID: "res-123",
			Amount:        10000,
			ExpiresAt:     time.Now().Add(10 * time.Minute),
			NewAvailable:  90000,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, "test-service")
	resp, err := client.Reserve(context.Background(), 1, 10000, "req-001", 10*time.Minute)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if resp.ReservationID != "res-123" {
		t.Errorf("ReservationID = %s, want res-123", resp.ReservationID)
	}
}

func TestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		resp := ReleaseResponse{
			EntryID:      1,
			Released:     2000,
			Consumed:     8000,
			NewAvailable: 92000,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, "test")
	resp, err := client.Release(context.Background(), "res-123", true, 8000)
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if resp.Consumed != 8000 {
		t.Errorf("Consumed = %d, want 8000", resp.Consumed)
	}
}

func TestGetHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}

		resp := HistoryResponse{
			Entries: []*LedgerEntry{
				{ID: 1, UserID: 1, EntryType: "deposit", Amount: 100000},
				{ID: 2, UserID: 1, EntryType: "consume", Amount: -5000},
			},
			TotalCount: 2,
			HasMore:    false,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, "test")
	resp, err := client.GetHistory(context.Background(), 1, "deposit", 10, 0)
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("Entries count = %d, want 2", len(resp.Entries))
	}
}

func TestGetHistoryNoFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := HistoryResponse{Entries: []*LedgerEntry{}, TotalCount: 0}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.GetHistory(context.Background(), 1, "", 0, 0)
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}
}

func TestClientHTTPError(t *testing.T) {
	client := New("http://invalid-host-that-does-not-exist:99999", "test")
	_, err := client.GetBalance(context.Background(), 1)
	if err == nil {
		t.Error("Expected error for invalid host")
	}
}

func TestClientDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.GetBalance(context.Background(), 1)
	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestClient400Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Deposit(context.Background(), &DepositRequest{UserID: 0})
	if err == nil {
		t.Error("Expected error for 400 response")
	}
}

func TestClient400ErrorNoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Deposit(context.Background(), &DepositRequest{UserID: 0})
	if err == nil {
		t.Error("Expected error for 400 response without body")
	}
}

func TestConsumeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "insufficient balance"})
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Consume(context.Background(), 1, 999999, "req-err", "test")
	if err == nil {
		t.Error("Expected error for consume failure")
	}
}

func TestReserveError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "insufficient balance"})
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Reserve(context.Background(), 1, 999999, "req-err", 5*time.Minute)
	if err == nil {
		t.Error("Expected error for reserve failure")
	}
}

func TestReleaseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "reservation not found"})
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Release(context.Background(), "non-existent", false, 0)
	if err == nil {
		t.Error("Expected error for release failure")
	}
}

func TestGetHistoryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.GetHistory(context.Background(), 1, "", 10, 0)
	if err == nil {
		t.Error("Expected error for history failure")
	}
}

func TestDepositDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Deposit(context.Background(), &DepositRequest{UserID: 1, Amount: 100})
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestConsumeDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid"))
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Consume(context.Background(), 1, 100, "req", "desc")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestReserveDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid"))
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Reserve(context.Background(), 1, 100, "req", time.Minute)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestReleaseDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid"))
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.Release(context.Background(), "res-1", false, 0)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGetHistoryDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid"))
	}))
	defer server.Close()

	client := New(server.URL, "test")
	_, err := client.GetHistory(context.Background(), 1, "", 10, 0)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}
