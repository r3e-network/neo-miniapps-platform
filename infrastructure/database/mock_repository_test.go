package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMockRepository_UserOperations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Test CreateUser
	user := &User{
		Address: "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR",
		Email:   "test@example.com",
	}
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if user.ID == "" {
		t.Error("CreateUser() should set ID")
	}

	// Test GetUser
	got, err := repo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.Address != user.Address {
		t.Errorf("GetUser() Address = %v, want %v", got.Address, user.Address)
	}

	// Test GetUserByAddress
	got, err = repo.GetUserByAddress(ctx, user.Address)
	if err != nil {
		t.Fatalf("GetUserByAddress() error = %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("GetUserByAddress() ID = %v, want %v", got.ID, user.ID)
	}

	// Test GetUserByEmail
	got, err = repo.GetUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("GetUserByEmail() ID = %v, want %v", got.ID, user.ID)
	}

	// Test UpdateUserEmail
	newEmail := "updated@example.com"
	err = repo.UpdateUserEmail(ctx, user.ID, newEmail)
	if err != nil {
		t.Fatalf("UpdateUserEmail() error = %v", err)
	}
	got, _ = repo.GetUser(ctx, user.ID)
	if got.Email != newEmail {
		t.Errorf("UpdateUserEmail() Email = %v, want %v", got.Email, newEmail)
	}

	// Test user not found
	_, err = repo.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("GetUser() should return error for nonexistent user")
	}
}

func TestMockRepository_GasBankOperations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Test GetOrCreateGasBankAccount (create)
	account, err := repo.GetOrCreateGasBankAccount(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetOrCreateGasBankAccount() error = %v", err)
	}
	if account.UserID != "user-123" {
		t.Errorf("GetOrCreateGasBankAccount() UserID = %v, want user-123", account.UserID)
	}

	// Test GetOrCreateGasBankAccount (get existing)
	account2, err := repo.GetOrCreateGasBankAccount(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetOrCreateGasBankAccount() error = %v", err)
	}
	if account2.ID != account.ID {
		t.Error("GetOrCreateGasBankAccount() should return existing account")
	}

	// Test UpdateGasBankBalance
	err = repo.UpdateGasBankBalance(ctx, "user-123", 1000000, 100000)
	if err != nil {
		t.Fatalf("UpdateGasBankBalance() error = %v", err)
	}
	got, _ := repo.GetGasBankAccount(ctx, "user-123")
	if got.Balance != 1000000 {
		t.Errorf("UpdateGasBankBalance() Balance = %v, want 1000000", got.Balance)
	}

	// Test CreateGasBankTransaction
	tx := &GasBankTransaction{
		AccountID:    account.ID,
		TxType:       "deposit",
		Amount:       1000000,
		BalanceAfter: 1000000,
	}
	err = repo.CreateGasBankTransaction(ctx, tx)
	if err != nil {
		t.Fatalf("CreateGasBankTransaction() error = %v", err)
	}

	// Test GetGasBankTransactions
	txs, err := repo.GetGasBankTransactions(ctx, account.ID, 10)
	if err != nil {
		t.Fatalf("GetGasBankTransactions() error = %v", err)
	}
	if len(txs) != 1 {
		t.Errorf("GetGasBankTransactions() len = %v, want 1", len(txs))
	}
}

func TestMockRepository_ErrorInjection(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Inject error
	expectedErr := errors.New("database connection failed")
	repo.ErrorOnNextCall = expectedErr

	// Next call should return the error
	_, err := repo.GetUser(ctx, "any-id")
	if err != expectedErr {
		t.Errorf("ErrorOnNextCall should return injected error, got %v", err)
	}

	// Subsequent call should succeed (error cleared)
	repo.CreateUser(ctx, &User{Address: "test"})
	_, err = repo.GetUserByAddress(ctx, "test")
	if err != nil {
		t.Errorf("Error should be cleared after first call, got %v", err)
	}
}

func TestMockRepository_Reset(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Add some data
	repo.CreateUser(ctx, &User{Address: "test"})

	// Reset
	repo.Reset()

	// Data should be cleared
	_, err := repo.GetUserByAddress(ctx, "test")
	if err == nil {
		t.Error("Reset() should clear all data")
	}
}

func TestMockRepository_DepositOperations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Test CreateDepositRequest
	deposit := &DepositRequest{
		UserID:      "user-123",
		AccountID:   "account-456",
		Amount:      1000000,
		TxHash:      "0x123abc",
		FromAddress: "NFrom123",
		Status:      "pending",
	}
	if err := repo.CreateDepositRequest(ctx, deposit); err != nil {
		t.Fatalf("CreateDepositRequest() error = %v", err)
	}

	// Test GetDepositByTxHash
	got, err := repo.GetDepositByTxHash(ctx, "0x123abc")
	if err != nil {
		t.Fatalf("GetDepositByTxHash() error = %v", err)
	}
	if got.Amount != deposit.Amount {
		t.Errorf("GetDepositByTxHash() Amount = %v, want %v", got.Amount, deposit.Amount)
	}

	// Test GetDepositRequests
	list, err := repo.GetDepositRequests(ctx, "user-123", 10)
	if err != nil {
		t.Fatalf("GetDepositRequests() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("GetDepositRequests() len = %v, want 1", len(list))
	}

	// Test UpdateDepositStatus
	if err := repo.UpdateDepositStatus(ctx, deposit.ID, "confirmed", 6); err != nil {
		t.Fatalf("UpdateDepositStatus() error = %v", err)
	}
	got, _ = repo.GetDepositByTxHash(ctx, "0x123abc")
	if got.Status != "confirmed" {
		t.Errorf("UpdateDepositStatus() Status = %v, want confirmed", got.Status)
	}
	if got.Confirmations != 6 {
		t.Errorf("UpdateDepositStatus() Confirmations = %v, want 6", got.Confirmations)
	}
}

func TestMockRepository_PriceFeedOperations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Create multiple price feeds
	feed1 := &PriceFeed{
		FeedID:    "BTC-USD",
		Pair:      "BTC/USD",
		Price:     5000000000000,
		Decimals:  8,
		Timestamp: time.Now().Add(-time.Hour),
	}
	feed2 := &PriceFeed{
		FeedID:    "BTC-USD",
		Pair:      "BTC/USD",
		Price:     5100000000000,
		Decimals:  8,
		Timestamp: time.Now(),
	}
	repo.CreatePriceFeed(ctx, feed1)
	repo.CreatePriceFeed(ctx, feed2)

	// GetLatestPrice should return the most recent
	latest, err := repo.GetLatestPrice(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("GetLatestPrice() error = %v", err)
	}
	if latest.Price != 5100000000000 {
		t.Errorf("GetLatestPrice() should return most recent price, got %v", latest.Price)
	}
}

func TestMockRepository_ServiceRequestOperations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Test CreateServiceRequest
		req := &ServiceRequest{
			UserID:      "user-123",
			ServiceType: "neocompute",
			Status:      "pending",
		}
	if err := repo.CreateServiceRequest(ctx, req); err != nil {
		t.Fatalf("CreateServiceRequest() error = %v", err)
	}

	// Test GetServiceRequests
	list, err := repo.GetServiceRequests(ctx, "user-123", 10)
	if err != nil {
		t.Fatalf("GetServiceRequests() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("GetServiceRequests() len = %v, want 1", len(list))
	}

	// Test UpdateServiceRequest
	req.Status = "completed"
	if err := repo.UpdateServiceRequest(ctx, req); err != nil {
		t.Fatalf("UpdateServiceRequest() error = %v", err)
	}
}

// Benchmark tests
func BenchmarkMockRepository_CreateUser(b *testing.B) {
	repo := NewMockRepository()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.CreateUser(ctx, &User{
			Address: "NXV7ZhHiyM1aHXwpVsRZC6BN3y4gABn6gR",
			Email:   "test@example.com",
		})
	}
}

func BenchmarkMockRepository_GetUser(b *testing.B) {
	repo := NewMockRepository()
	ctx := context.Background()
	user := &User{Address: "test"}
	repo.CreateUser(ctx, user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetUser(ctx, user.ID)
	}
}
