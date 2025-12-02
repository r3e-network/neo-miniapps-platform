//go:build integration

// Package mixer_test provides integration tests for the mixer service with SGX SIM mode.
//
// These tests verify the complete mixer service workflow including:
// - TEE initialization in simulation mode
// - HD key derivation and multi-sig address generation
// - Mix request lifecycle (create, deposit, mix, withdraw)
// - Pool account management
// - Error handling and recovery
//
// Run with: go test -v -tags=integration ./tests/integration/mixer/...
package mixer_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"
	"time"

	mixer "github.com/R3E-Network/service_layer/packages/com.r3e.services.mixer/service"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/tee"
)

// testEnv holds the test environment configuration.
type testEnv struct {
	mu           sync.Mutex
	teeProvider  tee.EngineProvider
	mixerService *mixer.Service
	store        *memoryStore
	teeManager   *mockTEEManager
	masterKey    *mockMasterKeyProvider
	chainClient  *mockChainClient
	accountStore *mockAccountStore
	log          *logger.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	initialized  bool
}

var globalEnv *testEnv

// setupTestEnv initializes the test environment with SGX SIM mode.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	if globalEnv != nil && globalEnv.initialized {
		return globalEnv
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	log := logger.NewDefault("mixer-integration-test")

	// Initialize TEE provider in simulation mode
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("generate encryption key: %v", err)
	}

	if err := tee.InitializeProvider(tee.ProviderConfig{
		Mode:                    tee.EnclaveModeSimulation,
		SecretEncryptionKey:     encKey,
		MaxConcurrentExecutions: 10,
		V8HeapSize:              64 * 1024 * 1024, // 64MB
		RegisterDefaultPolicies: true,
	}); err != nil {
		// Provider might already be initialized
		log.WithError(err).Warn("TEE provider initialization (may already exist)")
	}

	provider := tee.GetProvider()
	if provider == nil {
		t.Fatal("TEE provider not available")
	}

	// Create mock dependencies
	store := newMemoryStore()
	teeManager := newMockTEEManager(provider)
	masterKey := newMockMasterKeyProvider()
	chainClient := newMockChainClient()
	accountStore := newMockAccountStore()

	// Create mixer service
	svc := mixer.New(accountStore, store, teeManager, masterKey, chainClient, log)

	globalEnv = &testEnv{
		teeProvider:  provider,
		mixerService: svc,
		store:        store,
		teeManager:   teeManager,
		masterKey:    masterKey,
		chainClient:  chainClient,
		accountStore: accountStore,
		log:          log,
		ctx:          ctx,
		cancel:       cancel,
		initialized:  true,
	}

	return globalEnv
}

// teardownTestEnv cleans up the test environment.
func teardownTestEnv(t *testing.T) {
	t.Helper()
	if globalEnv != nil {
		globalEnv.cancel()
		globalEnv = nil
	}
}

// TestMain sets up and tears down the test environment.
func TestMain(m *testing.M) {
	code := m.Run()
	if globalEnv != nil {
		globalEnv.cancel()
	}
	os.Exit(code)
}

// --- Memory Store Implementation ---

type memoryStore struct {
	mu       sync.RWMutex
	requests map[string]mixer.MixRequest
	pools    map[string]mixer.PoolAccount
	txs      map[string]mixer.MixTransaction
	claims   map[string]mixer.WithdrawalClaim
	hdIndex  uint32
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		requests: make(map[string]mixer.MixRequest),
		pools:    make(map[string]mixer.PoolAccount),
		txs:      make(map[string]mixer.MixTransaction),
		claims:   make(map[string]mixer.WithdrawalClaim),
		hdIndex:  0,
	}
}

func (s *memoryStore) CreateMixRequest(ctx context.Context, req mixer.MixRequest) (mixer.MixRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()
	req.ID = id
	req.CreatedAt = time.Now().UTC()
	req.UpdatedAt = req.CreatedAt
	s.requests[id] = req
	return req, nil
}

func (s *memoryStore) GetMixRequest(ctx context.Context, id string) (mixer.MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.requests[id]
	if !ok {
		return mixer.MixRequest{}, mixer.ErrRequestNotFound
	}
	return req, nil
}

func (s *memoryStore) GetMixRequestByProofHash(ctx context.Context, proofHash string) (mixer.MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, req := range s.requests {
		if req.ZKProofHash == proofHash {
			return req, nil
		}
	}
	return mixer.MixRequest{}, mixer.ErrRequestNotFound
}

func (s *memoryStore) UpdateMixRequest(ctx context.Context, req mixer.MixRequest) (mixer.MixRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.requests[req.ID]; !ok {
		return mixer.MixRequest{}, mixer.ErrRequestNotFound
	}
	req.UpdatedAt = time.Now().UTC()
	s.requests[req.ID] = req
	return req, nil
}

func (s *memoryStore) ListMixRequests(ctx context.Context, accountID string, limit int) ([]mixer.MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.MixRequest
	for _, req := range s.requests {
		if accountID == "" || req.AccountID == accountID {
			result = append(result, req)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) ListMixRequestsByStatus(ctx context.Context, status mixer.RequestStatus, limit int) ([]mixer.MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.MixRequest
	for _, req := range s.requests {
		if req.Status == status {
			result = append(result, req)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) ListPendingMixRequests(ctx context.Context) ([]mixer.MixRequest, error) {
	return s.ListMixRequestsByStatus(ctx, mixer.RequestStatusPending, 0)
}

func (s *memoryStore) ListExpiredMixRequests(ctx context.Context, before time.Time) ([]mixer.MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.MixRequest
	for _, req := range s.requests {
		if req.WithdrawableAt.Before(before) {
			result = append(result, req)
		}
	}
	return result, nil
}

func (s *memoryStore) CreatePoolAccount(ctx context.Context, pool mixer.PoolAccount) (mixer.PoolAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()
	pool.ID = id
	pool.CreatedAt = time.Now().UTC()
	pool.UpdatedAt = pool.CreatedAt
	s.pools[id] = pool
	return pool, nil
}

func (s *memoryStore) GetPoolAccount(ctx context.Context, id string) (mixer.PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pool, ok := s.pools[id]
	if !ok {
		return mixer.PoolAccount{}, mixer.ErrRequestNotFound
	}
	return pool, nil
}

func (s *memoryStore) GetPoolAccountByWallet(ctx context.Context, wallet string) (mixer.PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, pool := range s.pools {
		if pool.WalletAddress == wallet {
			return pool, nil
		}
	}
	return mixer.PoolAccount{}, mixer.ErrRequestNotFound
}

func (s *memoryStore) UpdatePoolAccount(ctx context.Context, pool mixer.PoolAccount) (mixer.PoolAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.pools[pool.ID]; !ok {
		return mixer.PoolAccount{}, mixer.ErrRequestNotFound
	}
	pool.UpdatedAt = time.Now().UTC()
	s.pools[pool.ID] = pool
	return pool, nil
}

func (s *memoryStore) ListPoolAccounts(ctx context.Context, status mixer.PoolAccountStatus) ([]mixer.PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.PoolAccount
	for _, pool := range s.pools {
		if status == "" || pool.Status == status {
			result = append(result, pool)
		}
	}
	return result, nil
}

func (s *memoryStore) ListActivePoolAccounts(ctx context.Context) ([]mixer.PoolAccount, error) {
	return s.ListPoolAccounts(ctx, mixer.PoolAccountStatusActive)
}

func (s *memoryStore) ListRetirablePoolAccounts(ctx context.Context, before time.Time) ([]mixer.PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.PoolAccount
	for _, pool := range s.pools {
		if pool.RetireAfter.Before(before) {
			result = append(result, pool)
		}
	}
	return result, nil
}

func (s *memoryStore) CreateMixTransaction(ctx context.Context, tx mixer.MixTransaction) (mixer.MixTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()
	tx.ID = id
	tx.CreatedAt = time.Now().UTC()
	tx.UpdatedAt = tx.CreatedAt
	s.txs[id] = tx
	return tx, nil
}

func (s *memoryStore) UpdateMixTransaction(ctx context.Context, tx mixer.MixTransaction) (mixer.MixTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.txs[tx.ID]; !ok {
		return mixer.MixTransaction{}, mixer.ErrRequestNotFound
	}
	tx.UpdatedAt = time.Now().UTC()
	s.txs[tx.ID] = tx
	return tx, nil
}

func (s *memoryStore) GetMixTransaction(ctx context.Context, id string) (mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.txs[id]
	if !ok {
		return mixer.MixTransaction{}, mixer.ErrRequestNotFound
	}
	return tx, nil
}

func (s *memoryStore) GetMixTransactionByHash(ctx context.Context, txHash string) (mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, tx := range s.txs {
		if tx.TxHash == txHash {
			return tx, nil
		}
	}
	return mixer.MixTransaction{}, mixer.ErrRequestNotFound
}

func (s *memoryStore) ListMixTransactions(ctx context.Context, requestID string, limit int) ([]mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.MixTransaction
	for _, tx := range s.txs {
		if tx.RequestID == requestID {
			result = append(result, tx)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) ListMixTransactionsByPool(ctx context.Context, poolID string, limit int) ([]mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.MixTransaction
	for _, tx := range s.txs {
		if tx.FromPoolID == poolID || tx.ToPoolID == poolID {
			result = append(result, tx)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) ListScheduledMixTransactions(ctx context.Context, before time.Time, limit int) ([]mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.MixTransaction
	for _, tx := range s.txs {
		if tx.ScheduledAt.Before(before) && tx.Status == mixer.MixTxStatusScheduled {
			result = append(result, tx)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) ListPendingMixTransactions(ctx context.Context) ([]mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.MixTransaction
	for _, tx := range s.txs {
		if tx.Status == mixer.MixTxStatusPending {
			result = append(result, tx)
		}
	}
	return result, nil
}

func (s *memoryStore) CreateWithdrawalClaim(ctx context.Context, claim mixer.WithdrawalClaim) (mixer.WithdrawalClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()
	claim.ID = id
	claim.CreatedAt = time.Now().UTC()
	s.claims[id] = claim
	return claim, nil
}

func (s *memoryStore) UpdateWithdrawalClaim(ctx context.Context, claim mixer.WithdrawalClaim) (mixer.WithdrawalClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.claims[claim.ID]; !ok {
		return mixer.WithdrawalClaim{}, mixer.ErrRequestNotFound
	}
	s.claims[claim.ID] = claim
	return claim, nil
}

func (s *memoryStore) GetWithdrawalClaim(ctx context.Context, id string) (mixer.WithdrawalClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	claim, ok := s.claims[id]
	if !ok {
		return mixer.WithdrawalClaim{}, mixer.ErrRequestNotFound
	}
	return claim, nil
}

func (s *memoryStore) GetWithdrawalClaimByRequest(ctx context.Context, requestID string) (mixer.WithdrawalClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, claim := range s.claims {
		if claim.RequestID == requestID {
			return claim, nil
		}
	}
	return mixer.WithdrawalClaim{}, mixer.ErrRequestNotFound
}

func (s *memoryStore) ListWithdrawalClaims(ctx context.Context, accountID string, limit int) ([]mixer.WithdrawalClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.WithdrawalClaim
	for _, claim := range s.claims {
		if accountID == "" || claim.AccountID == accountID {
			result = append(result, claim)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) ListClaimableWithdrawals(ctx context.Context, before time.Time) ([]mixer.WithdrawalClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mixer.WithdrawalClaim
	for _, claim := range s.claims {
		if claim.ClaimableAt.Before(before) && claim.Status == mixer.ClaimStatusPending {
			result = append(result, claim)
		}
	}
	return result, nil
}

func (s *memoryStore) GetServiceDeposit(ctx context.Context) (mixer.ServiceDeposit, error) {
	return mixer.ServiceDeposit{
		Amount:          "1000000000000000",
		LockedAmount:    "0",
		AvailableAmount: "1000000000000000",
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

func (s *memoryStore) UpdateServiceDeposit(ctx context.Context, deposit mixer.ServiceDeposit) (mixer.ServiceDeposit, error) {
	deposit.UpdatedAt = time.Now().UTC()
	return deposit, nil
}

func (s *memoryStore) GetMixStats(ctx context.Context) (mixer.MixStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return mixer.MixStats{
		TotalRequests:      int64(len(s.requests)),
		TotalVolume:        "0",
		ActivePoolAccounts: int64(len(s.pools)),
	}, nil
}

// --- Mock TEE Manager ---

type mockTEEManager struct {
	provider tee.EngineProvider
	keys     map[uint32][]byte // HD index -> derived key
	mu       sync.RWMutex
	hdIndex  uint32
}

func newMockTEEManager(provider tee.EngineProvider) *mockTEEManager {
	return &mockTEEManager{
		provider: provider,
		keys:     make(map[uint32][]byte),
	}
}

func (m *mockTEEManager) DerivePoolKeys(ctx context.Context, index uint32, masterPublicKey []byte) (*mixer.PoolKeyPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	teeKey := make([]byte, 33)
	teeKey[0] = 0x02
	rand.Read(teeKey[1:])
	m.keys[index] = teeKey

	return &mixer.PoolKeyPair{
		Index:           index,
		TEEPublicKey:    teeKey,
		MasterPublicKey: masterPublicKey,
		Address:         "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		MultiSigScript:  []byte("mock-multisig-script"),
	}, nil
}

func (m *mockTEEManager) SignTransaction(ctx context.Context, hdIndex uint32, txData []byte) ([]byte, error) {
	// Simulate signing
	sig := make([]byte, 64)
	if _, err := rand.Read(sig); err != nil {
		return nil, err
	}
	return sig, nil
}

func (m *mockTEEManager) GetTEEPublicKey(ctx context.Context, hdIndex uint32) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if key, ok := m.keys[hdIndex]; ok {
		return key, nil
	}

	key := make([]byte, 33)
	key[0] = 0x02
	rand.Read(key[1:])
	m.keys[hdIndex] = key
	return key, nil
}

func (m *mockTEEManager) GetNextPoolIndex(ctx context.Context) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hdIndex++
	return m.hdIndex, nil
}

func (m *mockTEEManager) GenerateZKProof(ctx context.Context, req mixer.MixRequest) (string, error) {
	proof := make([]byte, 32)
	rand.Read(proof)
	return hex.EncodeToString(proof), nil
}

func (m *mockTEEManager) SignAttestation(ctx context.Context, data []byte) (string, error) {
	sig := make([]byte, 64)
	rand.Read(sig)
	return hex.EncodeToString(sig), nil
}

func (m *mockTEEManager) VerifyAttestation(ctx context.Context, data []byte, signature string) (bool, error) {
	return true, nil
}

// --- Mock Master Key Provider ---

type mockMasterKeyProvider struct {
	keys map[uint32][]byte
	mu   sync.Mutex
}

func newMockMasterKeyProvider() *mockMasterKeyProvider {
	return &mockMasterKeyProvider{
		keys: make(map[uint32][]byte),
	}
}

func (m *mockMasterKeyProvider) GetMasterPublicKey(ctx context.Context, hdIndex uint32) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if key, ok := m.keys[hdIndex]; ok {
		return key, nil
	}

	pubKey := make([]byte, 33) // Compressed public key
	pubKey[0] = 0x03           // Odd y-coordinate prefix
	rand.Read(pubKey[1:])
	m.keys[hdIndex] = pubKey
	return pubKey, nil
}

func (m *mockMasterKeyProvider) VerifyMasterSignature(ctx context.Context, hdIndex uint32, data, signature []byte) (bool, error) {
	return true, nil
}

// --- Mock Chain Client ---

type mockChainClient struct {
	mu           sync.Mutex
	transactions map[string]bool
	balances     map[string]string
}

func newMockChainClient() *mockChainClient {
	return &mockChainClient{
		transactions: make(map[string]bool),
		balances:     make(map[string]string),
	}
}

func (c *mockChainClient) GetBalance(ctx context.Context, address string, tokenAddress string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if bal, ok := c.balances[address]; ok {
		return bal, nil
	}
	return "1000.0", nil
}

func (c *mockChainClient) SendTransaction(ctx context.Context, signedTx []byte) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	txHash := generateID()
	c.transactions[txHash] = true
	return txHash, nil
}

func (c *mockChainClient) GetTransactionStatus(ctx context.Context, txHash string) (bool, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.transactions[txHash]; ok {
		return true, 12345, nil
	}
	return false, 0, nil
}

func (c *mockChainClient) BuildTransferTx(ctx context.Context, from, to, amount, tokenAddress string) ([]byte, error) {
	return []byte("mock-unsigned-tx"), nil
}

func (c *mockChainClient) SubmitMixProof(ctx context.Context, requestID, proofHash, teeSignature string) (string, error) {
	return generateID(), nil
}

func (c *mockChainClient) SubmitCompletionProof(ctx context.Context, requestID string, deliveredAmount string) (string, error) {
	return generateID(), nil
}

func (c *mockChainClient) GetWithdrawableRequests(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (c *mockChainClient) SetBalance(address, amount string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.balances[address] = amount
}

// --- Mock Account Store ---

type mockAccountStore struct {
	accounts map[string]bool
	mu       sync.RWMutex
}

func newMockAccountStore() *mockAccountStore {
	return &mockAccountStore{
		accounts: map[string]bool{
			"test-account-1": true,
			"test-account-2": true,
			"test-account-3": true,
		},
	}
}

func (s *mockAccountStore) AccountExists(ctx context.Context, accountID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.accounts[accountID]; !ok {
		return mixer.ErrRequestNotFound
	}
	return nil
}

func (s *mockAccountStore) AccountTenant(ctx context.Context, accountID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.accounts[accountID]; !ok {
		return ""
	}
	return "default"
}

// --- Helpers ---

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
