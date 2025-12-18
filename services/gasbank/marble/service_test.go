package neogasbank

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
)

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()

	svc, err := New(Config{
		Marble: m,
		DB:     mockDB,
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

func TestNewNilMarble(t *testing.T) {
	_, err := New(Config{Marble: nil})
	if err == nil {
		t.Error("New() expected error for nil marble")
	}
}

func TestServiceConstants(t *testing.T) {
	if ServiceID != "neogasbank" {
		t.Errorf("ServiceID = %s, want neogasbank", ServiceID)
	}
	if ServiceName != "NeoGasBank Service" {
		t.Errorf("ServiceName = %s, want NeoGasBank Service", ServiceName)
	}
	if Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", Version)
	}
	if GASContractHash != "0xd2a4cff31913016155e38e474a2c06d08be276cf" {
		t.Errorf("GASContractHash mismatch")
	}
}

func TestDeductFeeValidation(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()

	// Empty user ID
	resp, err := svc.DeductFee(ctx, &DeductFeeRequest{UserID: "", Amount: 100, ServiceID: "test"})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if resp.Success {
		t.Error("DeductFee() expected failure for empty user_id")
	}

	// Zero amount
	resp, err = svc.DeductFee(ctx, &DeductFeeRequest{UserID: "user1", Amount: 0, ServiceID: "test"})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if resp.Success {
		t.Error("DeductFee() expected failure for zero amount")
	}

	// Empty service ID
	resp, err = svc.DeductFee(ctx, &DeductFeeRequest{UserID: "user1", Amount: 100, ServiceID: ""})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if resp.Success {
		t.Error("DeductFee() expected failure for empty service_id")
	}
}

func TestDeductFeeInsufficientBalance(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()

	// Create account with low balance
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  50,
		Reserved: 0,
	})

	resp, err := svc.DeductFee(ctx, &DeductFeeRequest{
		UserID:    "user1",
		Amount:    100,
		ServiceID: "neofeeds",
	})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if resp.Success {
		t.Error("DeductFee() expected failure for insufficient balance")
	}
	if resp.Error == "" {
		t.Error("DeductFee() expected error message")
	}
}

func TestDeductFeeSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()

	// Create account with sufficient balance
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 0,
	})

	resp, err := svc.DeductFee(ctx, &DeductFeeRequest{
		UserID:      "user1",
		Amount:      100,
		ServiceID:   "neofeeds",
		ReferenceID: "ref123",
	})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if !resp.Success {
		t.Errorf("DeductFee() expected success, got error: %s", resp.Error)
	}
	if resp.BalanceAfter != 900 {
		t.Errorf("BalanceAfter = %d, want 900", resp.BalanceAfter)
	}
	if resp.TransactionID == "" {
		t.Error("TransactionID should not be empty")
	}
}

func TestReserveFundsValidation(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()

	// Empty user ID
	resp, err := svc.ReserveFunds(ctx, &ReserveFundsRequest{UserID: "", Amount: 100})
	if err != nil {
		t.Fatalf("ReserveFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReserveFunds() expected failure for empty user_id")
	}

	// Zero amount
	resp, err = svc.ReserveFunds(ctx, &ReserveFundsRequest{UserID: "user1", Amount: 0})
	if err != nil {
		t.Fatalf("ReserveFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReserveFunds() expected failure for zero amount")
	}
}

func TestReserveFundsSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()

	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 0,
	})

	resp, err := svc.ReserveFunds(ctx, &ReserveFundsRequest{
		UserID: "user1",
		Amount: 200,
	})
	if err != nil {
		t.Fatalf("ReserveFunds() error = %v", err)
	}
	if !resp.Success {
		t.Error("ReserveFunds() expected success")
	}
	if resp.Reserved != 200 {
		t.Errorf("Reserved = %d, want 200", resp.Reserved)
	}
}

func TestReleaseFundsValidation(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()

	resp, err := svc.ReleaseFunds(ctx, &ReleaseFundsRequest{UserID: "", Amount: 100})
	if err != nil {
		t.Fatalf("ReleaseFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReleaseFunds() expected failure for empty user_id")
	}
}

func TestGetAccountCreatesNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()

	account, err := svc.GetAccount(ctx, "newuser")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if account.UserID != "newuser" {
		t.Errorf("UserID = %s, want newuser", account.UserID)
	}
	if account.Balance != 0 {
		t.Errorf("Balance = %d, want 0", account.Balance)
	}
}

func TestHandleDeductFeeNoServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	body, _ := json.Marshal(DeductFeeRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/deduct", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-Service-ID header

	w := httptest.NewRecorder()
	svc.handleDeductFee(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleDeductFeeWithServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 0,
	})

	body, _ := json.Marshal(DeductFeeRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/deduct", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleDeductFee(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestDepositStatusConstants(t *testing.T) {
	statuses := []DepositStatus{
		DepositStatusPending,
		DepositStatusConfirming,
		DepositStatusConfirmed,
		DepositStatusFailed,
		DepositStatusExpired,
	}

	expected := []string{"pending", "confirming", "confirmed", "failed", "expired"}
	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("DepositStatus = %s, want %s", s, expected[i])
		}
	}
}

func TestTransactionTypeConstants(t *testing.T) {
	types := []TransactionType{
		TxTypeDeposit,
		TxTypeWithdraw,
		TxTypeServiceFee,
		TxTypeRefund,
	}

	expected := []string{"deposit", "withdraw", "service_fee", "refund"}
	for i, tt := range types {
		if string(tt) != expected[i] {
			t.Errorf("TransactionType = %s, want %s", tt, expected[i])
		}
	}
}

func TestTypesJSONSerialization(t *testing.T) {
	resp := GetAccountResponse{
		ID:        "acc1",
		UserID:    "user1",
		Balance:   1000000000,
		Reserved:  500000000,
		Available: 500000000,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Verify string serialization for int64 fields
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	// With json:",string" tag, these should be strings
	if _, ok := raw["balance"].(string); !ok {
		t.Error("balance should be serialized as string")
	}
	if _, ok := raw["reserved"].(string); !ok {
		t.Error("reserved should be serialized as string")
	}
}

func TestHandleGetAccountNoUserID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	// No X-User-ID header

	w := httptest.NewRecorder()
	svc.handleGetAccount(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleGetAccountSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 100,
	})

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.Header.Set("X-User-ID", "user1")

	w := httptest.NewRecorder()
	svc.handleGetAccount(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleReserveFundsNoServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	body, _ := json.Marshal(ReserveFundsRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/reserve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	svc.handleReserveFunds(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleReserveFundsSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 0,
	})

	body, _ := json.Marshal(ReserveFundsRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/reserve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleReserveFunds(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleReleaseFundsNoServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	body, _ := json.Marshal(ReleaseFundsRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/release", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	svc.handleReleaseFunds(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleReleaseFundsSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 200,
	})

	body, _ := json.Marshal(ReleaseFundsRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/release", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleReleaseFunds(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestReleaseFundsWithCommit(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 200,
	})

	resp, err := svc.ReleaseFunds(ctx, &ReleaseFundsRequest{
		UserID: "user1",
		Amount: 100,
		Commit: true,
	})
	if err != nil {
		t.Fatalf("ReleaseFunds() error = %v", err)
	}
	if !resp.Success {
		t.Error("ReleaseFunds() expected success")
	}
	if resp.BalanceAfter != 900 {
		t.Errorf("BalanceAfter = %d, want 900", resp.BalanceAfter)
	}
}

func TestReleaseFundsInsufficientReserved(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 50,
	})

	resp, err := svc.ReleaseFunds(ctx, &ReleaseFundsRequest{
		UserID: "user1",
		Amount: 100,
	})
	if err != nil {
		t.Fatalf("ReleaseFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReleaseFunds() expected failure for insufficient reserved")
	}
}

func TestReserveFundsInsufficientBalance(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  100,
		Reserved: 50,
	})

	resp, err := svc.ReserveFunds(ctx, &ReserveFundsRequest{
		UserID: "user1",
		Amount: 100,
	})
	if err != nil {
		t.Fatalf("ReserveFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReserveFunds() expected failure for insufficient balance")
	}
}

func TestGetAccountEmptyUserID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	_, err := svc.GetAccount(context.Background(), "")
	if err == nil {
		t.Error("GetAccount() expected error for empty user_id")
	}
}

func TestStatistics(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	stats := svc.statistics()
	if stats["chain_connected"] != false {
		t.Error("chain_connected should be false")
	}
	if stats["required_confirmations"] != RequiredConfirmations {
		t.Errorf("required_confirmations = %v, want %d", stats["required_confirmations"], RequiredConfirmations)
	}
}

func TestDeductFeeResponseJSONSerialization(t *testing.T) {
	resp := DeductFeeResponse{
		Success:       true,
		TransactionID: "tx123",
		BalanceAfter:  9000000000000,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["balance_after"].(string); !ok {
		t.Error("balance_after should be serialized as string")
	}
}

func TestTransactionInfoJSONSerialization(t *testing.T) {
	info := TransactionInfo{
		ID:           "tx1",
		TxType:       TxTypeServiceFee,
		Amount:       -100000000,
		BalanceAfter: 900000000,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["amount"].(string); !ok {
		t.Error("amount should be serialized as string")
	}
	if _, ok := raw["balance_after"].(string); !ok {
		t.Error("balance_after should be serialized as string")
	}
}

func TestDepositInfoJSONSerialization(t *testing.T) {
	info := DepositInfo{
		ID:            "dep1",
		Amount:        500000000,
		Status:        DepositStatusConfirmed,
		Confirmations: 1,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["amount"].(string); !ok {
		t.Error("amount should be serialized as string")
	}
}

func TestHandleDeductFeePaymentRequired(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  50,
		Reserved: 0,
	})

	body, _ := json.Marshal(DeductFeeRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/deduct", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleDeductFee(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Errorf("status = %d, want %d", w.Code, http.StatusPaymentRequired)
	}
}

func TestHandleReserveFundsPaymentRequired(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  50,
		Reserved: 0,
	})

	body, _ := json.Marshal(ReserveFundsRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/reserve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleReserveFunds(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Errorf("status = %d, want %d", w.Code, http.StatusPaymentRequired)
	}
}

func TestHandleReleaseFundsBadRequest(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 50,
	})

	body, _ := json.Marshal(ReleaseFundsRequest{UserID: "user1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/release", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleReleaseFunds(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleGetTransactionsNoUserID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest(http.MethodGet, "/transactions", nil)

	w := httptest.NewRecorder()
	svc.handleGetTransactions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleGetTransactionsSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 0,
	})

	req := httptest.NewRequest(http.MethodGet, "/transactions", nil)
	req.Header.Set("X-User-ID", "user1")

	w := httptest.NewRecorder()
	svc.handleGetTransactions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleGetDepositsNoUserID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest(http.MethodGet, "/deposits", nil)

	w := httptest.NewRecorder()
	svc.handleGetDeposits(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleGetDepositsSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest(http.MethodGet, "/deposits", nil)
	req.Header.Set("X-User-ID", "user1")

	w := httptest.NewRecorder()
	svc.handleGetDeposits(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProcessDepositVerificationNoChainClient(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	// Should not panic with nil chain client
	svc.processDepositVerification(context.Background())
}

func TestGetPendingDepositsNilDB(t *testing.T) {
	svc := &Service{
		BaseService: nil,
		db:          nil,
	}

	deposits, err := svc.getPendingDeposits(context.Background())
	if err != nil {
		t.Errorf("getPendingDeposits() error = %v", err)
	}
	if deposits != nil {
		t.Error("deposits should be nil")
	}
}

func TestVerifyTransactionNoChainClient(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	_, _, err := svc.verifyTransaction(context.Background(), "txhash", "addr", 100)
	if err == nil {
		t.Error("verifyTransaction() expected error for nil chain client")
	}
}

func TestCleanupExpiredDeposits(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	// Should not panic
	svc.cleanupExpiredDeposits(context.Background())
}

func TestDepositCheckIntervalConstant(t *testing.T) {
	expected := 15000000000 // 15 seconds in nanoseconds
	if int64(DepositCheckInterval) != int64(expected) {
		t.Errorf("DepositCheckInterval mismatch")
	}
}

func TestDepositExpirationTimeConstant(t *testing.T) {
	expected := 86400000000000 // 24 hours in nanoseconds
	if int64(DepositExpirationTime) != int64(expected) {
		t.Errorf("DepositExpirationTime mismatch")
	}
}

func TestMaxPendingDepositsPerRunConstant(t *testing.T) {
	if MaxPendingDepositsPerRun != 100 {
		t.Errorf("MaxPendingDepositsPerRun = %d, want 100", MaxPendingDepositsPerRun)
	}
}

func TestRequiredConfirmationsConstant(t *testing.T) {
	if RequiredConfirmations != 1 {
		t.Errorf("RequiredConfirmations = %d, want 1", RequiredConfirmations)
	}
}

func TestHandleDeductFeeInvalidJSON(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest(http.MethodPost, "/deduct", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleDeductFee(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleReserveFundsInvalidJSON(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest(http.MethodPost, "/reserve", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleReserveFunds(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleReleaseFundsInvalidJSON(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	req := httptest.NewRequest(http.MethodPost, "/release", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", "neofeeds")

	w := httptest.NewRecorder()
	svc.handleReleaseFunds(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestReleaseFundsValidationEmptyUserID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	resp, err := svc.ReleaseFunds(context.Background(), &ReleaseFundsRequest{
		UserID: "",
		Amount: 100,
	})
	if err != nil {
		t.Fatalf("ReleaseFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReleaseFunds() expected failure for empty user_id")
	}
}

func TestReleaseFundsValidationZeroAmount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	resp, err := svc.ReleaseFunds(context.Background(), &ReleaseFundsRequest{
		UserID: "user1",
		Amount: 0,
	})
	if err != nil {
		t.Fatalf("ReleaseFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReleaseFunds() expected failure for zero amount")
	}
}

func TestDeductFeeNegativeAmount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	resp, err := svc.DeductFee(context.Background(), &DeductFeeRequest{
		UserID:    "user1",
		Amount:    -100,
		ServiceID: "test",
	})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if resp.Success {
		t.Error("DeductFee() expected failure for negative amount")
	}
}

func TestReserveFundsNegativeAmount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	resp, err := svc.ReserveFunds(context.Background(), &ReserveFundsRequest{
		UserID: "user1",
		Amount: -100,
	})
	if err != nil {
		t.Fatalf("ReserveFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReserveFunds() expected failure for negative amount")
	}
}

func TestReleaseFundsNegativeAmount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	resp, err := svc.ReleaseFunds(context.Background(), &ReleaseFundsRequest{
		UserID: "user1",
		Amount: -100,
	})
	if err != nil {
		t.Fatalf("ReleaseFunds() error = %v", err)
	}
	if resp.Success {
		t.Error("ReleaseFunds() expected failure for negative amount")
	}
}

func TestGetPendingDepositsWithDB(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	deposits, err := svc.getPendingDeposits(context.Background())
	if err != nil {
		t.Errorf("getPendingDeposits() error = %v", err)
	}
	if deposits == nil {
		// Empty slice is fine
	}
}

func TestProcessDepositVerificationNilDB(t *testing.T) {
	svc := &Service{
		BaseService: nil,
		chainClient: nil,
		db:          nil,
	}
	// Should not panic
	svc.processDepositVerification(context.Background())
}

func TestReserveFundsResponseFields(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 100,
	})

	resp, _ := svc.ReserveFunds(ctx, &ReserveFundsRequest{
		UserID: "user1",
		Amount: 200,
	})
	if !resp.Success {
		t.Error("ReserveFunds() expected success")
	}
	if resp.Reserved != 300 {
		t.Errorf("Reserved = %d, want 300", resp.Reserved)
	}
	if resp.BalanceAfter != 1000 {
		t.Errorf("BalanceAfter = %d, want 1000", resp.BalanceAfter)
	}
}

func TestReleaseFundsResponseFields(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 300,
	})

	resp, _ := svc.ReleaseFunds(ctx, &ReleaseFundsRequest{
		UserID: "user1",
		Amount: 100,
		Commit: false,
	})
	if !resp.Success {
		t.Error("ReleaseFunds() expected success")
	}
	if resp.BalanceAfter != 1000 {
		t.Errorf("BalanceAfter = %d, want 1000 (no commit)", resp.BalanceAfter)
	}
}

func TestDeductFeeTransactionRecorded(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 0,
	})

	resp, err := svc.DeductFee(ctx, &DeductFeeRequest{
		UserID:      "user1",
		Amount:      100,
		ServiceID:   "neofeeds",
		ReferenceID: "ref123",
		Description: "test fee",
	})
	if err != nil {
		t.Fatalf("DeductFee() error = %v", err)
	}
	if !resp.Success {
		t.Errorf("DeductFee() expected success, got error: %s", resp.Error)
	}
	if resp.TransactionID == "" {
		t.Error("TransactionID should not be empty")
	}
}

func TestGetAccountResponseFields(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 200,
	})

	resp, err := svc.GetAccount(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if resp.ID != "acc1" {
		t.Errorf("ID = %s, want acc1", resp.ID)
	}
	if resp.UserID != "user1" {
		t.Errorf("UserID = %s, want user1", resp.UserID)
	}
	if resp.Balance != 1000 {
		t.Errorf("Balance = %d, want 1000", resp.Balance)
	}
	if resp.Reserved != 200 {
		t.Errorf("Reserved = %d, want 200", resp.Reserved)
	}
	if resp.Available != 800 {
		t.Errorf("Available = %d, want 800", resp.Available)
	}
}

func TestDeductFeeErrorMessage(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  50,
		Reserved: 0,
	})

	resp, _ := svc.DeductFee(ctx, &DeductFeeRequest{
		UserID:    "user1",
		Amount:    100,
		ServiceID: "neofeeds",
	})
	if resp.Success {
		t.Error("DeductFee() expected failure")
	}
	if resp.Error == "" {
		t.Error("Error message should not be empty")
	}
	if resp.BalanceAfter != 50 {
		t.Errorf("BalanceAfter = %d, want 50", resp.BalanceAfter)
	}
}

func TestReserveFundsBalanceAfterOnFailure(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  100,
		Reserved: 80,
	})

	resp, _ := svc.ReserveFunds(ctx, &ReserveFundsRequest{
		UserID: "user1",
		Amount: 50,
	})
	if resp.Success {
		t.Error("ReserveFunds() expected failure")
	}
	if resp.BalanceAfter != 100 {
		t.Errorf("BalanceAfter = %d, want 100", resp.BalanceAfter)
	}
}

func TestReleaseFundsBalanceAfterOnFailure(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: ServiceID})
	mockDB := database.NewMockRepository()
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	mockDB.CreateGasBankAccount(ctx, &database.GasBankAccount{
		ID:       "acc1",
		UserID:   "user1",
		Balance:  1000,
		Reserved: 50,
	})

	resp, _ := svc.ReleaseFunds(ctx, &ReleaseFundsRequest{
		UserID: "user1",
		Amount: 100,
	})
	if resp.Success {
		t.Error("ReleaseFunds() expected failure")
	}
	if resp.BalanceAfter != 1000 {
		t.Errorf("BalanceAfter = %d, want 1000", resp.BalanceAfter)
	}
}
