package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/R3E-Network/service_layer/domain/gasbank"
	"github.com/R3E-Network/service_layer/domain/oracle"
	"github.com/R3E-Network/service_layer/domain/secret"
)

func TestStoreCoreIntegration(t *testing.T) {
	store, ctx := newTestStore(t)

	acct, err := store.CreateAccount(ctx, account.Account{Owner: "owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	fn, err := store.CreateFunction(ctx, function.Definition{AccountID: acct.ID, Name: "fn", Source: "() => 1"})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}
	_ = fn

	gasAcct, err := store.CreateGasAccount(ctx, gasbank.Account{
		AccountID:     acct.ID,
		WalletAddress: "NeOWallet",
	})
	if err != nil {
		t.Fatalf("create gas account: %v", err)
	}
	if gasAcct.ID == "" {
		t.Fatalf("expected gas account id to be set")
	}
	if gasAcct.CreatedAt.IsZero() || gasAcct.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set on gas account")
	}
	if gasAcct.WalletAddress != strings.ToLower("NeOWallet") {
		t.Fatalf("wallet normalisation mismatch: %q", gasAcct.WalletAddress)
	}

	reloadedAcct, err := store.GetGasAccount(ctx, gasAcct.ID)
	if err != nil {
		t.Fatalf("get gas account: %v", err)
	}
	if reloadedAcct.ID != gasAcct.ID {
		t.Fatalf("expected matching gas account id")
	}

	byWallet, err := store.GetGasAccountByWallet(ctx, "NEOWALLET")
	if err != nil {
		t.Fatalf("get gas account by wallet: %v", err)
	}
	if byWallet.ID != gasAcct.ID {
		t.Fatalf("expected wallet lookup to match gas account")
	}

	accts, err := store.ListGasAccounts(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list gas accounts: %v", err)
	}
	if len(accts) != 1 {
		t.Fatalf("expected single gas account, got %d", len(accts))
	}

	gasAcct.Balance = 10
	gasAcct.Available = 8
	gasAcct.Pending = 2
	gasAcct.DailyWithdrawal = 5
	gasAcct.LastWithdrawal = time.Now().UTC()
	gasAcct, err = store.UpdateGasAccount(ctx, gasAcct)
	if err != nil {
		t.Fatalf("update gas account: %v", err)
	}
	if gasAcct.UpdatedAt.Before(gasAcct.CreatedAt) {
		t.Fatalf("expected updated_at to be after created_at")
	}

	deposit := gasbank.Transaction{
		AccountID:      gasAcct.ID,
		UserAccountID:  acct.ID,
		Type:           gasbank.TransactionDeposit,
		Amount:         10,
		NetAmount:      10,
		Status:         gasbank.StatusCompleted,
		BlockchainTxID: "hash1",
		FromAddress:    "wallet-a",
		ToAddress:      "wallet-b",
	}
	deposit, err = store.CreateGasTransaction(ctx, deposit)
	if err != nil {
		t.Fatalf("create deposit transaction: %v", err)
	}
	if deposit.CreatedAt.IsZero() || deposit.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set on deposit")
	}

	txs, err := store.ListGasTransactions(ctx, gasAcct.ID, 10)
	if err != nil {
		t.Fatalf("list gas transactions: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected single gas transaction, got %d", len(txs))
	}

	withdraw := gasbank.Transaction{
		AccountID:     gasAcct.ID,
		UserAccountID: acct.ID,
		Type:          gasbank.TransactionWithdrawal,
		Amount:        4,
		NetAmount:     4,
		Status:        gasbank.StatusPending,
		ToAddress:     "wallet-c",
	}
	withdraw, err = store.CreateGasTransaction(ctx, withdraw)
	if err != nil {
		t.Fatalf("create withdrawal transaction: %v", err)
	}

	pending, err := store.ListPendingWithdrawals(ctx)
	if err != nil {
		t.Fatalf("list pending withdrawals: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != withdraw.ID {
		t.Fatalf("expected one pending withdrawal matching created transaction")
	}

	withdraw.Status = gasbank.StatusCompleted
	withdraw.CompletedAt = time.Now().UTC()
	withdraw.NetAmount = withdraw.Amount
	withdraw, err = store.UpdateGasTransaction(ctx, withdraw)
	if err != nil {
		t.Fatalf("update withdrawal transaction: %v", err)
	}
	if withdraw.Status != gasbank.StatusCompleted {
		t.Fatalf("expected status to remain completed")
	}

	pending, err = store.ListPendingWithdrawals(ctx)
	if err != nil {
		t.Fatalf("list pending withdrawals: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending withdrawals after settlement, got %d", len(pending))
	}

	wallet, err := store.CreateWorkspaceWallet(ctx, account.WorkspaceWallet{
		WorkspaceID:   acct.ID,
		WalletAddress: "0xabc123abc123abc123abc123abc123abc123abcd",
		Label:         "primary",
		Status:        "active",
	})
	if err != nil {
		t.Fatalf("create workspace wallet: %v", err)
	}
	if _, err := store.FindWorkspaceWalletByAddress(ctx, acct.ID, wallet.WalletAddress); err != nil {
		t.Fatalf("find wallet by address: %v", err)
	}
}

func TestStoreSecretsIntegration(t *testing.T) {
	store, ctx := newTestStore(t)

	acct, err := store.CreateAccount(ctx, account.Account{Owner: "secret-owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Create secret
	sec, err := store.CreateSecret(ctx, secret.Secret{
		AccountID: acct.ID,
		Name:      "API_KEY",
		Value:     "encrypted-secret-value",
	})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if sec.ID == "" {
		t.Fatalf("expected secret ID to be set")
	}
	if sec.Version != 1 {
		t.Fatalf("expected version 1, got %d", sec.Version)
	}
	if sec.CreatedAt.IsZero() || sec.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set")
	}

	// Get secret by name (case insensitive)
	retrieved, err := store.GetSecret(ctx, acct.ID, "api_key")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if retrieved.ID != sec.ID {
		t.Fatalf("expected ID %q, got %q", sec.ID, retrieved.ID)
	}
	if retrieved.Value != "encrypted-secret-value" {
		t.Fatalf("expected value 'encrypted-secret-value', got %q", retrieved.Value)
	}

	// Update secret
	sec.Value = "new-encrypted-value"
	updated, err := store.UpdateSecret(ctx, sec)
	if err != nil {
		t.Fatalf("update secret: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version 2 after update, got %d", updated.Version)
	}
	if updated.Value != "new-encrypted-value" {
		t.Fatalf("expected updated value")
	}

	// List secrets
	secrets, err := store.ListSecrets(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list secrets: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}

	// Delete secret
	if err := store.DeleteSecret(ctx, acct.ID, "API_KEY"); err != nil {
		t.Fatalf("delete secret: %v", err)
	}

	// Verify deletion
	secrets, err = store.ListSecrets(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list secrets after delete: %v", err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets after deletion, got %d", len(secrets))
	}
}

func TestStoreOracleIntegration(t *testing.T) {
	store, ctx := newTestStore(t)

	acct, err := store.CreateAccount(ctx, account.Account{Owner: "oracle-owner"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Create data source
	src, err := store.CreateDataSource(ctx, oracle.DataSource{
		AccountID:   acct.ID,
		Name:        "weather-api",
		Description: "Weather data provider",
		URL:         "https://api.weather.example/v1/data",
		Method:      "GET",
		Headers:     map[string]string{"Authorization": "Bearer xxx"},
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create data source: %v", err)
	}
	if src.ID == "" {
		t.Fatalf("expected data source ID to be set")
	}

	// Get data source
	retrieved, err := store.GetDataSource(ctx, src.ID)
	if err != nil {
		t.Fatalf("get data source: %v", err)
	}
	if retrieved.Name != "weather-api" {
		t.Fatalf("expected name 'weather-api', got %q", retrieved.Name)
	}
	if retrieved.Headers["Authorization"] != "Bearer xxx" {
		t.Fatalf("expected Authorization header preserved")
	}

	// Update data source
	src.Description = "Updated weather provider"
	src.Enabled = false
	updated, err := store.UpdateDataSource(ctx, src)
	if err != nil {
		t.Fatalf("update data source: %v", err)
	}
	if updated.Description != "Updated weather provider" {
		t.Fatalf("expected updated description")
	}
	if updated.Enabled {
		t.Fatalf("expected enabled to be false")
	}

	// List data sources
	sources, err := store.ListDataSources(ctx, acct.ID)
	if err != nil {
		t.Fatalf("list data sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 data source, got %d", len(sources))
	}

	// Create oracle request
	req, err := store.CreateRequest(ctx, oracle.Request{
		AccountID:    acct.ID,
		DataSourceID: src.ID,
		Status:       oracle.StatusPending,
		Payload:      `{"city": "tokyo"}`,
	})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if req.ID == "" {
		t.Fatalf("expected request ID to be set")
	}

	// Get request
	retrievedReq, err := store.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if retrievedReq.Status != oracle.StatusPending {
		t.Fatalf("expected status 'pending', got %q", retrievedReq.Status)
	}

	// List pending requests
	pending, err := store.ListPendingRequests(ctx)
	if err != nil {
		t.Fatalf("list pending requests: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}

	// Update request to succeeded
	req.Status = oracle.StatusSucceeded
	req.Result = `{"temp": 25, "humidity": 60}`
	req.CompletedAt = time.Now().UTC()
	updated2, err := store.UpdateRequest(ctx, req)
	if err != nil {
		t.Fatalf("update request: %v", err)
	}
	if updated2.Status != oracle.StatusSucceeded {
		t.Fatalf("expected status 'succeeded', got %q", updated2.Status)
	}
	if updated2.CompletedAt.IsZero() {
		t.Fatalf("expected completed_at to be set")
	}

	// List requests with status filter
	requests, err := store.ListRequests(ctx, acct.ID, 10, string(oracle.StatusSucceeded))
	if err != nil {
		t.Fatalf("list requests with filter: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 succeeded request, got %d", len(requests))
	}

	// Verify no more pending
	pending, err = store.ListPendingRequests(ctx)
	if err != nil {
		t.Fatalf("list pending requests: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending requests after completion, got %d", len(pending))
	}
}
