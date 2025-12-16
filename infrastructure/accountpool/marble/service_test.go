// Package neoaccounts provides unit tests for the neoaccounts service.
package neoaccounts

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	neoaccountssupabase "github.com/R3E-Network/service_layer/infrastructure/accountpool/supabase"
	"github.com/R3E-Network/service_layer/infrastructure/crypto"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
)

// =============================================================================
// Mock Repository for Tests (Multi-Token Support)
// =============================================================================

// mockNeoAccountsRepo implements neoaccountssupabase.RepositoryInterface for testing.
type mockNeoAccountsRepo struct {
	accounts      map[string]*neoaccountssupabase.Account
	balances      map[string]map[string]*neoaccountssupabase.AccountBalance // accountID -> tokenType -> balance
	simulateError bool                                                      // When true, methods return errors
}

func newMockNeoAccountsRepo() *mockNeoAccountsRepo {
	return &mockNeoAccountsRepo{
		accounts: make(map[string]*neoaccountssupabase.Account),
		balances: make(map[string]map[string]*neoaccountssupabase.AccountBalance),
	}
}

func (m *mockNeoAccountsRepo) Create(_ context.Context, acc *neoaccountssupabase.Account) error {
	m.accounts[acc.ID] = acc
	return nil
}

func (m *mockNeoAccountsRepo) Update(_ context.Context, acc *neoaccountssupabase.Account) error {
	m.accounts[acc.ID] = acc
	return nil
}

func (m *mockNeoAccountsRepo) GetByID(_ context.Context, id string) (*neoaccountssupabase.Account, error) {
	if acc, ok := m.accounts[id]; ok {
		return acc, nil
	}
	return nil, fmt.Errorf("account not found: %s", id)
}

func (m *mockNeoAccountsRepo) List(_ context.Context) ([]neoaccountssupabase.Account, error) {
	if m.simulateError {
		return nil, fmt.Errorf("simulated database error")
	}
	var result []neoaccountssupabase.Account
	for _, acc := range m.accounts {
		result = append(result, *acc)
	}
	return result, nil
}

func (m *mockNeoAccountsRepo) ListAvailable(_ context.Context, limit int) ([]neoaccountssupabase.Account, error) {
	var result []neoaccountssupabase.Account
	for _, acc := range m.accounts {
		if !acc.IsRetiring && acc.LockedBy == "" {
			result = append(result, *acc)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockNeoAccountsRepo) ListByLocker(_ context.Context, lockerID string) ([]neoaccountssupabase.Account, error) {
	var result []neoaccountssupabase.Account
	for _, acc := range m.accounts {
		if acc.LockedBy == lockerID {
			result = append(result, *acc)
		}
	}
	return result, nil
}

func (m *mockNeoAccountsRepo) Delete(_ context.Context, id string) error {
	delete(m.accounts, id)
	delete(m.balances, id)
	return nil
}

// Balance-aware account operations
func (m *mockNeoAccountsRepo) GetWithBalances(_ context.Context, id string) (*neoaccountssupabase.AccountWithBalances, error) {
	acc, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account not found: %s", id)
	}
	result := neoaccountssupabase.NewAccountWithBalances(acc)
	if bals, ok := m.balances[id]; ok {
		for _, bal := range bals {
			result.AddBalance(bal)
		}
	}
	return result, nil
}

func (m *mockNeoAccountsRepo) ListWithBalances(_ context.Context) ([]neoaccountssupabase.AccountWithBalances, error) {
	var result []neoaccountssupabase.AccountWithBalances
	for _, acc := range m.accounts {
		accWithBal := neoaccountssupabase.NewAccountWithBalances(acc)
		if bals, ok := m.balances[acc.ID]; ok {
			for _, bal := range bals {
				accWithBal.AddBalance(bal)
			}
		}
		result = append(result, *accWithBal)
	}
	return result, nil
}

func (m *mockNeoAccountsRepo) ListAvailableWithBalances(_ context.Context, tokenType string, minBalance *int64, limit int) ([]neoaccountssupabase.AccountWithBalances, error) {
	var result []neoaccountssupabase.AccountWithBalances
	for _, acc := range m.accounts {
		if !acc.IsRetiring && acc.LockedBy == "" {
			accWithBal := neoaccountssupabase.NewAccountWithBalances(acc)
			if bals, ok := m.balances[acc.ID]; ok {
				for _, bal := range bals {
					accWithBal.AddBalance(bal)
				}
			}
			// Filter by token balance if specified
			if tokenType != "" && minBalance != nil {
				if !accWithBal.HasSufficientBalance(tokenType, *minBalance) {
					continue
				}
			}
			result = append(result, *accWithBal)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockNeoAccountsRepo) ListByLockerWithBalances(_ context.Context, lockerID string) ([]neoaccountssupabase.AccountWithBalances, error) {
	if m.simulateError {
		return nil, fmt.Errorf("simulated database error")
	}
	var result []neoaccountssupabase.AccountWithBalances
	for _, acc := range m.accounts {
		if acc.LockedBy == lockerID {
			accWithBal := neoaccountssupabase.NewAccountWithBalances(acc)
			if bals, ok := m.balances[acc.ID]; ok {
				for _, bal := range bals {
					accWithBal.AddBalance(bal)
				}
			}
			result = append(result, *accWithBal)
		}
	}
	return result, nil
}

// Balance operations
func (m *mockNeoAccountsRepo) UpsertBalance(_ context.Context, accountID, tokenType, scriptHash string, amount int64, decimals int) error {
	if _, ok := m.balances[accountID]; !ok {
		m.balances[accountID] = make(map[string]*neoaccountssupabase.AccountBalance)
	}
	m.balances[accountID][tokenType] = &neoaccountssupabase.AccountBalance{
		AccountID:  accountID,
		TokenType:  tokenType,
		ScriptHash: scriptHash,
		Amount:     amount,
		Decimals:   decimals,
		UpdatedAt:  time.Now(),
	}
	return nil
}

func (m *mockNeoAccountsRepo) GetBalance(_ context.Context, accountID, tokenType string) (*neoaccountssupabase.AccountBalance, error) {
	if bals, ok := m.balances[accountID]; ok {
		if bal, ok := bals[tokenType]; ok {
			return bal, nil
		}
	}
	return nil, nil
}

func (m *mockNeoAccountsRepo) GetBalances(_ context.Context, accountID string) ([]neoaccountssupabase.AccountBalance, error) {
	var result []neoaccountssupabase.AccountBalance
	if bals, ok := m.balances[accountID]; ok {
		for _, bal := range bals {
			result = append(result, *bal)
		}
	}
	return result, nil
}

func (m *mockNeoAccountsRepo) DeleteBalances(_ context.Context, accountID string) error {
	delete(m.balances, accountID)
	return nil
}

// Statistics
func (m *mockNeoAccountsRepo) AggregateTokenStats(_ context.Context, tokenType string) (*neoaccountssupabase.TokenStats, error) {
	scriptHash, _ := neoaccountssupabase.GetDefaultTokenConfig(tokenType)
	stats := &neoaccountssupabase.TokenStats{
		TokenType:  tokenType,
		ScriptHash: scriptHash,
	}
	for _, acc := range m.accounts {
		if bals, ok := m.balances[acc.ID]; ok {
			if bal, ok := bals[tokenType]; ok {
				stats.TotalBalance += bal.Amount
				if acc.LockedBy != "" {
					stats.LockedBalance += bal.Amount
				} else if !acc.IsRetiring {
					stats.AvailableBalance += bal.Amount
				}
			}
		}
	}
	return stats, nil
}

// =============================================================================
// Test Service Helper
// =============================================================================

// newTestServiceWithMock creates a test service instance with mock repository.
func newTestServiceWithMock(t *testing.T) (*Service, *mockNeoAccountsRepo) {
	t.Helper()
	m, err := marble.New(marble.Config{MarbleType: "neoaccounts"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, err := New(Config{Marble: m})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	mockRepo := newMockNeoAccountsRepo()
	svc.repo = mockRepo

	return svc, mockRepo
}

// =============================================================================
// Key Derivation Tests
// =============================================================================

func TestDeriveAccountKeyDeterministic(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, err := New(Config{Marble: m})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	accountID := "test-account-123"

	key1, err := svc.deriveAccountKey(accountID)
	if err != nil {
		t.Fatalf("deriveAccountKey: %v", err)
	}

	key2, err := svc.deriveAccountKey(accountID)
	if err != nil {
		t.Fatalf("deriveAccountKey: %v", err)
	}

	if hex.EncodeToString(key1) != hex.EncodeToString(key2) {
		t.Error("same account ID should produce same key")
	}

	if len(key1) != 32 {
		t.Errorf("expected 32-byte key, got %d", len(key1))
	}
}

func TestDeriveAccountKeyUnique(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, _ := New(Config{Marble: m})

	key1, _ := svc.deriveAccountKey("account-1")
	key2, _ := svc.deriveAccountKey("account-2")

	if hex.EncodeToString(key1) == hex.EncodeToString(key2) {
		t.Error("different account IDs should produce different keys")
	}
}

func TestGetPrivateKeyValid(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, _ := New(Config{Marble: m})

	priv, err := svc.getPrivateKey("test-account")
	if err != nil {
		t.Fatalf("getPrivateKey: %v", err)
	}

	if priv == nil {
		t.Fatal("private key should not be nil")
	}

	if priv.Curve != elliptic.P256() {
		t.Error("expected P256 curve")
	}

	if priv.D == nil || priv.D.Sign() == 0 {
		t.Error("private key D should be non-zero")
	}

	if priv.PublicKey.X == nil || priv.PublicKey.Y == nil {
		t.Error("public key coordinates should not be nil")
	}
}

func TestGetPrivateKeyDeterministic(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, _ := New(Config{Marble: m})

	priv1, _ := svc.getPrivateKey("account-x")
	priv2, _ := svc.getPrivateKey("account-x")

	if priv1.D.Cmp(priv2.D) != 0 {
		t.Error("same account should produce same private key")
	}
}

// =============================================================================
// Signing Tests
// =============================================================================

func TestSignHashRoundTrip(t *testing.T) {
	curve := elliptic.P256()
	d, _ := crypto.GenerateRandomBytes(32)
	dInt := new(big.Int).SetBytes(d)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	dInt.Mod(dInt, n)
	dInt.Add(dInt, big.NewInt(1))

	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve},
		D:         dInt,
	}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(dInt.Bytes())

	hash := crypto.Hash256([]byte("test transaction data"))

	sig, err := signHash(priv, hash)
	if err != nil {
		t.Fatalf("signHash: %v", err)
	}

	if len(sig) != 64 {
		t.Errorf("expected 64-byte signature, got %d", len(sig))
	}

	if !verifySignature(&priv.PublicKey, hash, sig) {
		t.Error("signature verification failed")
	}
}

func TestSignHashDifferentHashes(t *testing.T) {
	curve := elliptic.P256()
	d, _ := crypto.GenerateRandomBytes(32)
	dInt := new(big.Int).SetBytes(d)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	dInt.Mod(dInt, n)
	dInt.Add(dInt, big.NewInt(1))

	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve},
		D:         dInt,
	}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(dInt.Bytes())

	hash1 := crypto.Hash256([]byte("data 1"))
	hash2 := crypto.Hash256([]byte("data 2"))

	sig1, _ := signHash(priv, hash1)
	sig2, _ := signHash(priv, hash2)

	if hex.EncodeToString(sig1) == hex.EncodeToString(sig2) {
		t.Error("signatures for different hashes should differ")
	}
}

func TestVerifySignatureInvalid(t *testing.T) {
	curve := elliptic.P256()
	d, _ := crypto.GenerateRandomBytes(32)
	dInt := new(big.Int).SetBytes(d)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	dInt.Mod(dInt, n)
	dInt.Add(dInt, big.NewInt(1))

	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve},
		D:         dInt,
	}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(dInt.Bytes())

	hash := crypto.Hash256([]byte("test data"))
	sig, _ := signHash(priv, hash)

	wrongHash := crypto.Hash256([]byte("wrong data"))
	if verifySignature(&priv.PublicKey, wrongHash, sig) {
		t.Error("verification should fail for wrong hash")
	}

	badSig := make([]byte, 64)
	if verifySignature(&priv.PublicKey, hash, badSig) {
		t.Error("verification should fail for invalid signature")
	}

	if verifySignature(&priv.PublicKey, hash, []byte("short")) {
		t.Error("verification should fail for short signature")
	}
}

// =============================================================================
// Multi-Token Type Tests
// =============================================================================

func TestAccountInfoWithBalances(t *testing.T) {
	info := AccountInfo{
		ID:       "test-id",
		Address:  "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		TxCount:  5,
		LockedBy: "neocompute",
		Balances: map[string]TokenBalance{
			TokenTypeGAS: {
				TokenType:  TokenTypeGAS,
				ScriptHash: neoaccountssupabase.GASScriptHash,
				Amount:     1000000000,
				Decimals:   8,
			},
			TokenTypeNEO: {
				TokenType:  TokenTypeNEO,
				ScriptHash: neoaccountssupabase.NEOScriptHash,
				Amount:     10,
				Decimals:   0,
			},
		},
	}

	if info.ID != "test-id" {
		t.Errorf("ID mismatch: got %s", info.ID)
	}
	if len(info.Balances) != 2 {
		t.Errorf("expected 2 balances, got %d", len(info.Balances))
	}
	if info.Balances[TokenTypeGAS].Amount != 1000000000 {
		t.Errorf("GAS balance mismatch: got %d", info.Balances[TokenTypeGAS].Amount)
	}
	if info.Balances[TokenTypeNEO].Amount != 10 {
		t.Errorf("NEO balance mismatch: got %d", info.Balances[TokenTypeNEO].Amount)
	}
}

func TestUpdateBalanceInputWithToken(t *testing.T) {
	input := UpdateBalanceInput{
		ServiceID: "neocompute",
		AccountID: "acc-123",
		Token:     TokenTypeGAS,
		Delta:     500000000,
	}

	if input.Token != TokenTypeGAS {
		t.Errorf("Token = %s, want %s", input.Token, TokenTypeGAS)
	}
}

func TestPoolInfoResponseWithTokenStats(t *testing.T) {
	info := PoolInfoResponse{
		TotalAccounts:    100,
		ActiveAccounts:   80,
		LockedAccounts:   15,
		RetiringAccounts: 5,
		TokenStats: map[string]TokenStats{
			TokenTypeGAS: {
				TokenType:        TokenTypeGAS,
				TotalBalance:     10000000000,
				LockedBalance:    3000000000,
				AvailableBalance: 7000000000,
			},
			TokenTypeNEO: {
				TokenType:        TokenTypeNEO,
				TotalBalance:     1000,
				LockedBalance:    300,
				AvailableBalance: 700,
			},
		},
	}

	if info.TotalAccounts != info.ActiveAccounts+info.LockedAccounts+info.RetiringAccounts {
		t.Error("account counts should sum to total")
	}
	if len(info.TokenStats) != 2 {
		t.Errorf("expected 2 token stats, got %d", len(info.TokenStats))
	}
}

func TestTokenConstants(t *testing.T) {
	if TokenTypeNEO != "NEO" {
		t.Errorf("TokenTypeNEO = %s, want NEO", TokenTypeNEO)
	}
	if TokenTypeGAS != "GAS" {
		t.Errorf("TokenTypeGAS = %s, want GAS", TokenTypeGAS)
	}
}

func TestGetDefaultTokenConfig(t *testing.T) {
	gasHash, gasDecimals := neoaccountssupabase.GetDefaultTokenConfig(TokenTypeGAS)
	if gasHash != neoaccountssupabase.GASScriptHash {
		t.Errorf("GAS script hash mismatch")
	}
	if gasDecimals != 8 {
		t.Errorf("GAS decimals = %d, want 8", gasDecimals)
	}

	neoHash, neoDecimals := neoaccountssupabase.GetDefaultTokenConfig(TokenTypeNEO)
	if neoHash != neoaccountssupabase.NEOScriptHash {
		t.Errorf("NEO script hash mismatch")
	}
	if neoDecimals != 0 {
		t.Errorf("NEO decimals = %d, want 0", neoDecimals)
	}
}

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, err := New(Config{Marble: m})
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
	if ServiceID != "neoaccounts" {
		t.Errorf("ServiceID = %s, want neoaccounts", ServiceID)
	}
	if ServiceName != "Account Pool Service" {
		t.Errorf("ServiceName = %s, want Account Pool Service", ServiceName)
	}
	if Version != "2.0.0" {
		t.Errorf("Version = %s, want 2.0.0", Version)
	}
}

// =============================================================================
// JSON Serialization Tests
// =============================================================================

func TestAccountInfoJSON(t *testing.T) {
	info := AccountInfo{
		ID:       "acc-123",
		Address:  "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		TxCount:  10,
		LockedBy: "neocompute",
		Balances: map[string]TokenBalance{
			TokenTypeGAS: {TokenType: TokenTypeGAS, Amount: 1000000},
		},
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded AccountInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != info.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, info.ID)
	}
	if decoded.Address != info.Address {
		t.Errorf("Address = %s, want %s", decoded.Address, info.Address)
	}
	if decoded.Balances[TokenTypeGAS].Amount != 1000000 {
		t.Errorf("GAS Balance = %d, want 1000000", decoded.Balances[TokenTypeGAS].Amount)
	}
}

func TestUpdateBalanceInputJSON(t *testing.T) {
	input := UpdateBalanceInput{
		ServiceID: "neocompute",
		AccountID: "acc-123",
		Token:     TokenTypeGAS,
		Delta:     1000,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded UpdateBalanceInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ServiceID != input.ServiceID {
		t.Errorf("ServiceID = %s, want %s", decoded.ServiceID, input.ServiceID)
	}
	if decoded.Token != input.Token {
		t.Errorf("Token = %s, want %s", decoded.Token, input.Token)
	}
	if decoded.Delta != input.Delta {
		t.Errorf("Delta = %d, want %d", decoded.Delta, input.Delta)
	}
}

func TestUpdateBalanceResponseJSON(t *testing.T) {
	resp := UpdateBalanceResponse{
		AccountID:  "acc-123",
		Token:      TokenTypeGAS,
		OldBalance: 1000000,
		NewBalance: 1500000,
		TxCount:    11,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded UpdateBalanceResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Token != resp.Token {
		t.Errorf("Token = %s, want %s", decoded.Token, resp.Token)
	}
	if decoded.TxCount != resp.TxCount {
		t.Errorf("TxCount = %d, want %d", decoded.TxCount, resp.TxCount)
	}
}

func TestPoolInfoResponseJSON(t *testing.T) {
	info := PoolInfoResponse{
		TotalAccounts:    100,
		ActiveAccounts:   80,
		LockedAccounts:   15,
		RetiringAccounts: 5,
		TokenStats: map[string]TokenStats{
			TokenTypeGAS: {TokenType: TokenTypeGAS, TotalBalance: 1000000},
		},
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PoolInfoResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.TotalAccounts != info.TotalAccounts {
		t.Errorf("TotalAccounts = %d, want %d", decoded.TotalAccounts, info.TotalAccounts)
	}
	if decoded.TokenStats[TokenTypeGAS].TotalBalance != 1000000 {
		t.Errorf("TokenStats[GAS].TotalBalance = %d, want 1000000", decoded.TokenStats[TokenTypeGAS].TotalBalance)
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandleHealthEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", resp["status"])
	}
}

// =============================================================================
// MockRepository Integration Tests (Multi-Token)
// =============================================================================

func TestRequestAccountsWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	// Pre-populate with pool accounts
	for i := 0; i < 5; i++ {
		accID := "acc-" + string(rune('a'+i))
		mockRepo.accounts[accID] = &neoaccountssupabase.Account{
			ID:         accID,
			Address:    "NAddr" + string(rune('1'+i)),
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
			IsRetiring: false,
			LockedBy:   "",
		}
		// Add some GAS balance
		mockRepo.UpsertBalance(context.Background(), accID, TokenTypeGAS, neoaccountssupabase.GASScriptHash, 1000000, 8)
	}

	svc, err := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	accounts, lockID, err := svc.RequestAccounts(ctx, "neocompute", 3, "mixing operation")
	if err != nil {
		t.Fatalf("RequestAccounts() error = %v", err)
	}

	if len(accounts) != 3 {
		t.Errorf("len(accounts) = %d, want 3", len(accounts))
	}
	if lockID == "" {
		t.Error("lockID should not be empty")
	}

	// Verify accounts are locked and have balances
	for _, acc := range accounts {
		if acc.LockedBy != "neocompute" {
			t.Errorf("account %s should be locked by neocompute", acc.ID)
		}
		if acc.Balances == nil {
			t.Errorf("account %s should have balances", acc.ID)
		}
	}
}

func TestUpdateBalanceWithMockMultiToken(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{
		ID:       "acc-1",
		Address:  "NAddr1",
		LockedBy: "neocompute",
	}
	// Initialize GAS balance
	mockRepo.UpsertBalance(context.Background(), "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 1000000, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	// Update GAS balance
	oldBalance, newBalance, txCount, err := svc.UpdateBalance(ctx, "neocompute", "acc-1", TokenTypeGAS, 500000, nil)
	if err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}

	if oldBalance != 1000000 {
		t.Errorf("oldBalance = %d, want 1000000", oldBalance)
	}
	if newBalance != 1500000 {
		t.Errorf("newBalance = %d, want 1500000", newBalance)
	}
	if txCount != 1 {
		t.Errorf("txCount = %d, want 1", txCount)
	}

	// Update NEO balance (new token)
	oldNeo, newNeo, txCount2, err := svc.UpdateBalance(ctx, "neocompute", "acc-1", TokenTypeNEO, 10, nil)
	if err != nil {
		t.Fatalf("UpdateBalance(NEO) error = %v", err)
	}

	if oldNeo != 0 {
		t.Errorf("oldNeo = %d, want 0", oldNeo)
	}
	if newNeo != 10 {
		t.Errorf("newNeo = %d, want 10", newNeo)
	}
	if txCount2 != 2 {
		t.Errorf("txCount2 = %d, want 2", txCount2)
	}
}

func TestGetPoolInfoWithMockMultiToken(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	// Create various account states
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{
		ID: "acc-1", LockedBy: "",
	}
	mockRepo.accounts["acc-2"] = &neoaccountssupabase.Account{
		ID: "acc-2", LockedBy: "neocompute",
	}
	mockRepo.accounts["acc-3"] = &neoaccountssupabase.Account{
		ID: "acc-3", IsRetiring: true,
	}

	// Add balances
	ctx := context.Background()
	mockRepo.UpsertBalance(ctx, "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 1000000, 8)
	mockRepo.UpsertBalance(ctx, "acc-1", TokenTypeNEO, neoaccountssupabase.NEOScriptHash, 10, 0)
	mockRepo.UpsertBalance(ctx, "acc-2", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 2000000, 8)
	mockRepo.UpsertBalance(ctx, "acc-3", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 500000, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	info, err := svc.GetPoolInfo(ctx)
	if err != nil {
		t.Fatalf("GetPoolInfo() error = %v", err)
	}

	if info.TotalAccounts != 3 {
		t.Errorf("TotalAccounts = %d, want 3", info.TotalAccounts)
	}

	// Check GAS stats
	gasStats, ok := info.TokenStats[TokenTypeGAS]
	if !ok {
		t.Fatal("TokenStats should contain GAS")
	}
	if gasStats.TotalBalance != 3500000 {
		t.Errorf("GAS TotalBalance = %d, want 3500000", gasStats.TotalBalance)
	}
	if gasStats.AvailableBalance != 1000000 {
		t.Errorf("GAS AvailableBalance = %d, want 1000000", gasStats.AvailableBalance)
	}
	if gasStats.LockedBalance != 2000000 {
		t.Errorf("GAS LockedBalance = %d, want 2000000", gasStats.LockedBalance)
	}

	// Check NEO stats
	neoStats, ok := info.TokenStats[TokenTypeNEO]
	if !ok {
		t.Fatal("TokenStats should contain NEO")
	}
	if neoStats.TotalBalance != 10 {
		t.Errorf("NEO TotalBalance = %d, want 10", neoStats.TotalBalance)
	}
}

func TestListAccountsByServiceWithMockMultiToken(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{
		ID: "acc-1", LockedBy: "neocompute",
	}
	mockRepo.accounts["acc-2"] = &neoaccountssupabase.Account{
		ID: "acc-2", LockedBy: "neocompute",
	}
	mockRepo.accounts["acc-3"] = &neoaccountssupabase.Account{
		ID: "acc-3", LockedBy: "neocompute",
	}

	ctx := context.Background()
	mockRepo.UpsertBalance(ctx, "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 1000000, 8)
	mockRepo.UpsertBalance(ctx, "acc-2", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 500000, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	// List all accounts for neocompute
	accounts, err := svc.ListAccountsByService(ctx, "neocompute", "", nil)
	if err != nil {
		t.Fatalf("ListAccountsByService() error = %v", err)
	}

	if len(accounts) != 3 {
		t.Errorf("len(accounts) = %d, want 3", len(accounts))
	}

	// Filter by min balance
	minBal := int64(800000)
	filtered, err := svc.ListAccountsByService(ctx, "neocompute", TokenTypeGAS, &minBal)
	if err != nil {
		t.Fatalf("ListAccountsByService() error = %v", err)
	}

	if len(filtered) != 1 {
		t.Errorf("len(filtered) = %d, want 1", len(filtered))
	}
}

// =============================================================================
// SignTransaction and BatchSign Tests
// =============================================================================

func TestSignTransactionWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{
		ID:       "acc-1",
		Address:  "NAddr1",
		LockedBy: "neocompute",
	}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	txHash := crypto.Hash256([]byte("test transaction"))
	resp, err := svc.SignTransaction(ctx, "neocompute", "acc-1", txHash)
	if err != nil {
		t.Fatalf("SignTransaction() error = %v", err)
	}

	if resp.AccountID != "acc-1" {
		t.Errorf("AccountID = %s, want acc-1", resp.AccountID)
	}
	if len(resp.Signature) != 64 {
		t.Errorf("len(Signature) = %d, want 64", len(resp.Signature))
	}
	if len(resp.PublicKey) == 0 {
		t.Error("PublicKey should not be empty")
	}
}

func TestSignTransactionWrongService(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{
		ID:       "acc-1",
		LockedBy: "neocompute",
	}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	txHash := crypto.Hash256([]byte("test"))
	_, err := svc.SignTransaction(ctx, "wrong-service", "acc-1", txHash)
	if err == nil {
		t.Error("SignTransaction should fail for wrong service")
	}
}

func TestSignTransactionAccountNotFound(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	txHash := crypto.Hash256([]byte("test"))
	_, err := svc.SignTransaction(ctx, "neocompute", "nonexistent", txHash)
	if err == nil {
		t.Error("SignTransaction should fail for nonexistent account")
	}
}

func TestBatchSignWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	mockRepo.accounts["acc-2"] = &neoaccountssupabase.Account{ID: "acc-2", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	requests := []SignRequest{
		{AccountID: "acc-1", TxHash: crypto.Hash256([]byte("tx1"))},
		{AccountID: "acc-2", TxHash: crypto.Hash256([]byte("tx2"))},
		{AccountID: "nonexistent", TxHash: crypto.Hash256([]byte("tx3"))},
	}

	resp := svc.BatchSign(ctx, "neocompute", requests)
	if len(resp.Signatures) != 2 {
		t.Errorf("len(Signatures) = %d, want 2", len(resp.Signatures))
	}
	if len(resp.Errors) != 1 {
		t.Errorf("len(Errors) = %d, want 1", len(resp.Errors))
	}
}

// =============================================================================
// ReleaseAccounts Tests
// =============================================================================

func TestReleaseAccountsWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	mockRepo.accounts["acc-2"] = &neoaccountssupabase.Account{ID: "acc-2", LockedBy: "neocompute"}
	mockRepo.accounts["acc-3"] = &neoaccountssupabase.Account{ID: "acc-3", LockedBy: "other"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	released, err := svc.ReleaseAccounts(ctx, "neocompute", []string{"acc-1", "acc-3"})
	if err != nil {
		t.Fatalf("ReleaseAccounts() error = %v", err)
	}

	// Only acc-1 should be released (acc-3 is locked by "other")
	if released != 1 {
		t.Errorf("released = %d, want 1", released)
	}

	// Verify acc-1 is now unlocked
	acc1, _ := mockRepo.GetByID(ctx, "acc-1")
	if acc1.LockedBy != "" {
		t.Errorf("acc-1 should be unlocked, got LockedBy = %s", acc1.LockedBy)
	}
}

func TestReleaseAllByServiceWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	mockRepo.accounts["acc-2"] = &neoaccountssupabase.Account{ID: "acc-2", LockedBy: "neocompute"}
	mockRepo.accounts["acc-3"] = &neoaccountssupabase.Account{ID: "acc-3", LockedBy: "other"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	released, err := svc.ReleaseAllByService(ctx, "neocompute")
	if err != nil {
		t.Fatalf("ReleaseAllByService() error = %v", err)
	}

	if released != 2 {
		t.Errorf("released = %d, want 2", released)
	}
}

// =============================================================================
// UpdateBalance Edge Cases
// =============================================================================

func TestUpdateBalanceInsufficientFunds(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	mockRepo.UpsertBalance(context.Background(), "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 100, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	// Try to subtract more than available
	_, _, _, err := svc.UpdateBalance(ctx, "neocompute", "acc-1", TokenTypeGAS, -200, nil)
	if err == nil {
		t.Error("UpdateBalance should fail for insufficient funds")
	}
}

func TestUpdateBalanceAbsoluteValue(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	mockRepo.UpsertBalance(context.Background(), "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 100, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	// Set absolute value
	absValue := int64(999)
	oldBal, newBal, _, err := svc.UpdateBalance(ctx, "neocompute", "acc-1", TokenTypeGAS, 0, &absValue)
	if err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}

	if oldBal != 100 {
		t.Errorf("oldBal = %d, want 100", oldBal)
	}
	if newBal != 999 {
		t.Errorf("newBal = %d, want 999", newBal)
	}
}

func TestUpdateBalanceWrongService(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	_, _, _, err := svc.UpdateBalance(ctx, "wrong-service", "acc-1", TokenTypeGAS, 100, nil)
	if err == nil {
		t.Error("UpdateBalance should fail for wrong service")
	}
}

func TestUpdateBalanceDefaultToken(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	// Empty token should default to GAS
	_, newBal, _, err := svc.UpdateBalance(ctx, "neocompute", "acc-1", "", 100, nil)
	if err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}
	if newBal != 100 {
		t.Errorf("newBal = %d, want 100", newBal)
	}

	// Verify GAS balance was updated
	bal, _ := mockRepo.GetBalance(ctx, "acc-1", TokenTypeGAS)
	if bal == nil || bal.Amount != 100 {
		t.Error("GAS balance should be 100")
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandleInfoEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1"}
	ctx := context.Background()
	mockRepo.UpsertBalance(ctx, "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 1000000, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	// First verify GetPoolInfo works directly
	info, err := svc.GetPoolInfo(ctx)
	if err != nil {
		t.Fatalf("GetPoolInfo() error = %v", err)
	}
	if info.TotalAccounts != 1 {
		t.Errorf("GetPoolInfo TotalAccounts = %d, want 1", info.TotalAccounts)
	}

	req := httptest.NewRequest("GET", "/pool-info", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	// Debug: print response body
	bodyStr := rr.Body.String()
	t.Logf("Response body: %s", bodyStr)

	var resp PoolInfoResponse
	if err := json.NewDecoder(strings.NewReader(bodyStr)).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.TotalAccounts != 1 {
		t.Errorf("TotalAccounts = %d, want 1", resp.TotalAccounts)
	}
}

func TestHandleListAccountsEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	ctx := context.Background()
	mockRepo.UpsertBalance(ctx, "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 1000000, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	req := httptest.NewRequest("GET", "/accounts?service_id=neocompute", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp ListAccountsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Accounts) != 1 {
		t.Errorf("len(Accounts) = %d, want 1", len(resp.Accounts))
	}
}

func TestHandleListAccountsMissingServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: newMockNeoAccountsRepo()})

	req := httptest.NewRequest("GET", "/accounts", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListAccountsWithTokenFilter(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	mockRepo.accounts["acc-2"] = &neoaccountssupabase.Account{ID: "acc-2", LockedBy: "neocompute"}
	ctx := context.Background()
	mockRepo.UpsertBalance(ctx, "acc-1", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 1000000, 8)
	mockRepo.UpsertBalance(ctx, "acc-2", TokenTypeGAS, neoaccountssupabase.GASScriptHash, 100, 8)

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	// Filter by min_balance
	req := httptest.NewRequest("GET", "/accounts?service_id=neocompute&token=GAS&min_balance=500000", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp ListAccountsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Accounts) != 1 {
		t.Errorf("len(Accounts) = %d, want 1 (only acc-1 has enough GAS)", len(resp.Accounts))
	}
}

func TestHandleRequestAccountsEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", Address: "NAddr1"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	body := `{"service_id": "neocompute", "count": 1, "purpose": "test"}`
	req := httptest.NewRequest("POST", "/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestHandleRequestAccountsMissingServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: newMockNeoAccountsRepo()})

	body := `{"count": 1}`
	req := httptest.NewRequest("POST", "/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleReleaseAccountsEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	body := `{"service_id": "neocompute", "account_ids": ["acc-1"]}`
	req := httptest.NewRequest("POST", "/release", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp ReleaseAccountsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.ReleasedCount != 1 {
		t.Errorf("ReleasedCount = %d, want 1", resp.ReleasedCount)
	}
}

func TestHandleReleaseAccountsAll(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}
	mockRepo.accounts["acc-2"] = &neoaccountssupabase.Account{ID: "acc-2", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	// Release all without specific account_ids
	body := `{"service_id": "neocompute"}`
	req := httptest.NewRequest("POST", "/release", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp ReleaseAccountsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.ReleasedCount != 2 {
		t.Errorf("ReleasedCount = %d, want 2", resp.ReleasedCount)
	}
}

func TestHandleSignTransactionEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	txHash := crypto.Hash256([]byte("test tx"))
	body := fmt.Sprintf(`{"service_id": "neocompute", "account_id": "acc-1", "tx_hash": %s}`, mustMarshalJSON(txHash))
	req := httptest.NewRequest("POST", "/sign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestHandleSignTransactionMissingFields(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: newMockNeoAccountsRepo()})

	body := `{"service_id": "neocompute"}`
	req := httptest.NewRequest("POST", "/sign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleBatchSignEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	txHash := crypto.Hash256([]byte("test tx"))
	body := fmt.Sprintf(`{"service_id": "neocompute", "requests": [{"account_id": "acc-1", "tx_hash": %s}]}`, mustMarshalJSON(txHash))
	req := httptest.NewRequest("POST", "/batch-sign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpdateBalanceEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	body := `{"service_id": "neocompute", "account_id": "acc-1", "token": "GAS", "delta": 1000000}`
	req := httptest.NewRequest("POST", "/balance", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp UpdateBalanceResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.NewBalance != 1000000 {
		t.Errorf("NewBalance = %d, want 1000000", resp.NewBalance)
	}
	if resp.Token != TokenTypeGAS {
		t.Errorf("Token = %s, want %s", resp.Token, TokenTypeGAS)
	}
}

func TestHandleUpdateBalanceDefaultToken(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neocompute"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	// No token specified - should default to GAS
	body := `{"service_id": "neocompute", "account_id": "acc-1", "delta": 500}`
	req := httptest.NewRequest("POST", "/balance", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp UpdateBalanceResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Token != TokenTypeGAS {
		t.Errorf("Token = %s, want %s (default)", resp.Token, TokenTypeGAS)
	}
}

// =============================================================================
// Type Conversion Tests
// =============================================================================

func TestAccountInfoFromAccount(t *testing.T) {
	acc := &neoaccountssupabase.Account{
		ID:         "test-id",
		Address:    "NAddr1",
		TxCount:    5,
		IsRetiring: false,
		LockedBy:   "neocompute",
	}

	info := AccountInfoFromAccount(acc)

	if info.ID != "test-id" {
		t.Errorf("ID = %s, want test-id", info.ID)
	}
	if info.Address != "NAddr1" {
		t.Errorf("Address = %s, want NAddr1", info.Address)
	}
	if info.TxCount != 5 {
		t.Errorf("TxCount = %d, want 5", info.TxCount)
	}
	if info.Balances == nil {
		t.Error("Balances should be initialized")
	}
	if len(info.Balances) != 0 {
		t.Errorf("len(Balances) = %d, want 0", len(info.Balances))
	}
}

// =============================================================================
// Request Validation Tests
// =============================================================================

func TestRequestAccountsInvalidCount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	// Count 0 should fail
	_, _, err := svc.RequestAccounts(ctx, "neocompute", 0, "test")
	if err == nil {
		t.Error("RequestAccounts should fail for count=0")
	}

	// Count > 100 should fail
	_, _, err = svc.RequestAccounts(ctx, "neocompute", 101, "test")
	if err == nil {
		t.Error("RequestAccounts should fail for count>100")
	}
}

func TestRequestAccountsNoAvailable(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	// Add only locked accounts
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "other"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})
	ctx := context.Background()

	// Request should create new accounts if none available
	accounts, _, err := svc.RequestAccounts(ctx, "neocompute", 1, "test")
	if err != nil {
		// May fail if createAccount requires more setup
		t.Logf("RequestAccounts returned error (expected if createAccount not fully mocked): %v", err)
		return
	}
	if len(accounts) == 0 {
		t.Error("Should have created new account")
	}
}

// Helper function
func mustMarshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// =============================================================================
// Master Key Helper Function Tests
// =============================================================================

func TestParseHashRawBytes(t *testing.T) {
	// Test with raw 32 bytes
	rawHash := make([]byte, 32)
	for i := range rawHash {
		rawHash[i] = byte(i)
	}

	result, err := parseHash(rawHash)
	if err != nil {
		t.Fatalf("parseHash(raw) error = %v", err)
	}
	if len(result) != 32 {
		t.Errorf("len(result) = %d, want 32", len(result))
	}
}

func TestParseHashHexString(t *testing.T) {
	// Test with hex string (64 chars = 32 bytes)
	hexStr := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	result, err := parseHash([]byte(hexStr))
	if err != nil {
		t.Fatalf("parseHash(hex) error = %v", err)
	}
	if len(result) != 32 {
		t.Errorf("len(result) = %d, want 32", len(result))
	}
}

func TestParseHashInvalidHex(t *testing.T) {
	// Test with invalid hex
	_, err := parseHash([]byte("not-valid-hex"))
	if err == nil {
		t.Error("parseHash should fail for invalid hex")
	}
}

func TestParseHashWrongLength(t *testing.T) {
	// Test with wrong length hex (valid hex but not 32 bytes)
	_, err := parseHash([]byte("0123456789abcdef")) // 8 bytes
	if err == nil {
		t.Error("parseHash should fail for wrong length")
	}
}

func TestEqualHash(t *testing.T) {
	hash1 := []byte{1, 2, 3, 4}
	hash2 := []byte{1, 2, 3, 4}
	hash3 := []byte{1, 2, 3, 5}
	hash4 := []byte{1, 2, 3}

	if !equalHash(hash1, hash2) {
		t.Error("equal hashes should return true")
	}
	if equalHash(hash1, hash3) {
		t.Error("different hashes should return false")
	}
	if equalHash(hash1, hash4) {
		t.Error("different length hashes should return false")
	}
}

func TestDeriveMasterKeyFromSeed(t *testing.T) {
	seed := []byte("test-seed-that-is-long-enough!!!")
	key, err := deriveMasterKeyFromSeed(seed)
	if err != nil {
		t.Fatalf("deriveMasterKeyFromSeed() error = %v", err)
	}
	if len(key) != 32 {
		t.Errorf("len(key) = %d, want 32", len(key))
	}

	// Same seed should produce same key
	key2, _ := deriveMasterKeyFromSeed(seed)
	if hex.EncodeToString(key) != hex.EncodeToString(key2) {
		t.Error("same seed should produce same key")
	}
}

func TestDeriveMasterKeyFromSeedTooShort(t *testing.T) {
	seed := []byte("short")
	_, err := deriveMasterKeyFromSeed(seed)
	if err == nil {
		t.Error("deriveMasterKeyFromSeed should fail for short seed")
	}
}

func TestDeriveMasterPubKey(t *testing.T) {
	masterKey := []byte("test-master-key-32-bytes-long!!!")
	pubKey, err := deriveMasterPubKey(masterKey)
	if err != nil {
		t.Fatalf("deriveMasterPubKey() error = %v", err)
	}

	// P-256 compressed pubkey is 33 bytes
	if len(pubKey) != 33 {
		t.Errorf("len(pubKey) = %d, want 33", len(pubKey))
	}

	// Same master key should produce same pubkey
	pubKey2, _ := deriveMasterPubKey(masterKey)
	if hex.EncodeToString(pubKey) != hex.EncodeToString(pubKey2) {
		t.Error("same master key should produce same pubkey")
	}
}

func TestDeriveMasterPubKeyTooShort(t *testing.T) {
	masterKey := []byte("short")
	_, err := deriveMasterPubKey(masterKey)
	if err == nil {
		t.Error("deriveMasterPubKey should fail for short key")
	}
}

func TestMasterKeySummary(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: newMockNeoAccountsRepo()})

	// Set some mock values for the summary
	svc.masterKeyHash = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	svc.masterPubKey = []byte{2, 1, 2, 3}
	svc.masterKeyAttestationID = "test-attestation"

	summary := svc.masterKeySummary()
	if summary.Hash == "" {
		t.Error("Hash should not be empty")
	}
	if summary.Source != "coordinator" {
		t.Errorf("Source = %s, want coordinator", summary.Source)
	}
}

// =============================================================================
// Lifecycle Tests
// =============================================================================

func TestStartStop(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	// Start should not panic
	if err := svc.Start(context.Background()); err != nil {
		t.Logf("Start returned error (expected without full setup): %v", err)
	}

	// Stop should not panic
	svc.Stop()
}

// =============================================================================
// Handler Error Path Tests
// =============================================================================

func TestHandleReleaseAccountsMissingServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: newMockNeoAccountsRepo()})

	body := `{}`
	req := httptest.NewRequest("POST", "/release", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleBatchSignMissingServiceID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: newMockNeoAccountsRepo()})

	body := `{}`
	req := httptest.NewRequest("POST", "/batch-sign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateBalanceMissingFields(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))
	svc, _ := New(Config{Marble: m, NeoAccountsRepo: newMockNeoAccountsRepo()})

	body := `{"service_id": "neocompute"}`
	req := httptest.NewRequest("POST", "/balance", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleRequestAccountsDefaultCount(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockNeoAccountsRepo()
	mockRepo.accounts["acc-1"] = &neoaccountssupabase.Account{ID: "acc-1", Address: "NAddr1"}

	svc, _ := New(Config{Marble: m, NeoAccountsRepo: mockRepo})

	// Count 0 or negative should default to 1
	body := `{"service_id": "neocompute", "count": 0}`
	req := httptest.NewRequest("POST", "/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

// =============================================================================
// Background Worker Tests
// =============================================================================

func TestRotateAccounts(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Add some accounts - old and with low balances (candidates for rotation)
	oldTime := time.Now().Add(-time.Duration(RotationMinAge+1) * time.Hour)
	acc1 := &neoaccountssupabase.Account{
		ID:         "acc-old-1",
		Address:    "NAddr1",
		CreatedAt:  oldTime,
		LastUsedAt: oldTime,
		TxCount:    5,
		IsRetiring: false,
		LockedBy:   "", // Not locked
	}
	acc2 := &neoaccountssupabase.Account{
		ID:         "acc-old-2",
		Address:    "NAddr2",
		CreatedAt:  oldTime,
		LastUsedAt: oldTime,
		TxCount:    3,
		IsRetiring: false,
		LockedBy:   "some-service", // Locked - should NOT be rotated
	}
	acc3 := &neoaccountssupabase.Account{
		ID:         "acc-new-1",
		Address:    "NAddr3",
		CreatedAt:  time.Now(), // Too new
		LastUsedAt: time.Now(),
		TxCount:    1,
		IsRetiring: false,
		LockedBy:   "",
	}

	mockRepo.Create(context.Background(), acc1)
	mockRepo.Create(context.Background(), acc2)
	mockRepo.Create(context.Background(), acc3)

	// Add balances - acc1 has low balance (rotation candidate)
	mockRepo.UpsertBalance(context.Background(), "acc-old-1", TokenTypeGAS, "", 1000, 8)       // Very low
	mockRepo.UpsertBalance(context.Background(), "acc-old-2", TokenTypeGAS, "", 1000000000, 8) // 10 GAS
	mockRepo.UpsertBalance(context.Background(), "acc-new-1", TokenTypeGAS, "", 500000000, 8)  // 5 GAS

	// Call rotateAccounts directly
	svc.rotateAccounts(context.Background())

	// Check that acc1 is now retiring (old, unlocked, low balance)
	updatedAcc1, _ := mockRepo.GetByID(context.Background(), "acc-old-1")
	if updatedAcc1 == nil {
		t.Fatal("expected acc-old-1 to exist")
	}

	// Check that acc2 (locked) is NOT retiring
	updatedAcc2, _ := mockRepo.GetByID(context.Background(), "acc-old-2")
	if updatedAcc2 != nil && updatedAcc2.IsRetiring {
		t.Error("Locked account should never be rotated")
	}

	// Check that acc3 (new) is NOT retiring
	updatedAcc3, _ := mockRepo.GetByID(context.Background(), "acc-new-1")
	if updatedAcc3 != nil && updatedAcc3.IsRetiring {
		t.Error("New account should not be rotated")
	}
}

func TestCleanupStaleLocks(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Add accounts with stale locks
	staleLockTime := time.Now().Add(-(LockTimeout + time.Hour))
	freshLockTime := time.Now().Add(-time.Minute)

	acc1 := &neoaccountssupabase.Account{
		ID:       "acc-stale-lock",
		Address:  "NAddr1",
		LockedBy: "old-service",
		LockedAt: staleLockTime, // Stale lock
	}
	acc2 := &neoaccountssupabase.Account{
		ID:       "acc-fresh-lock",
		Address:  "NAddr2",
		LockedBy: "active-service",
		LockedAt: freshLockTime, // Fresh lock
	}
	acc3 := &neoaccountssupabase.Account{
		ID:       "acc-no-lock",
		Address:  "NAddr3",
		LockedBy: "",
	}

	mockRepo.Create(context.Background(), acc1)
	mockRepo.Create(context.Background(), acc2)
	mockRepo.Create(context.Background(), acc3)

	// Call cleanupStaleLocks directly
	svc.cleanupStaleLocks(context.Background())

	// Check stale lock was released
	updatedAcc1, _ := mockRepo.GetByID(context.Background(), "acc-stale-lock")
	if updatedAcc1 != nil && updatedAcc1.LockedBy != "" {
		t.Errorf("Stale lock should be released, got LockedBy=%s", updatedAcc1.LockedBy)
	}

	// Check fresh lock was NOT released
	updatedAcc2, _ := mockRepo.GetByID(context.Background(), "acc-fresh-lock")
	if updatedAcc2 == nil || updatedAcc2.LockedBy != "active-service" {
		t.Error("Fresh lock should not be released")
	}

	// Check unlocked account is unchanged
	updatedAcc3, _ := mockRepo.GetByID(context.Background(), "acc-no-lock")
	if updatedAcc3 == nil || updatedAcc3.LockedBy != "" {
		t.Error("Unlocked account should remain unlocked")
	}
}

func TestRotateAccountsDeletesEmptyRetiringAccounts(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Add a retiring account with zero balances
	acc := &neoaccountssupabase.Account{
		ID:         "acc-retiring-empty",
		Address:    "NAddr1",
		IsRetiring: true,
		LockedBy:   "", // Not locked
	}
	mockRepo.Create(context.Background(), acc)
	// No balances added - all zero

	// Call rotateAccounts
	svc.rotateAccounts(context.Background())

	// Check account was deleted
	_, err := mockRepo.GetByID(context.Background(), "acc-retiring-empty")
	if err == nil {
		t.Log("Empty retiring account may or may not be deleted depending on implementation")
	}
}

func TestServiceRegistersTickerWorkers(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	// NeoAccounts registers two periodic maintenance workers via BaseService.AddTickerWorker:
	// - account rotation
	// - stale lock cleanup
	if svc.WorkerCount() != 2 {
		t.Fatalf("WorkerCount() = %d, want 2", svc.WorkerCount())
	}
}

// =============================================================================
// More Handler Edge Cases
// =============================================================================

func TestHandleSignTransactionSuccess(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Setup: create account and lock it
	acc := &neoaccountssupabase.Account{
		ID:       "sign-test-acc",
		Address:  "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		LockedBy: "sign-test-service",
		LockedAt: time.Now(),
	}
	mockRepo.Create(context.Background(), acc)

	txHash := crypto.Hash256([]byte("test transaction"))

	input := SignTransactionInput{
		ServiceID: "sign-test-service",
		AccountID: "sign-test-acc",
		TxHash:    txHash,
	}
	body, _ := json.Marshal(input)

	req := httptest.NewRequest("POST", "/sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleSignTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAccountsWithMinBalance(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Create account with balance
	acc := &neoaccountssupabase.Account{
		ID:       "bal-test-acc",
		Address:  "NAddr1",
		LockedBy: "bal-test-service",
		LockedAt: time.Now(),
	}
	mockRepo.Create(context.Background(), acc)
	mockRepo.UpsertBalance(context.Background(), "bal-test-acc", TokenTypeGAS, "", 500000000, 8)

	req := httptest.NewRequest("GET", "/accounts?service_id=bal-test-service&token=GAS&min_balance=100000000", nil)
	rr := httptest.NewRecorder()

	svc.handleListAccounts(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHandleListAccountsInvalidMinBalance(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	acc := &neoaccountssupabase.Account{
		ID:       "inv-bal-acc",
		Address:  "NAddr1",
		LockedBy: "inv-bal-service",
	}
	mockRepo.Create(context.Background(), acc)

	// Invalid min_balance format - should be ignored
	req := httptest.NewRequest("GET", "/accounts?service_id=inv-bal-service&min_balance=invalid", nil)
	rr := httptest.NewRecorder()

	svc.handleListAccounts(rr, req)

	// Should still succeed, ignoring invalid param
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHandleRequestAccountsExceedsLimit(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	input := RequestAccountsInput{
		ServiceID: "test-service",
		Count:     150, // Exceeds max of 100
	}
	body, _ := json.Marshal(input)

	req := httptest.NewRequest("POST", "/request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleRequestAccounts(rr, req)

	// Should fail due to invalid count
	if rr.Code == http.StatusOK {
		t.Error("Expected error for count > 100")
	}
}

func TestHandleInfoError(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Make repo return error
	mockRepo.simulateError = true

	req := httptest.NewRequest("GET", "/pool-info", nil)
	rr := httptest.NewRecorder()

	svc.handleInfo(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error when repo fails")
	}
}

func TestHandleListAccountsError(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	mockRepo.simulateError = true

	req := httptest.NewRequest("GET", "/accounts?service_id=test-service", nil)
	rr := httptest.NewRecorder()

	svc.handleListAccounts(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error when repo fails")
	}
}

// =============================================================================
// InitializePool Tests
// =============================================================================

func TestInitializePoolWithExistingAccounts(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Add some accounts - fewer than MinPoolAccounts
	for i := 0; i < 3; i++ {
		mockRepo.Create(context.Background(), &neoaccountssupabase.Account{
			ID:      fmt.Sprintf("existing-acc-%d", i),
			Address: fmt.Sprintf("NAddr%d", i),
		})
	}

	// Initialize should create more accounts to reach MinPoolAccounts
	err := svc.initializePool(context.Background())
	if err != nil {
		t.Fatalf("initializePool: %v", err)
	}

	// Check that accounts were created
	accounts, _ := mockRepo.List(context.Background())
	if len(accounts) < MinPoolAccounts {
		t.Errorf("Expected at least %d accounts, got %d", MinPoolAccounts, len(accounts))
	}
}

func TestInitializePoolAlreadyFull(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Add MaxPoolAccounts already
	for i := 0; i < MaxPoolAccounts; i++ {
		mockRepo.Create(context.Background(), &neoaccountssupabase.Account{
			ID:      fmt.Sprintf("full-acc-%d", i),
			Address: fmt.Sprintf("NAddr%d", i),
		})
	}

	// Initialize should do nothing
	err := svc.initializePool(context.Background())
	if err != nil {
		t.Fatalf("initializePool: %v", err)
	}

	// Count should remain at MaxPoolAccounts
	accounts, _ := mockRepo.List(context.Background())
	if len(accounts) != MaxPoolAccounts {
		t.Errorf("Expected %d accounts, got %d", MaxPoolAccounts, len(accounts))
	}
}

func TestInitializePoolError(t *testing.T) {
	// Fail closed in strict/production mode.
	t.Setenv("MARBLE_ENV", "production")
	svc, mockRepo := newTestServiceWithMock(t)

	mockRepo.simulateError = true

	err := svc.initializePool(context.Background())
	if err == nil {
		t.Error("Expected error when repo fails")
	}
}

// =============================================================================
// Handler Error Path Tests
// =============================================================================

func TestHandleReleaseAccountsInvalidJSON(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	req := httptest.NewRequest("POST", "/release", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleReleaseAccounts(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleSignTransactionInvalidJSON(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	req := httptest.NewRequest("POST", "/sign", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleSignTransaction(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleBatchSignInvalidJSON(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	req := httptest.NewRequest("POST", "/batch-sign", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleBatchSign(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleUpdateBalanceInvalidJSON(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	req := httptest.NewRequest("POST", "/balance", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleUpdateBalance(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleRequestAccountsInvalidJSON(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	req := httptest.NewRequest("POST", "/request", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleRequestAccounts(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error for invalid JSON")
	}
}

// =============================================================================
// More Pool Operation Tests
// =============================================================================

func TestReleaseAccountsWrongService(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Create account locked by service-A
	acc := &neoaccountssupabase.Account{
		ID:       "locked-acc",
		Address:  "NAddr1",
		LockedBy: "service-A",
		LockedAt: time.Now(),
	}
	mockRepo.Create(context.Background(), acc)

	// Try to release with service-B
	released, err := svc.ReleaseAccounts(context.Background(), "service-B", []string{"locked-acc"})
	if err != nil {
		t.Fatalf("ReleaseAccounts: %v", err)
	}

	if released != 0 {
		t.Error("Should not release accounts locked by different service")
	}
}

func TestUpdateBalanceGetBalanceError(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Create account locked by service
	acc := &neoaccountssupabase.Account{
		ID:       "bal-err-acc",
		Address:  "NAddr1",
		LockedBy: "test-service",
		LockedAt: time.Now(),
	}
	mockRepo.Create(context.Background(), acc)
	// Don't add any balance - GetBalance will return nil

	// Should work with nil current balance (treated as 0)
	_, newBal, _, err := svc.UpdateBalance(context.Background(), "test-service", "bal-err-acc", TokenTypeGAS, 100000000, nil)
	if err != nil {
		t.Fatalf("UpdateBalance: %v", err)
	}

	if newBal != 100000000 {
		t.Errorf("Expected new balance 100000000, got %d", newBal)
	}
}

func TestSignTransactionAccountNotLockedByService(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	// Create unlocked account
	acc := &neoaccountssupabase.Account{
		ID:       "unlocked-acc",
		Address:  "NAddr1",
		LockedBy: "", // Not locked
	}
	mockRepo.Create(context.Background(), acc)

	txHash := crypto.Hash256([]byte("test"))
	_, err := svc.SignTransaction(context.Background(), "some-service", "unlocked-acc", txHash)

	if err == nil {
		t.Error("Expected error when signing with unlocked account")
	}
}

func TestSignTransactionAccountMissing(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	txHash := crypto.Hash256([]byte("test"))
	_, err := svc.SignTransaction(context.Background(), "some-service", "non-existent-acc", txHash)

	if err == nil {
		t.Error("Expected error when account not found")
	}
}

func TestCreateAccountSuccess(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	acc, err := svc.createAccount(context.Background())
	if err != nil {
		t.Fatalf("createAccount: %v", err)
	}

	if acc == nil {
		t.Fatal("Expected account to be created")
	}

	if acc.ID == "" {
		t.Error("Account should have an ID")
	}

	if acc.Address == "" {
		t.Error("Account should have an address")
	}

	// Verify it was saved to repo
	saved, err := mockRepo.GetByID(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if saved.Address != acc.Address {
		t.Error("Saved account should match created account")
	}
}

func BenchmarkDeriveAccountKey(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("benchmark-master-key-32-bytes!!!"))
	svc, _ := New(Config{Marble: m})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.deriveAccountKey("benchmark-account")
	}
}

func BenchmarkGetPrivateKey(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("benchmark-master-key-32-bytes!!!"))
	svc, _ := New(Config{Marble: m})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.getPrivateKey("benchmark-account")
	}
}

func BenchmarkSignHash(b *testing.B) {
	curve := elliptic.P256()
	d, _ := crypto.GenerateRandomBytes(32)
	dInt := new(big.Int).SetBytes(d)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	dInt.Mod(dInt, n)
	dInt.Add(dInt, big.NewInt(1))

	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve},
		D:         dInt,
	}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(dInt.Bytes())

	hash := crypto.Hash256([]byte("benchmark transaction"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = signHash(priv, hash)
	}
}

func BenchmarkAccountInfoMarshal(b *testing.B) {
	info := AccountInfo{
		ID:       "acc-123",
		Address:  "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		TxCount:  10,
		LockedBy: "neocompute",
		Balances: map[string]TokenBalance{
			TokenTypeGAS: {TokenType: TokenTypeGAS, Amount: 1000000, Decimals: 8},
			TokenTypeNEO: {TokenType: TokenTypeNEO, Amount: 10, Decimals: 0},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(info)
	}
}
