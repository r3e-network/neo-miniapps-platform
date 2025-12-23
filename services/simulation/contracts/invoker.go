// Package contracts provides contract invocation utilities for the simulation service.
package contracts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// ContractAddresses holds the addresses of deployed contracts.
type ContractAddresses struct {
	PriceFeed        string
	RandomnessLog    string
	PaymentHub       string
	AutomationAnchor string
	AppRegistry      string
	Governance       string
}

// Invoker provides methods to invoke smart contracts.
type Invoker struct {
	client    *rpcclient.Client
	actor     *actor.Actor
	addresses ContractAddresses
}

// NewInvoker creates a new contract invoker.
func NewInvoker(rpcURL string) (*Invoker, error) {
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		return nil, fmt.Errorf("NEO_TESTNET_WIF environment variable not set")
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		return nil, fmt.Errorf("invalid WIF: %w", err)
	}

	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		return nil, fmt.Errorf("create RPC client: %w", err)
	}

	acc := wallet.NewAccountFromPrivateKey(privateKey)
	acc.Label = "simulation"

	act, err := actor.NewSimple(client, acc)
	if err != nil {
		return nil, fmt.Errorf("create actor: %w", err)
	}

	// Load contract addresses from environment
	addresses := ContractAddresses{
		PriceFeed:        getEnvOrDefault("CONTRACT_PRICEFEED_HASH", "0xc5d9117d255054489d1cf59b2c1d188c01bc9954"),
		RandomnessLog:    getEnvOrDefault("CONTRACT_RANDOMNESSLOG_HASH", "0x76dfee17f2f4b9fa8f32bd3f4da6406319ab7b39"),
		PaymentHub:       getEnvOrDefault("CONTRACT_PAYMENTHUB_HASH", "0x45777109546ceaacfbeed9336d695bb8b8bd77ca"),
		AutomationAnchor: getEnvOrDefault("CONTRACT_AUTOMATIONANCHOR_HASH", "0x1c888d699ce76b0824028af310d90c3c18adeab5"),
		AppRegistry:      getEnvOrDefault("CONTRACT_APPREGISTRY_HASH", "0x79d16bee03122e992bb80c478ad4ed405f33bc7f"),
		Governance:       getEnvOrDefault("CONTRACT_GOVERNANCE_HASH", "0xc8f3bbe1c205c932aab00b28f7df99f9bc788a05"),
	}

	return &Invoker{
		client:    client,
		actor:     act,
		addresses: addresses,
	}, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(hashStr, "0x")
	return util.Uint160DecodeStringLE(hashStr)
}

// UpdatePriceFeed updates a price feed with new data.
// Returns the transaction hash on success.
func (inv *Invoker) UpdatePriceFeed(ctx context.Context, symbol string, roundID int64, price int64, timestamp uint64) (string, error) {
	contractHash, err := parseContractHash(inv.addresses.PriceFeed)
	if err != nil {
		return "", fmt.Errorf("parse contract hash: %w", err)
	}

	// Generate attestation hash (32 bytes)
	attestationHash := make([]byte, 32)
	rand.Read(attestationHash)

	sourceSetID := int64(1) // Default source set

	// Call Update(symbol, roundId, price, timestamp, attestationHash, sourceSetId)
	txHash, _, err := inv.actor.SendCall(
		contractHash,
		"update",
		symbol,
		roundID,
		price,
		timestamp,
		attestationHash,
		sourceSetID,
	)
	if err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}

	return txHash.StringLE(), nil
}

// RecordRandomness records a randomness value on-chain.
// Returns the transaction hash on success.
func (inv *Invoker) RecordRandomness(ctx context.Context, requestID string) (string, error) {
	contractHash, err := parseContractHash(inv.addresses.RandomnessLog)
	if err != nil {
		return "", fmt.Errorf("parse contract hash: %w", err)
	}

	// Generate random bytes (32 bytes)
	randomness := make([]byte, 32)
	rand.Read(randomness)

	// Generate attestation hash (32 bytes)
	attestationHash := make([]byte, 32)
	rand.Read(attestationHash)

	timestamp := uint64(time.Now().Unix())

	// Call Record(requestId, randomness, attestationHash, timestamp)
	txHash, _, err := inv.actor.SendCall(
		contractHash,
		"record",
		requestID,
		randomness,
		attestationHash,
		timestamp,
	)
	if err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}

	return txHash.StringLE(), nil
}

// PayToApp makes a payment to a MiniApp via PaymentHub.
// Returns the transaction hash on success.
func (inv *Invoker) PayToApp(ctx context.Context, appID string, amount int64, memo string) (string, error) {
	contractHash, err := parseContractHash(inv.addresses.PaymentHub)
	if err != nil {
		return "", fmt.Errorf("parse contract hash: %w", err)
	}

	// Call Pay(appId, amount, memo)
	txHash, _, err := inv.actor.SendCall(
		contractHash,
		"pay",
		appID,
		amount,
		memo,
	)
	if err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}

	return txHash.StringLE(), nil
}

// GetPriceFeedLatest gets the latest price for a symbol.
func (inv *Invoker) GetPriceFeedLatest(ctx context.Context, symbol string) (map[string]interface{}, error) {
	contractHash, err := parseContractHash(inv.addresses.PriceFeed)
	if err != nil {
		return nil, fmt.Errorf("parse contract hash: %w", err)
	}

	result, err := inv.actor.Call(contractHash, "getLatest", symbol)
	if err != nil {
		return nil, fmt.Errorf("call getLatest: %w", err)
	}

	if result.State != "HALT" {
		return nil, fmt.Errorf("call failed: %s", result.FaultException)
	}

	// Parse result
	data := make(map[string]interface{})
	if len(result.Stack) > 0 {
		data["raw"] = result.Stack[0].Value()
	}

	return data, nil
}

// GenerateRequestID generates a unique request ID for randomness.
func GenerateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GeneratePrice generates a simulated price with some variance.
func GeneratePrice(basePrice int64, variancePercent int) int64 {
	variance := basePrice * int64(variancePercent) / 100
	n, _ := rand.Int(rand.Reader, big.NewInt(variance*2))
	return basePrice - variance + n.Int64()
}

// Close closes the invoker's connections.
func (inv *Invoker) Close() {
	// RPC client doesn't need explicit close in neo-go
}
