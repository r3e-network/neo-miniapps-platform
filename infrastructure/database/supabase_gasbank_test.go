package database

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// =============================================================================
// Gas Bank Account Tests
// =============================================================================

func TestCreateGasBankAccountNil(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.CreateGasBankAccount(context.Background(), nil)
	if err == nil {
		t.Error("CreateGasBankAccount() should return error for nil account")
	}
}

func TestCreateGasBankAccountInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	account := &GasBankAccount{UserID: ""}
	err := repo.CreateGasBankAccount(context.Background(), account)
	if err == nil {
		t.Error("CreateGasBankAccount() should return error for empty user ID")
	}
}

func TestCreateGasBankAccountSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]GasBankAccount{{ID: "account-1", UserID: "user-123"}})
	})
	defer cleanup()

	account := &GasBankAccount{UserID: "user-123"}
	err := repo.CreateGasBankAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("CreateGasBankAccount() error = %v", err)
	}
	if account.ID != "account-1" {
		t.Errorf("account.ID = %q, want %q", account.ID, "account-1")
	}
}

func TestCreateGasBankAccountRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	account := &GasBankAccount{UserID: "user-123"}
	err := repo.CreateGasBankAccount(context.Background(), account)
	if err == nil {
		t.Error("CreateGasBankAccount() should return error on server error")
	}
}

func TestCreateGasBankAccountUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	account := &GasBankAccount{UserID: "user-123"}
	err := repo.CreateGasBankAccount(context.Background(), account)
	if err == nil {
		t.Error("CreateGasBankAccount() should return error for invalid JSON")
	}
}

func TestGetOrCreateGasBankAccountInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetOrCreateGasBankAccount(context.Background(), "")
	if err == nil {
		t.Error("GetOrCreateGasBankAccount() should return error for empty user ID")
	}
}

func TestGetOrCreateGasBankAccountExisting(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Return existing account on GET
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]GasBankAccount{{ID: "existing-account", UserID: "user-123"}})
	})
	defer cleanup()

	account, err := repo.GetOrCreateGasBankAccount(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("GetOrCreateGasBankAccount() error = %v", err)
	}
	if account.ID != "existing-account" {
		t.Errorf("account.ID = %q, want %q", account.ID, "existing-account")
	}
}

func TestGetOrCreateGasBankAccountCreateNew(t *testing.T) {
	callCount := 0
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// First call: GET returns empty (not found)
			json.NewEncoder(w).Encode([]GasBankAccount{})
		} else {
			// Second call: POST creates new account
			json.NewEncoder(w).Encode([]GasBankAccount{{ID: "new-account", UserID: "user-123"}})
		}
	})
	defer cleanup()

	account, err := repo.GetOrCreateGasBankAccount(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("GetOrCreateGasBankAccount() error = %v", err)
	}
	if account.ID != "new-account" {
		t.Errorf("account.ID = %q, want %q", account.ID, "new-account")
	}
}

func TestGetOrCreateGasBankAccountGetError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Return server error (not a "not found" error)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetOrCreateGasBankAccount(context.Background(), "user-123")
	if err == nil {
		t.Error("GetOrCreateGasBankAccount() should return error on server error")
	}
}

func TestGetOrCreateGasBankAccountCreateErrorWithFallback(t *testing.T) {
	callCount := 0
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// First GET: not found
			json.NewEncoder(w).Encode([]GasBankAccount{})
		} else if callCount == 2 {
			// POST: fails
			w.WriteHeader(http.StatusConflict)
		} else {
			// Fallback GET: returns existing account
			json.NewEncoder(w).Encode([]GasBankAccount{{ID: "fallback-account", UserID: "user-123"}})
		}
	})
	defer cleanup()

	account, err := repo.GetOrCreateGasBankAccount(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("GetOrCreateGasBankAccount() error = %v", err)
	}
	if account.ID != "fallback-account" {
		t.Errorf("account.ID = %q, want %q", account.ID, "fallback-account")
	}
}

func TestUpdateGasBankBalanceInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateGasBankBalance(context.Background(), "", 100, 0)
	if err == nil {
		t.Error("UpdateGasBankBalance() should return error for empty user ID")
	}
}

func TestUpdateGasBankBalanceNegativeBalance(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateGasBankBalance(context.Background(), "user-123", -100, 0)
	if err == nil {
		t.Error("UpdateGasBankBalance() should return error for negative balance")
	}
}

func TestUpdateGasBankBalanceNegativeReserved(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateGasBankBalance(context.Background(), "user-123", 100, -50)
	if err == nil {
		t.Error("UpdateGasBankBalance() should return error for negative reserved")
	}
}

func TestUpdateGasBankBalanceSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateGasBankBalance(context.Background(), "user-123", 1000, 100)
	if err != nil {
		t.Fatalf("UpdateGasBankBalance() error = %v", err)
	}
}

func TestUpdateGasBankBalanceRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.UpdateGasBankBalance(context.Background(), "user-123", 1000, 100)
	if err == nil {
		t.Error("UpdateGasBankBalance() should return error on server error")
	}
}

// =============================================================================
// Gas Bank Transaction Tests
// =============================================================================

func TestCreateGasBankTransactionNil(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.CreateGasBankTransaction(context.Background(), nil)
	if err == nil {
		t.Error("CreateGasBankTransaction() should return error for nil transaction")
	}
}

func TestCreateGasBankTransactionEmptyID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	tx := &GasBankTransaction{ID: ""}
	err := repo.CreateGasBankTransaction(context.Background(), tx)
	if err == nil {
		t.Error("CreateGasBankTransaction() should return error for empty ID")
	}
}

func TestCreateGasBankTransactionSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	tx := &GasBankTransaction{ID: "tx-123", AccountID: "account-123", Amount: 100}
	err := repo.CreateGasBankTransaction(context.Background(), tx)
	if err != nil {
		t.Fatalf("CreateGasBankTransaction() error = %v", err)
	}
}

func TestCreateGasBankTransactionRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	tx := &GasBankTransaction{ID: "tx-123", AccountID: "account-123", Amount: 100}
	err := repo.CreateGasBankTransaction(context.Background(), tx)
	if err == nil {
		t.Error("CreateGasBankTransaction() should return error on server error")
	}
}

func TestGetGasBankTransactionsInvalidAccountID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetGasBankTransactions(context.Background(), "", 50)
	if err == nil {
		t.Error("GetGasBankTransactions() should return error for empty account ID")
	}
}

func TestGetGasBankTransactionsSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "account_id=eq.") {
			t.Errorf("Query = %s, should contain account_id filter", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]GasBankTransaction{
			{ID: "tx-1", AccountID: "account-123"},
			{ID: "tx-2", AccountID: "account-123"},
		})
	})
	defer cleanup()

	txs, err := repo.GetGasBankTransactions(context.Background(), "account-123", 50)
	if err != nil {
		t.Fatalf("GetGasBankTransactions() error = %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("len(txs) = %d, want 2", len(txs))
	}
}

func TestGetGasBankTransactionsRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetGasBankTransactions(context.Background(), "account-123", 50)
	if err == nil {
		t.Error("GetGasBankTransactions() should return error on server error")
	}
}

func TestGetGasBankTransactionsUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetGasBankTransactions(context.Background(), "account-123", 50)
	if err == nil {
		t.Error("GetGasBankTransactions() should return error for invalid JSON")
	}
}

// =============================================================================
// Deposit Request Tests
// =============================================================================

func TestCreateDepositRequestNil(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.CreateDepositRequest(context.Background(), nil)
	if err == nil {
		t.Error("CreateDepositRequest() should return error for nil deposit")
	}
}

func TestCreateDepositRequestInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	deposit := &DepositRequest{UserID: ""}
	err := repo.CreateDepositRequest(context.Background(), deposit)
	if err == nil {
		t.Error("CreateDepositRequest() should return error for empty user ID")
	}
}

func TestCreateDepositRequestSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]DepositRequest{{ID: "deposit-1", UserID: "user-123"}})
	})
	defer cleanup()

	deposit := &DepositRequest{UserID: "user-123", Amount: 1000}
	err := repo.CreateDepositRequest(context.Background(), deposit)
	if err != nil {
		t.Fatalf("CreateDepositRequest() error = %v", err)
	}
	if deposit.ID != "deposit-1" {
		t.Errorf("deposit.ID = %q, want %q", deposit.ID, "deposit-1")
	}
}

func TestCreateDepositRequestRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	deposit := &DepositRequest{UserID: "user-123", Amount: 1000}
	err := repo.CreateDepositRequest(context.Background(), deposit)
	if err == nil {
		t.Error("CreateDepositRequest() should return error on server error")
	}
}

func TestCreateDepositRequestUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	deposit := &DepositRequest{UserID: "user-123", Amount: 1000}
	err := repo.CreateDepositRequest(context.Background(), deposit)
	if err == nil {
		t.Error("CreateDepositRequest() should return error for invalid JSON")
	}
}

func TestGetDepositRequestsInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetDepositRequests(context.Background(), "", 50)
	if err == nil {
		t.Error("GetDepositRequests() should return error for empty user ID")
	}
}

func TestGetDepositRequestsSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]DepositRequest{
			{ID: "deposit-1", UserID: "user-123"},
			{ID: "deposit-2", UserID: "user-123"},
		})
	})
	defer cleanup()

	deposits, err := repo.GetDepositRequests(context.Background(), "user-123", 50)
	if err != nil {
		t.Fatalf("GetDepositRequests() error = %v", err)
	}
	if len(deposits) != 2 {
		t.Errorf("len(deposits) = %d, want 2", len(deposits))
	}
}

func TestGetDepositRequestsRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetDepositRequests(context.Background(), "user-123", 50)
	if err == nil {
		t.Error("GetDepositRequests() should return error on server error")
	}
}

func TestGetDepositRequestsUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetDepositRequests(context.Background(), "user-123", 50)
	if err == nil {
		t.Error("GetDepositRequests() should return error for invalid JSON")
	}
}

func TestGetDepositByTxHashInvalid(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetDepositByTxHash(context.Background(), "")
	if err == nil {
		t.Error("GetDepositByTxHash() should return error for empty tx hash")
	}
}

func TestGetDepositByTxHashSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]DepositRequest{{ID: "deposit-1", TxHash: "0xabc123"}})
	})
	defer cleanup()

	deposit, err := repo.GetDepositByTxHash(context.Background(), "0xabc123")
	if err != nil {
		t.Fatalf("GetDepositByTxHash() error = %v", err)
	}
	if deposit.ID != "deposit-1" {
		t.Errorf("deposit.ID = %q, want %q", deposit.ID, "deposit-1")
	}
}

func TestGetDepositByTxHashNotFound(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]DepositRequest{})
	})
	defer cleanup()

	// Use a valid hex format that doesn't exist
	_, err := repo.GetDepositByTxHash(context.Background(), "0xabcdef1234567890abcdef1234567890")
	if err == nil {
		t.Error("GetDepositByTxHash() should return error for not found")
	}
	if !IsNotFound(err) {
		t.Errorf("error should be NotFoundError, got: %v", err)
	}
}

func TestGetDepositByTxHashRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetDepositByTxHash(context.Background(), "0xabc123")
	if err == nil {
		t.Error("GetDepositByTxHash() should return error on server error")
	}
}

func TestGetDepositByTxHashUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetDepositByTxHash(context.Background(), "0xabc123")
	if err == nil {
		t.Error("GetDepositByTxHash() should return error for invalid JSON")
	}
}

func TestUpdateDepositStatusInvalidID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateDepositStatus(context.Background(), "", "confirmed", 6)
	if err == nil {
		t.Error("UpdateDepositStatus() should return error for empty ID")
	}
}

func TestUpdateDepositStatusInvalidStatus(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateDepositStatus(context.Background(), "deposit-123", "invalid-status", 6)
	if err == nil {
		t.Error("UpdateDepositStatus() should return error for invalid status")
	}
}

func TestUpdateDepositStatusSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateDepositStatus(context.Background(), "deposit-123", "confirmed", 6)
	if err != nil {
		t.Fatalf("UpdateDepositStatus() error = %v", err)
	}
}

func TestUpdateDepositStatusPending(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateDepositStatus(context.Background(), "deposit-123", "pending", 0)
	if err != nil {
		t.Fatalf("UpdateDepositStatus() error = %v", err)
	}
}

func TestUpdateDepositStatusRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.UpdateDepositStatus(context.Background(), "deposit-123", "confirmed", 6)
	if err == nil {
		t.Error("UpdateDepositStatus() should return error on server error")
	}
}
