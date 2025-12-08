// Package accountpool provides unit tests for the account pool service.
package accountpoolmarble

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/marble"
	accountpoolsupabase "github.com/R3E-Network/service_layer/services/accountpool/supabase"
)

// =============================================================================
// Mock Repository for Tests
// =============================================================================

// mockAccountPoolRepo implements accountpoolsupabase.RepositoryInterface for testing.
type mockAccountPoolRepo struct {
	accounts map[string]*accountpoolsupabase.Account
}

func newMockAccountPoolRepo() *mockAccountPoolRepo {
	return &mockAccountPoolRepo{
		accounts: make(map[string]*accountpoolsupabase.Account),
	}
}

func (m *mockAccountPoolRepo) Create(_ context.Context, acc *accountpoolsupabase.Account) error {
	m.accounts[acc.ID] = acc
	return nil
}

func (m *mockAccountPoolRepo) Update(_ context.Context, acc *accountpoolsupabase.Account) error {
	m.accounts[acc.ID] = acc
	return nil
}

func (m *mockAccountPoolRepo) GetByID(_ context.Context, id string) (*accountpoolsupabase.Account, error) {
	if acc, ok := m.accounts[id]; ok {
		return acc, nil
	}
	return nil, fmt.Errorf("account not found: %s", id)
}

func (m *mockAccountPoolRepo) List(_ context.Context) ([]accountpoolsupabase.Account, error) {
	var result []accountpoolsupabase.Account
	for _, acc := range m.accounts {
		result = append(result, *acc)
	}
	return result, nil
}

func (m *mockAccountPoolRepo) ListAvailable(_ context.Context, limit int) ([]accountpoolsupabase.Account, error) {
	var result []accountpoolsupabase.Account
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

func (m *mockAccountPoolRepo) ListByLocker(_ context.Context, lockerID string) ([]accountpoolsupabase.Account, error) {
	var result []accountpoolsupabase.Account
	for _, acc := range m.accounts {
		if acc.LockedBy == lockerID {
			result = append(result, *acc)
		}
	}
	return result, nil
}

func (m *mockAccountPoolRepo) Delete(_ context.Context, id string) error {
	delete(m.accounts, id)
	return nil
}

func TestDeriveAccountKeyDeterministic(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
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
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, _ := New(Config{Marble: m})

	key1, _ := svc.deriveAccountKey("account-1")
	key2, _ := svc.deriveAccountKey("account-2")

	if hex.EncodeToString(key1) == hex.EncodeToString(key2) {
		t.Error("different account IDs should produce different keys")
	}
}

func TestGetPrivateKeyValid(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
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
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, _ := New(Config{Marble: m})

	priv1, _ := svc.getPrivateKey("account-x")
	priv2, _ := svc.getPrivateKey("account-x")

	if priv1.D.Cmp(priv2.D) != 0 {
		t.Error("same account should produce same private key")
	}
}

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

func TestAccountInfoTypes(t *testing.T) {
	info := AccountInfo{
		ID:       "test-id",
		Address:  "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		Balance:  1000,
		TxCount:  5,
		LockedBy: "mixer",
	}

	if info.ID != "test-id" {
		t.Errorf("ID mismatch: got %s", info.ID)
	}
	if info.Balance != 1000 {
		t.Errorf("Balance mismatch: got %d", info.Balance)
	}
	if info.LockedBy != "mixer" {
		t.Errorf("LockedBy mismatch: got %s", info.LockedBy)
	}
}

func TestRequestAccountsInputValidation(t *testing.T) {
	input := RequestAccountsInput{
		ServiceID: "mixer",
		Count:     5,
		Purpose:   "mixing operation",
	}

	if input.ServiceID == "" {
		t.Error("ServiceID should not be empty")
	}
	if input.Count <= 0 {
		t.Error("Count should be positive")
	}
}

func TestBatchSignInputTypes(t *testing.T) {
	input := BatchSignInput{
		ServiceID: "mixer",
		Requests: []SignRequest{
			{AccountID: "acc-1", TxHash: []byte("hash1")},
			{AccountID: "acc-2", TxHash: []byte("hash2")},
		},
	}

	if len(input.Requests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(input.Requests))
	}
	if input.Requests[0].AccountID != "acc-1" {
		t.Errorf("first request account ID mismatch")
	}
}

func TestPoolInfoResponseTypes(t *testing.T) {
	info := PoolInfoResponse{
		TotalAccounts:    100,
		ActiveAccounts:   80,
		LockedAccounts:   15,
		RetiringAccounts: 5,
		TotalBalance:     1000000,
	}

	if info.TotalAccounts != info.ActiveAccounts+info.LockedAccounts+info.RetiringAccounts {
		t.Error("account counts should sum to total")
	}
}

func BenchmarkDeriveAccountKey(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("benchmark-master-key-32-bytes!!!"))
	svc, _ := New(Config{Marble: m})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.deriveAccountKey("benchmark-account")
	}
}

func BenchmarkGetPrivateKey(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
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

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
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
	if ServiceID != "accountpool" {
		t.Errorf("ServiceID = %s, want accountpool", ServiceID)
	}
	if ServiceName != "Account Pool Service" {
		t.Errorf("ServiceName = %s, want Account Pool Service", ServiceName)
	}
	if Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", Version)
	}
}

// =============================================================================
// JSON Serialization Tests
// =============================================================================

func TestAccountInfoJSON(t *testing.T) {
	info := AccountInfo{
		ID:       "acc-123",
		Address:  "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		Balance:  1000000,
		TxCount:  10,
		LockedBy: "mixer",
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
	if decoded.Balance != info.Balance {
		t.Errorf("Balance = %d, want %d", decoded.Balance, info.Balance)
	}
}

func TestRequestAccountsInputJSON(t *testing.T) {
	input := RequestAccountsInput{
		ServiceID: "mixer",
		Count:     5,
		Purpose:   "mixing operation",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded RequestAccountsInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ServiceID != input.ServiceID {
		t.Errorf("ServiceID = %s, want %s", decoded.ServiceID, input.ServiceID)
	}
	if decoded.Count != input.Count {
		t.Errorf("Count = %d, want %d", decoded.Count, input.Count)
	}
}

func TestReleaseAccountsInputJSON(t *testing.T) {
	input := ReleaseAccountsInput{
		ServiceID:  "mixer",
		AccountIDs: []string{"acc-1", "acc-2", "acc-3"},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ReleaseAccountsInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ServiceID != input.ServiceID {
		t.Errorf("ServiceID = %s, want %s", decoded.ServiceID, input.ServiceID)
	}
	if len(decoded.AccountIDs) != len(input.AccountIDs) {
		t.Errorf("len(AccountIDs) = %d, want %d", len(decoded.AccountIDs), len(input.AccountIDs))
	}
}

func TestSignRequestJSON(t *testing.T) {
	req := SignRequest{
		AccountID: "acc-123",
		TxHash:    []byte("transaction-hash-bytes"),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SignRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.AccountID != req.AccountID {
		t.Errorf("AccountID = %s, want %s", decoded.AccountID, req.AccountID)
	}
}

func TestSignTransactionResponseJSON(t *testing.T) {
	resp := SignTransactionResponse{
		AccountID: "acc-123",
		Signature: []byte("signature-bytes"),
		PublicKey: []byte("public-key-bytes"),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SignTransactionResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.AccountID != resp.AccountID {
		t.Errorf("AccountID = %s, want %s", decoded.AccountID, resp.AccountID)
	}
}

func TestPoolInfoResponseJSON(t *testing.T) {
	info := PoolInfoResponse{
		TotalAccounts:    100,
		ActiveAccounts:   80,
		LockedAccounts:   15,
		RetiringAccounts: 5,
		TotalBalance:     1000000,
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
	if decoded.TotalBalance != info.TotalBalance {
		t.Errorf("TotalBalance = %d, want %d", decoded.TotalBalance, info.TotalBalance)
	}
}

func TestUpdateBalanceInputJSON(t *testing.T) {
	input := UpdateBalanceInput{
		ServiceID: "mixer",
		AccountID: "acc-123",
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
	if decoded.Delta != input.Delta {
		t.Errorf("Delta = %d, want %d", decoded.Delta, input.Delta)
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandleHealthEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
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
// Additional Type Tests
// =============================================================================

func TestBatchSignInputJSON(t *testing.T) {
	input := BatchSignInput{
		ServiceID: "mixer",
		Requests: []SignRequest{
			{AccountID: "acc-1", TxHash: []byte("hash1")},
			{AccountID: "acc-2", TxHash: []byte("hash2")},
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded BatchSignInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ServiceID != input.ServiceID {
		t.Errorf("ServiceID = %s, want %s", decoded.ServiceID, input.ServiceID)
	}
	if len(decoded.Requests) != len(input.Requests) {
		t.Errorf("len(Requests) = %d, want %d", len(decoded.Requests), len(input.Requests))
	}
}

func TestRequestAccountsResponseJSON(t *testing.T) {
	resp := RequestAccountsResponse{
		Accounts: []AccountInfo{
			{ID: "acc-1", Address: "NAddr1", Balance: 1000},
			{ID: "acc-2", Address: "NAddr2", Balance: 2000},
		},
		LockID: "lock-123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded RequestAccountsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(decoded.Accounts) != len(resp.Accounts) {
		t.Errorf("len(Accounts) = %d, want %d", len(decoded.Accounts), len(resp.Accounts))
	}
	if decoded.LockID != resp.LockID {
		t.Errorf("LockID = %s, want %s", decoded.LockID, resp.LockID)
	}
}

// =============================================================================
// Additional Benchmarks
// =============================================================================

func BenchmarkAccountInfoMarshal(b *testing.B) {
	info := AccountInfo{
		ID:       "acc-123",
		Address:  "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		Balance:  1000000,
		TxCount:  10,
		LockedBy: "mixer",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(info)
	}
}

func BenchmarkVerifySignature(b *testing.B) {
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
	sig, _ := signHash(priv, hash)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = verifySignature(&priv.PublicKey, hash, sig)
	}
}

// =============================================================================
// MockRepository Integration Tests
// =============================================================================

func TestRequestAccountsWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	// Pre-populate with pool accounts
	for i := 0; i < 5; i++ {
		mockRepo.accounts["acc-"+string(rune('a'+i))] = &accountpoolsupabase.Account{
			ID:         "acc-" + string(rune('a'+i)),
			Address:    "NAddr" + string(rune('1'+i)),
			Balance:    1000000,
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
			IsRetiring: false,
			LockedBy:   "",
		}
	}

	svc, err := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	accounts, lockID, err := svc.RequestAccounts(ctx, "mixer", 3, "mixing operation")
	if err != nil {
		t.Fatalf("RequestAccounts() error = %v", err)
	}

	if len(accounts) != 3 {
		t.Errorf("len(accounts) = %d, want 3", len(accounts))
	}
	if lockID == "" {
		t.Error("lockID should not be empty")
	}

	// Verify accounts are locked
	for _, acc := range accounts {
		if acc.LockedBy != "mixer" {
			t.Errorf("account %s should be locked by mixer", acc.ID)
		}
	}
}

func TestReleaseAccountsWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	// Create locked accounts
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID:       "acc-1",
		Address:  "NAddr1",
		LockedBy: "mixer",
		LockedAt: time.Now(),
	}
	mockRepo.accounts["acc-2"] = &accountpoolsupabase.Account{
		ID:       "acc-2",
		Address:  "NAddr2",
		LockedBy: "mixer",
		LockedAt: time.Now(),
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	released, err := svc.ReleaseAccounts(ctx, "mixer", []string{"acc-1", "acc-2"})
	if err != nil {
		t.Fatalf("ReleaseAccounts() error = %v", err)
	}

	if released != 2 {
		t.Errorf("released = %d, want 2", released)
	}
}

func TestUpdateBalanceWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID:       "acc-1",
		Address:  "NAddr1",
		Balance:  1000000,
		LockedBy: "mixer",
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	oldBalance, newBalance, err := svc.UpdateBalance(ctx, "mixer", "acc-1", 500000, nil)
	if err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}

	if oldBalance != 1000000 {
		t.Errorf("oldBalance = %d, want 1000000", oldBalance)
	}
	if newBalance != 1500000 {
		t.Errorf("newBalance = %d, want 1500000", newBalance)
	}
}

func TestGetPoolInfoWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	// Create various account states
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID: "acc-1", Balance: 1000000, LockedBy: "",
	}
	mockRepo.accounts["acc-2"] = &accountpoolsupabase.Account{
		ID: "acc-2", Balance: 2000000, LockedBy: "mixer",
	}
	mockRepo.accounts["acc-3"] = &accountpoolsupabase.Account{
		ID: "acc-3", Balance: 500000, IsRetiring: true,
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	info, err := svc.GetPoolInfo(ctx)
	if err != nil {
		t.Fatalf("GetPoolInfo() error = %v", err)
	}

	if info.TotalAccounts != 3 {
		t.Errorf("TotalAccounts = %d, want 3", info.TotalAccounts)
	}
	if info.TotalBalance != 3500000 {
		t.Errorf("TotalBalance = %d, want 3500000", info.TotalBalance)
	}
}

func TestSignTransactionWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID:       "acc-1",
		Address:  "NAddr1",
		LockedBy: "mixer",
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	txHash := crypto.Hash256([]byte("test transaction"))
	resp, err := svc.SignTransaction(ctx, "mixer", "acc-1", txHash)
	if err != nil {
		t.Fatalf("SignTransaction() error = %v", err)
	}

	if resp.AccountID != "acc-1" {
		t.Errorf("AccountID = %s, want acc-1", resp.AccountID)
	}
	if len(resp.Signature) != 64 {
		t.Errorf("Signature length = %d, want 64", len(resp.Signature))
	}
	if len(resp.PublicKey) == 0 {
		t.Error("PublicKey should not be empty")
	}
}

func TestBatchSignWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID: "acc-1", LockedBy: "mixer",
	}
	mockRepo.accounts["acc-2"] = &accountpoolsupabase.Account{
		ID: "acc-2", LockedBy: "mixer",
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	requests := []SignRequest{
		{AccountID: "acc-1", TxHash: crypto.Hash256([]byte("tx1"))},
		{AccountID: "acc-2", TxHash: crypto.Hash256([]byte("tx2"))},
	}

	resp := svc.BatchSign(ctx, "mixer", requests)

	if len(resp.Signatures) != 2 {
		t.Errorf("len(Signatures) = %d, want 2", len(resp.Signatures))
	}
	if len(resp.Errors) != 0 {
		t.Errorf("len(Errors) = %d, want 0", len(resp.Errors))
	}
}

func TestListAccountsByServiceWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID: "acc-1", Balance: 1000000, LockedBy: "mixer",
	}
	mockRepo.accounts["acc-2"] = &accountpoolsupabase.Account{
		ID: "acc-2", Balance: 500000, LockedBy: "mixer",
	}
	mockRepo.accounts["acc-3"] = &accountpoolsupabase.Account{
		ID: "acc-3", Balance: 2000000, LockedBy: "vrf",
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	accounts, err := svc.ListAccountsByService(ctx, "mixer", nil)
	if err != nil {
		t.Fatalf("ListAccountsByService() error = %v", err)
	}

	if len(accounts) != 2 {
		t.Errorf("len(accounts) = %d, want 2", len(accounts))
	}
}

func TestReleaseAllByServiceWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID: "acc-1", LockedBy: "mixer",
	}
	mockRepo.accounts["acc-2"] = &accountpoolsupabase.Account{
		ID: "acc-2", LockedBy: "mixer",
	}
	mockRepo.accounts["acc-3"] = &accountpoolsupabase.Account{
		ID: "acc-3", LockedBy: "vrf",
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	released, err := svc.ReleaseAllByService(ctx, "mixer")
	if err != nil {
		t.Fatalf("ReleaseAllByService() error = %v", err)
	}

	if released != 2 {
		t.Errorf("released = %d, want 2", released)
	}
}

func TestRequestAccountsPartialAvailable(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	// Create 2 available accounts
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID: "acc-1", LockedBy: "",
	}
	mockRepo.accounts["acc-2"] = &accountpoolsupabase.Account{
		ID: "acc-2", LockedBy: "",
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	// Request 2 accounts (all available)
	accounts, _, err := svc.RequestAccounts(ctx, "mixer", 2, "test")
	if err != nil {
		t.Fatalf("RequestAccounts() error = %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("len(accounts) = %d, want 2", len(accounts))
	}
}

func TestSignTransactionUnauthorized(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	mockRepo := newMockAccountPoolRepo()
	mockRepo.accounts["acc-1"] = &accountpoolsupabase.Account{
		ID: "acc-1", LockedBy: "mixer",
	}

	svc, _ := New(Config{Marble: m, AccountPoolRepo: mockRepo})
	ctx := context.Background()

	// Try to sign with wrong service ID
	txHash := crypto.Hash256([]byte("test transaction"))
	_, err := svc.SignTransaction(ctx, "vrf", "acc-1", txHash)
	if err == nil {
		t.Error("SignTransaction() should return error for unauthorized service")
	}
}
