// Package neosimulation provides contract invocation capabilities for the simulation service.
// All contract invocations use pool accounts managed by the neoaccounts service.
// Private keys never leave the TEE - signing happens inside the account pool service.
package neosimulation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	neoaccountsclient "github.com/R3E-Network/service_layer/infrastructure/accountpool/client"
)

// ContractInvoker handles smart contract invocations using pool accounts.
// All signing happens inside the TEE via the account pool service.
type ContractInvoker struct {
	poolClient PoolClientInterface

	// Contract addresses (as strings for InvokeContract API)
	priceFeedHash     string
	randomnessLogHash string
	paymentHubHash    string

	// Price feed configuration
	priceFeeds map[string]int64 // symbol -> base price (8 decimals)

	// Account management
	mu              sync.RWMutex
	lockedAccounts  map[string]string // purpose -> accountID
	accountBalances map[string]int64  // accountID -> estimated GAS balance

	// Statistics
	priceFeedUpdates  int64
	randomnessRecords int64
	paymentHubPays    int64
	contractErrors    int64

	// Round ID counter for price feeds
	roundID int64
}

// ContractInvokerConfig holds configuration for the contract invoker.
type ContractInvokerConfig struct {
	PoolClient        PoolClientInterface
	PriceFeedHash     string
	RandomnessLogHash string
	PaymentHubHash    string
}

// NewContractInvoker creates a new contract invoker using pool accounts.
func NewContractInvoker(cfg ContractInvokerConfig) (*ContractInvoker, error) {
	if cfg.PoolClient == nil {
		return nil, fmt.Errorf("pool client is required")
	}

	// Normalize contract hashes (remove 0x prefix if present)
	priceFeedHash := strings.TrimPrefix(cfg.PriceFeedHash, "0x")
	randomnessLogHash := strings.TrimPrefix(cfg.RandomnessLogHash, "0x")
	paymentHubHash := strings.TrimPrefix(cfg.PaymentHubHash, "0x")

	if priceFeedHash == "" || randomnessLogHash == "" || paymentHubHash == "" {
		return nil, fmt.Errorf("all contract hashes are required")
	}

	return &ContractInvoker{
		poolClient:        cfg.PoolClient,
		priceFeedHash:     priceFeedHash,
		randomnessLogHash: randomnessLogHash,
		paymentHubHash:    paymentHubHash,
		priceFeeds: map[string]int64{
			"BTCUSD": 10500000000000, // $105,000 (8 decimals)
			"ETHUSD": 390000000000,   // $3,900
			"NEOUSD": 1500000000,     // $15
			"GASUSD": 700000000,      // $7
		},
		lockedAccounts:  make(map[string]string),
		accountBalances: make(map[string]int64),
		roundID:         time.Now().Unix(),
	}, nil
}

// getOrRequestAccount gets an existing locked account or requests a new one.
func (inv *ContractInvoker) getOrRequestAccount(ctx context.Context, purpose string) (string, error) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	// Check if we already have an account for this purpose
	if accountID, ok := inv.lockedAccounts[purpose]; ok {
		return accountID, nil
	}

	// Request a new account from the pool
	resp, err := inv.poolClient.RequestAccounts(ctx, 1, purpose)
	if err != nil {
		return "", fmt.Errorf("request account: %w", err)
	}

	if len(resp.Accounts) == 0 {
		return "", fmt.Errorf("no accounts available in pool")
	}

	account := resp.Accounts[0]
	inv.lockedAccounts[purpose] = account.ID

	// Track initial balance if available
	if gasBalance, ok := account.Balances["GAS"]; ok {
		inv.accountBalances[account.ID] = gasBalance.Amount
	}

	return account.ID, nil
}

// releaseAccount releases an account back to the pool.
func (inv *ContractInvoker) releaseAccount(ctx context.Context, purpose string) {
	inv.mu.Lock()
	accountID, ok := inv.lockedAccounts[purpose]
	if ok {
		delete(inv.lockedAccounts, purpose)
		delete(inv.accountBalances, accountID)
	}
	inv.mu.Unlock()

	if ok {
		_, _ = inv.poolClient.ReleaseAccounts(ctx, []string{accountID})
	}
}

// UpdatePriceFeed updates a price feed with simulated data using the master wallet.
// PriceFeed requires the caller to be a registered TEE signer in AppRegistry.
func (inv *ContractInvoker) UpdatePriceFeed(ctx context.Context, symbol string) (string, error) {
	basePrice, ok := inv.priceFeeds[symbol]
	if !ok {
		return "", fmt.Errorf("unknown symbol: %s", symbol)
	}

	// Generate price with 2% variance
	price := generatePrice(basePrice, 2)
	timestamp := uint64(time.Now().UnixMilli())
	attestationHash := generateRandomBytes(32)
	sourceSetID := int64(1)

	// Increment round ID atomically
	roundID := atomic.AddInt64(&inv.roundID, 1)

	// Invoke contract via pool client using master wallet (TEE signer)
	// PriceFeed requires the caller to be registered in AppRegistry
	resp, err := inv.poolClient.InvokeMaster(ctx, inv.priceFeedHash, "update", []neoaccountsclient.ContractParam{
		{Type: "String", Value: symbol},
		{Type: "Integer", Value: roundID},
		{Type: "Integer", Value: price},
		{Type: "Integer", Value: timestamp},
		{Type: "ByteArray", Value: hex.EncodeToString(attestationHash)},
		{Type: "Integer", Value: sourceSetID},
	}, "") // Empty string = CalledByEntry (default)
	if err != nil {
		atomic.AddInt64(&inv.contractErrors, 1)
		return "", fmt.Errorf("invoke contract: %w", err)
	}

	if resp.State != "HALT" {
		atomic.AddInt64(&inv.contractErrors, 1)
		return "", fmt.Errorf("contract execution failed: %s", resp.Exception)
	}

	atomic.AddInt64(&inv.priceFeedUpdates, 1)
	return resp.TxHash, nil
}

// RecordRandomness records a randomness value on-chain using the master wallet.
// RandomnessLog requires the caller to be a registered TEE signer in AppRegistry.
func (inv *ContractInvoker) RecordRandomness(ctx context.Context) (string, error) {
	requestID := generateRequestID()
	randomness := generateRandomBytes(32)
	attestationHash := generateRandomBytes(32)
	timestamp := uint64(time.Now().UnixMilli())

	// Invoke contract via pool client using master wallet (TEE signer)
	// RandomnessLog requires the caller to be registered in AppRegistry
	resp, err := inv.poolClient.InvokeMaster(ctx, inv.randomnessLogHash, "record", []neoaccountsclient.ContractParam{
		{Type: "String", Value: requestID},
		{Type: "ByteArray", Value: hex.EncodeToString(randomness)},
		{Type: "ByteArray", Value: hex.EncodeToString(attestationHash)},
		{Type: "Integer", Value: timestamp},
	}, "") // Empty string = CalledByEntry (default)
	if err != nil {
		atomic.AddInt64(&inv.contractErrors, 1)
		return "", fmt.Errorf("invoke contract: %w", err)
	}

	if resp.State != "HALT" {
		atomic.AddInt64(&inv.contractErrors, 1)
		return "", fmt.Errorf("contract execution failed: %s", resp.Exception)
	}

	atomic.AddInt64(&inv.randomnessRecords, 1)
	return resp.TxHash, nil
}

// PayToApp makes a payment to a MiniApp via PaymentHub using a pool account.
func (inv *ContractInvoker) PayToApp(ctx context.Context, appID string, amount int64, memo string) (string, error) {
	// Get or request an account for payments (one per app for isolation)
	purpose := fmt.Sprintf("payment-%s", appID)
	accountID, err := inv.getOrRequestAccount(ctx, purpose)
	if err != nil {
		atomic.AddInt64(&inv.contractErrors, 1)
		return "", err
	}

	// Invoke contract via pool client (signing happens in TEE)
	// PaymentHub requires Global scope because it calls GAS.Transfer internally
	resp, err := inv.poolClient.InvokeContract(ctx, accountID, inv.paymentHubHash, "pay", []neoaccountsclient.ContractParam{
		{Type: "String", Value: appID},
		{Type: "Integer", Value: amount},
		{Type: "String", Value: memo},
	}, "Global") // Global scope required for GAS.Transfer
	if err != nil {
		atomic.AddInt64(&inv.contractErrors, 1)
		return "", fmt.Errorf("invoke contract: %w", err)
	}

	if resp.State != "HALT" {
		atomic.AddInt64(&inv.contractErrors, 1)
		return "", fmt.Errorf("contract execution failed: %s", resp.Exception)
	}

	atomic.AddInt64(&inv.paymentHubPays, 1)
	return resp.TxHash, nil
}

// GetStats returns contract invocation statistics.
func (inv *ContractInvoker) GetStats() map[string]interface{} {
	inv.mu.RLock()
	lockedCount := len(inv.lockedAccounts)
	inv.mu.RUnlock()

	return map[string]interface{}{
		"price_feed_updates":  atomic.LoadInt64(&inv.priceFeedUpdates),
		"randomness_records":  atomic.LoadInt64(&inv.randomnessRecords),
		"payment_hub_pays":    atomic.LoadInt64(&inv.paymentHubPays),
		"contract_errors":     atomic.LoadInt64(&inv.contractErrors),
		"locked_accounts":     lockedCount,
	}
}

// GetPriceSymbols returns the list of price feed symbols.
func (inv *ContractInvoker) GetPriceSymbols() []string {
	symbols := make([]string, 0, len(inv.priceFeeds))
	for symbol := range inv.priceFeeds {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// GetLockedAccountCount returns the number of currently locked accounts.
func (inv *ContractInvoker) GetLockedAccountCount() int {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return len(inv.lockedAccounts)
}

// ReleaseAllAccounts releases all locked accounts back to the pool.
func (inv *ContractInvoker) ReleaseAllAccounts(ctx context.Context) {
	inv.mu.Lock()
	accountIDs := make([]string, 0, len(inv.lockedAccounts))
	for _, accountID := range inv.lockedAccounts {
		accountIDs = append(accountIDs, accountID)
	}
	inv.lockedAccounts = make(map[string]string)
	inv.accountBalances = make(map[string]int64)
	inv.mu.Unlock()

	if len(accountIDs) > 0 {
		_, _ = inv.poolClient.ReleaseAccounts(ctx, accountIDs)
	}
}

// Close releases all accounts and cleans up resources.
func (inv *ContractInvoker) Close() {
	inv.ReleaseAllAccounts(context.Background())
}

func generatePrice(basePrice int64, variancePercent int) int64 {
	variance := basePrice * int64(variancePercent) / 100
	n, _ := rand.Int(rand.Reader, big.NewInt(variance*2))
	return basePrice - variance + n.Int64()
}

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// NewContractInvokerFromEnv creates a contract invoker from environment variables.
// This is a convenience function for creating the invoker with standard configuration.
func NewContractInvokerFromEnv(poolClient *neoaccountsclient.Client) (*ContractInvoker, error) {
	priceFeedHash := strings.TrimSpace(os.Getenv("CONTRACT_PRICEFEED_HASH"))
	randomnessLogHash := strings.TrimSpace(os.Getenv("CONTRACT_RANDOMNESSLOG_HASH"))
	paymentHubHash := strings.TrimSpace(os.Getenv("CONTRACT_PAYMENTHUB_HASH"))

	if priceFeedHash == "" || randomnessLogHash == "" || paymentHubHash == "" {
		return nil, fmt.Errorf("contract hashes not configured (CONTRACT_PRICEFEED_HASH, CONTRACT_RANDOMNESSLOG_HASH, CONTRACT_PAYMENTHUB_HASH)")
	}

	return NewContractInvoker(ContractInvokerConfig{
		PoolClient:        poolClient,
		PriceFeedHash:     priceFeedHash,
		RandomnessLogHash: randomnessLogHash,
		PaymentHubHash:    paymentHubHash,
	})
}
