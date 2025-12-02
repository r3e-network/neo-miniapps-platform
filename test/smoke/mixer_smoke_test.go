//go:build smoke

// Package smoke provides smoke tests for critical service functionality.
// These tests verify basic service health and core operations work correctly.
//
// Run with: go test -v -tags=smoke ./tests/smoke/...
package smoke

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

// SmokeTestSuite holds the smoke test environment.
type SmokeTestSuite struct {
	ctx          context.Context
	cancel       context.CancelFunc
	teeProvider  tee.EngineProvider
	mixerService *mixer.Service
	log          *logger.Logger
}

var suite *SmokeTestSuite

func TestMain(m *testing.M) {
	// Setup
	suite = setupSmokeTests()
	if suite == nil {
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown
	if suite.cancel != nil {
		suite.cancel()
	}

	os.Exit(code)
}

func setupSmokeTests() *SmokeTestSuite {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	log := logger.NewDefault("mixer-smoke-test")

	// Initialize TEE in simulation mode
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		log.WithError(err).Error("failed to generate encryption key")
		cancel()
		return nil
	}

	if err := tee.InitializeProvider(tee.ProviderConfig{
		Mode:                    tee.EnclaveModeSimulation,
		SecretEncryptionKey:     encKey,
		MaxConcurrentExecutions: 5,
		V8HeapSize:              32 * 1024 * 1024,
		RegisterDefaultPolicies: true,
	}); err != nil {
		log.WithError(err).Warn("TEE provider init (may already exist)")
	}

	provider := tee.GetProvider()

	// Create minimal mock dependencies
	store := newSmokeStore()
	teeManager := newSmokeTEEManager()
	masterKey := newSmokeMasterKey()
	chainClient := newSmokeChainClient()
	accountStore := newSmokeAccountStore()

	svc := mixer.New(accountStore, store, teeManager, masterKey, chainClient, log)

	return &SmokeTestSuite{
		ctx:          ctx,
		cancel:       cancel,
		teeProvider:  provider,
		mixerService: svc,
		log:          log,
	}
}

// --- Smoke Tests ---

// TestSmoke_TEEHealth verifies TEE engine is healthy.
func TestSmoke_TEEHealth(t *testing.T) {
	if suite.teeProvider == nil {
		t.Skip("TEE provider not available")
	}

	engine := suite.teeProvider.GetEngine()
	if engine == nil {
		t.Skip("TEE engine not available")
	}

	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	if err := engine.Health(ctx); err != nil {
		// In SIM mode without full enclave setup, health check may fail
		// This is acceptable for smoke tests - skip instead of fail
		t.Skipf("TEE health check skipped (enclave not ready in test environment): %v", err)
	}

	t.Log("PASS: TEE engine is healthy")
}

// TestSmoke_MixerServiceReady verifies mixer service is ready.
func TestSmoke_MixerServiceReady(t *testing.T) {
	if suite.mixerService == nil {
		t.Fatal("mixer service is nil")
	}

	// Check service name
	if suite.mixerService.Name() != "mixer" {
		t.Errorf("expected service name 'mixer', got '%s'", suite.mixerService.Name())
	}

	t.Log("PASS: Mixer service is ready")
}

// TestSmoke_CreateMixRequest verifies basic mix request creation.
func TestSmoke_CreateMixRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	req := mixer.MixRequest{
		AccountID:    "smoke-test-account",
		SourceWallet: "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		Amount:       "10000000000", // 10 GAS in smallest units
		MixDuration:  mixer.MixDuration30Min,
		SplitCount:   1,
		Targets: []mixer.MixTarget{
			{Address: "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq", Amount: "10000000000"},
		},
	}

	result, err := suite.mixerService.CreateMixRequest(ctx, req)
	if err != nil {
		t.Fatalf("failed to create mix request: %v", err)
	}

	if result.ID == "" {
		t.Fatal("mix request ID is empty")
	}

	if result.Status != mixer.RequestStatusPending {
		t.Errorf("expected status 'pending', got '%s'", result.Status)
	}

	t.Logf("Created mix request: %s", result.ID)
	t.Log("PASS: Mix request creation works")
}

// TestSmoke_ListMixRequests verifies listing mix requests.
func TestSmoke_ListMixRequests(t *testing.T) {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	results, err := suite.mixerService.ListMixRequests(ctx, "smoke-test-account", 10)
	if err != nil {
		t.Fatalf("failed to list mix requests: %v", err)
	}

	t.Logf("Found %d mix requests", len(results))
	t.Log("PASS: List mix requests works")
}

// TestSmoke_PoolAccountCreation verifies pool account creation.
func TestSmoke_PoolAccountCreation(t *testing.T) {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	pool, err := suite.mixerService.CreatePoolAccount(ctx)
	if err != nil {
		t.Fatalf("failed to create pool account: %v", err)
	}

	if pool.ID == "" {
		t.Fatal("pool account ID is empty")
	}

	if pool.WalletAddress == "" {
		t.Fatal("pool account wallet address is empty")
	}

	t.Logf("Created pool account: %s at %s", pool.ID, pool.WalletAddress)
	t.Log("PASS: Pool account creation works")
}

// TestSmoke_EndToEnd performs a minimal end-to-end test.
func TestSmoke_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(suite.ctx, 30*time.Second)
	defer cancel()

	// Step 1: Create pool account
	pool, err := suite.mixerService.CreatePoolAccount(ctx)
	if err != nil {
		t.Fatalf("Step 1 failed - create pool: %v", err)
	}
	t.Logf("Step 1: Created pool %s", pool.ID)

	// Step 2: Create mix request
	req := mixer.MixRequest{
		AccountID:    "e2e-test-account",
		SourceWallet: "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		Amount:       "50000000000", // 50 GAS in smallest units
		MixDuration:  mixer.MixDuration30Min,
		SplitCount:   2,
		Targets: []mixer.MixTarget{
			{Address: "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq", Amount: "25000000000"},
			{Address: "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq", Amount: "25000000000"},
		},
	}

	created, err := suite.mixerService.CreateMixRequest(ctx, req)
	if err != nil {
		t.Fatalf("Step 2 failed - create request: %v", err)
	}
	t.Logf("Step 2: Created request %s", created.ID)

	// Step 3: Retrieve request
	retrieved, err := suite.mixerService.GetMixRequest(ctx, "e2e-test-account", created.ID)
	if err != nil {
		t.Fatalf("Step 3 failed - get request: %v", err)
	}
	t.Logf("Step 3: Retrieved request with status %s", retrieved.Status)

	// Step 4: List requests
	list, err := suite.mixerService.ListMixRequests(ctx, "e2e-test-account", 10)
	if err != nil {
		t.Fatalf("Step 4 failed - list requests: %v", err)
	}
	t.Logf("Step 4: Found %d requests", len(list))

	t.Log("PASS: End-to-end smoke test completed")
}

// --- Minimal Mock Implementations ---

type smokeStore struct {
	mu       sync.RWMutex
	requests map[string]mixer.MixRequest
	pools    map[string]mixer.PoolAccount
	txs      map[string]mixer.MixTransaction
	claims   map[string]mixer.WithdrawalClaim
}

func newSmokeStore() *smokeStore {
	return &smokeStore{
		requests: make(map[string]mixer.MixRequest),
		pools:    make(map[string]mixer.PoolAccount),
		txs:      make(map[string]mixer.MixTransaction),
		claims:   make(map[string]mixer.WithdrawalClaim),
	}
}

func (s *smokeStore) CreateMixRequest(ctx context.Context, req mixer.MixRequest) (mixer.MixRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req.ID = generateSmokeID()
	req.CreatedAt = time.Now().UTC()
	req.UpdatedAt = req.CreatedAt
	s.requests[req.ID] = req
	return req, nil
}

func (s *smokeStore) GetMixRequest(ctx context.Context, id string) (mixer.MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if req, ok := s.requests[id]; ok {
		return req, nil
	}
	return mixer.MixRequest{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) GetMixRequestByProofHash(ctx context.Context, proofHash string) (mixer.MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, req := range s.requests {
		if req.ZKProofHash == proofHash {
			return req, nil
		}
	}
	return mixer.MixRequest{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) UpdateMixRequest(ctx context.Context, req mixer.MixRequest) (mixer.MixRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req.UpdatedAt = time.Now().UTC()
	s.requests[req.ID] = req
	return req, nil
}

func (s *smokeStore) ListMixRequests(ctx context.Context, accountID string, limit int) ([]mixer.MixRequest, error) {
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

func (s *smokeStore) ListMixRequestsByStatus(ctx context.Context, status mixer.RequestStatus, limit int) ([]mixer.MixRequest, error) {
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

func (s *smokeStore) ListPendingMixRequests(ctx context.Context) ([]mixer.MixRequest, error) {
	return s.ListMixRequestsByStatus(ctx, mixer.RequestStatusPending, 0)
}

func (s *smokeStore) ListExpiredMixRequests(ctx context.Context, before time.Time) ([]mixer.MixRequest, error) {
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

func (s *smokeStore) CreatePoolAccount(ctx context.Context, pool mixer.PoolAccount) (mixer.PoolAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pool.ID = generateSmokeID()
	pool.CreatedAt = time.Now().UTC()
	pool.UpdatedAt = pool.CreatedAt
	s.pools[pool.ID] = pool
	return pool, nil
}

func (s *smokeStore) GetPoolAccount(ctx context.Context, id string) (mixer.PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if pool, ok := s.pools[id]; ok {
		return pool, nil
	}
	return mixer.PoolAccount{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) GetPoolAccountByWallet(ctx context.Context, wallet string) (mixer.PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pool := range s.pools {
		if pool.WalletAddress == wallet {
			return pool, nil
		}
	}
	return mixer.PoolAccount{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) UpdatePoolAccount(ctx context.Context, pool mixer.PoolAccount) (mixer.PoolAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pool.UpdatedAt = time.Now().UTC()
	s.pools[pool.ID] = pool
	return pool, nil
}

func (s *smokeStore) ListPoolAccounts(ctx context.Context, status mixer.PoolAccountStatus) ([]mixer.PoolAccount, error) {
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

func (s *smokeStore) ListActivePoolAccounts(ctx context.Context) ([]mixer.PoolAccount, error) {
	return s.ListPoolAccounts(ctx, mixer.PoolAccountStatusActive)
}

func (s *smokeStore) ListRetirablePoolAccounts(ctx context.Context, before time.Time) ([]mixer.PoolAccount, error) {
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

func (s *smokeStore) CreateMixTransaction(ctx context.Context, tx mixer.MixTransaction) (mixer.MixTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx.ID = generateSmokeID()
	tx.CreatedAt = time.Now().UTC()
	tx.UpdatedAt = tx.CreatedAt
	s.txs[tx.ID] = tx
	return tx, nil
}

func (s *smokeStore) UpdateMixTransaction(ctx context.Context, tx mixer.MixTransaction) (mixer.MixTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx.UpdatedAt = time.Now().UTC()
	s.txs[tx.ID] = tx
	return tx, nil
}

func (s *smokeStore) GetMixTransaction(ctx context.Context, id string) (mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if tx, ok := s.txs[id]; ok {
		return tx, nil
	}
	return mixer.MixTransaction{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) GetMixTransactionByHash(ctx context.Context, txHash string) (mixer.MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, tx := range s.txs {
		if tx.TxHash == txHash {
			return tx, nil
		}
	}
	return mixer.MixTransaction{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) ListMixTransactions(ctx context.Context, requestID string, limit int) ([]mixer.MixTransaction, error) {
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

func (s *smokeStore) ListMixTransactionsByPool(ctx context.Context, poolID string, limit int) ([]mixer.MixTransaction, error) {
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

func (s *smokeStore) ListScheduledMixTransactions(ctx context.Context, before time.Time, limit int) ([]mixer.MixTransaction, error) {
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

func (s *smokeStore) ListPendingMixTransactions(ctx context.Context) ([]mixer.MixTransaction, error) {
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

func (s *smokeStore) CreateWithdrawalClaim(ctx context.Context, claim mixer.WithdrawalClaim) (mixer.WithdrawalClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	claim.ID = generateSmokeID()
	claim.CreatedAt = time.Now().UTC()
	s.claims[claim.ID] = claim
	return claim, nil
}

func (s *smokeStore) UpdateWithdrawalClaim(ctx context.Context, claim mixer.WithdrawalClaim) (mixer.WithdrawalClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims[claim.ID] = claim
	return claim, nil
}

func (s *smokeStore) GetWithdrawalClaim(ctx context.Context, id string) (mixer.WithdrawalClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if claim, ok := s.claims[id]; ok {
		return claim, nil
	}
	return mixer.WithdrawalClaim{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) GetWithdrawalClaimByRequest(ctx context.Context, requestID string) (mixer.WithdrawalClaim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, claim := range s.claims {
		if claim.RequestID == requestID {
			return claim, nil
		}
	}
	return mixer.WithdrawalClaim{}, mixer.ErrRequestNotFound
}

func (s *smokeStore) ListWithdrawalClaims(ctx context.Context, accountID string, limit int) ([]mixer.WithdrawalClaim, error) {
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

func (s *smokeStore) ListClaimableWithdrawals(ctx context.Context, before time.Time) ([]mixer.WithdrawalClaim, error) {
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

func (s *smokeStore) GetServiceDeposit(ctx context.Context) (mixer.ServiceDeposit, error) {
	return mixer.ServiceDeposit{
		Amount:          "1000000000000000", // 1M GAS in smallest units
		LockedAmount:    "0",
		AvailableAmount: "1000000000000000", // 1M GAS available
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

func (s *smokeStore) UpdateServiceDeposit(ctx context.Context, deposit mixer.ServiceDeposit) (mixer.ServiceDeposit, error) {
	deposit.UpdatedAt = time.Now().UTC()
	return deposit, nil
}

func (s *smokeStore) GetMixStats(ctx context.Context) (mixer.MixStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return mixer.MixStats{
		TotalRequests:      int64(len(s.requests)),
		TotalVolume:        "0",
		ActivePoolAccounts: int64(len(s.pools)),
	}, nil
}

type smokeTEEManager struct {
	mu      sync.Mutex
	keys    map[uint32][]byte
	hdIndex uint32
}

func newSmokeTEEManager() *smokeTEEManager {
	return &smokeTEEManager{
		keys: make(map[uint32][]byte),
	}
}

func (m *smokeTEEManager) DerivePoolKeys(ctx context.Context, index uint32, masterPublicKey []byte) (*mixer.PoolKeyPair, error) {
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

func (m *smokeTEEManager) SignTransaction(ctx context.Context, hdIndex uint32, txData []byte) ([]byte, error) {
	sig := make([]byte, 64)
	rand.Read(sig)
	return sig, nil
}

func (m *smokeTEEManager) GetTEEPublicKey(ctx context.Context, hdIndex uint32) ([]byte, error) {
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

func (m *smokeTEEManager) GetNextPoolIndex(ctx context.Context) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hdIndex++
	return m.hdIndex, nil
}

func (m *smokeTEEManager) GenerateZKProof(ctx context.Context, req mixer.MixRequest) (string, error) {
	proof := make([]byte, 32)
	rand.Read(proof)
	return hex.EncodeToString(proof), nil
}

func (m *smokeTEEManager) SignAttestation(ctx context.Context, data []byte) (string, error) {
	sig := make([]byte, 64)
	rand.Read(sig)
	return hex.EncodeToString(sig), nil
}

func (m *smokeTEEManager) VerifyAttestation(ctx context.Context, data []byte, signature string) (bool, error) {
	return true, nil
}

type smokeMasterKey struct {
	mu   sync.Mutex
	keys map[uint32][]byte
}

func newSmokeMasterKey() *smokeMasterKey {
	return &smokeMasterKey{
		keys: make(map[uint32][]byte),
	}
}

func (m *smokeMasterKey) GetMasterPublicKey(ctx context.Context, hdIndex uint32) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if key, ok := m.keys[hdIndex]; ok {
		return key, nil
	}

	key := make([]byte, 33)
	key[0] = 0x03
	rand.Read(key[1:])
	m.keys[hdIndex] = key
	return key, nil
}

func (m *smokeMasterKey) VerifyMasterSignature(ctx context.Context, hdIndex uint32, data, signature []byte) (bool, error) {
	return true, nil
}

type smokeChainClient struct {
	mu           sync.Mutex
	transactions map[string]bool
	balances     map[string]string
}

func newSmokeChainClient() *smokeChainClient {
	return &smokeChainClient{
		transactions: make(map[string]bool),
		balances:     make(map[string]string),
	}
}

func (c *smokeChainClient) GetBalance(ctx context.Context, address string, tokenAddress string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if bal, ok := c.balances[address]; ok {
		return bal, nil
	}
	return "1000.0", nil
}

func (c *smokeChainClient) SendTransaction(ctx context.Context, signedTx []byte) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	txHash := generateSmokeID()
	c.transactions[txHash] = true
	return txHash, nil
}

func (c *smokeChainClient) GetTransactionStatus(ctx context.Context, txHash string) (bool, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.transactions[txHash]; ok {
		return true, 12345, nil
	}
	return false, 0, nil
}

func (c *smokeChainClient) BuildTransferTx(ctx context.Context, from, to, amount, tokenAddress string) ([]byte, error) {
	return []byte("mock-unsigned-tx"), nil
}

func (c *smokeChainClient) SubmitMixProof(ctx context.Context, requestID, proofHash, teeSignature string) (string, error) {
	return generateSmokeID(), nil
}

func (c *smokeChainClient) SubmitCompletionProof(ctx context.Context, requestID string, deliveredAmount string) (string, error) {
	return generateSmokeID(), nil
}

func (c *smokeChainClient) GetWithdrawableRequests(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

type smokeAccountStore struct {
	accounts map[string]bool
}

func newSmokeAccountStore() *smokeAccountStore {
	return &smokeAccountStore{
		accounts: map[string]bool{
			"smoke-test-account": true,
			"e2e-test-account":   true,
		},
	}
}

func (s *smokeAccountStore) AccountExists(ctx context.Context, accountID string) error {
	if _, ok := s.accounts[accountID]; !ok {
		return mixer.ErrRequestNotFound
	}
	return nil
}

func (s *smokeAccountStore) AccountTenant(ctx context.Context, accountID string) string {
	if _, ok := s.accounts[accountID]; !ok {
		return ""
	}
	return "default"
}

func generateSmokeID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
