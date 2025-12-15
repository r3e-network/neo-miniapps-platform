package chain

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// =============================================================================
// Transaction Builder - Proper Neo N3 Transaction Construction
// =============================================================================

// TxBuilder builds and signs Neo N3 transactions.
type TxBuilder struct {
	client   *Client
	netMagic netmode.Magic
	extraFee int64  // Additional network fee buffer (in GAS fractions)
	blockBuf uint32 // ValidUntilBlock buffer (blocks ahead of current)
}

// NewTxBuilder creates a new transaction builder.
func NewTxBuilder(client *Client, networkID uint32) *TxBuilder {
	// Map network ID to netmode.Magic
	var magic netmode.Magic
	switch networkID {
	case 860833102:
		magic = netmode.MainNet
	case 894710606:
		magic = netmode.TestNet
	default:
		// Private network - use the network ID directly
		magic = netmode.Magic(networkID)
	}

	return &TxBuilder{
		client:   client,
		netMagic: magic,
		extraFee: 100000, // 0.001 GAS extra buffer
		blockBuf: 100,    // Valid for ~100 blocks (~25 minutes)
	}
}

// BuildAndSignTx builds a transaction from an invoke simulation and signs it.
// Parameters:
//   - ctx: context for RPC calls
//   - invokeResult: result from InvokeFunction simulation
//   - account: neo-go wallet account for signing
//   - signerScopes: witness scope for the signer
func (b *TxBuilder) BuildAndSignTx(
	ctx context.Context,
	invokeResult *InvokeResult,
	account *wallet.Account,
	signerScopes transaction.WitnessScope,
) (*transaction.Transaction, error) {
	// 1. Decode script from simulation result
	script, err := base64.StdEncoding.DecodeString(invokeResult.Script)
	if err != nil {
		// Try hex decoding as fallback
		script, err = hex.DecodeString(invokeResult.Script)
		if err != nil {
			return nil, fmt.Errorf("decode script: %w", err)
		}
	}

	// 2. Parse system fee from simulation
	systemFee, err := parseGasValue(invokeResult.GasConsumed)
	if err != nil {
		return nil, fmt.Errorf("parse system fee: %w", err)
	}

	// 3. Get current block height for ValidUntilBlock
	blockCount, err := b.client.GetBlockCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("get block count: %w", err)
	}
	maxValidUntilBlock := uint64(^uint32(0) - b.blockBuf)
	if blockCount > maxValidUntilBlock {
		return nil, fmt.Errorf("block height %d overflows uint32", blockCount)
	}
	validUntilBlock := uint32(blockCount) + b.blockBuf // #nosec G115 -- range checked above

	// 4. Create transaction
	tx := transaction.New(script, systemFee)
	tx.ValidUntilBlock = validUntilBlock
	tx.Nonce = rand.Uint32()

	// 5. Set signer
	tx.Signers = []transaction.Signer{
		{
			Account: account.ScriptHash(),
			Scopes:  signerScopes,
		},
	}

	// 6. Initialize witness with verification script
	tx.Scripts = []transaction.Witness{
		{
			VerificationScript: account.GetVerificationScript(),
		},
	}

	// 7. Calculate network fee
	networkFee := b.calculateNetworkFee(ctx, tx)
	tx.NetworkFee = networkFee + b.extraFee

	// 8. Sign transaction
	if err := account.SignTx(b.netMagic, tx); err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	return tx, nil
}

// calculateNetworkFee calculates the network fee for a transaction.
// Uses the calculatenetworkfee RPC method.
func (b *TxBuilder) calculateNetworkFee(ctx context.Context, tx *transaction.Transaction) int64 {
	// Serialize transaction for RPC
	txBytes := tx.Bytes()
	txBase64 := base64.StdEncoding.EncodeToString(txBytes)

	result, err := b.client.Call(ctx, "calculatenetworkfee", []interface{}{txBase64})
	if err != nil {
		// Fallback: estimate based on transaction size
		return b.estimateNetworkFee(tx)
	}

	var feeResult struct {
		NetworkFee string `json:"networkfee"`
	}
	if unmarshalErr := json.Unmarshal(result, &feeResult); unmarshalErr != nil {
		return b.estimateNetworkFee(tx)
	}

	fee, err := strconv.ParseInt(feeResult.NetworkFee, 10, 64)
	if err != nil {
		return b.estimateNetworkFee(tx)
	}

	return fee
}

// estimateNetworkFee provides a fallback fee estimation.
func (b *TxBuilder) estimateNetworkFee(tx *transaction.Transaction) int64 {
	// Base fee + size-based fee + verification cost
	// This is a conservative estimate
	baseSize := len(tx.Bytes())
	return int64(baseSize)*1000 + 1000000 // 0.01 GAS base + size cost
}

// parseGasValue parses a GAS value string to int64 (in fractions).
func parseGasValue(gasStr string) (int64, error) {
	// GasConsumed is returned as a decimal string (e.g., "0.0123456")
	// or as an integer string (e.g., "1234560")

	// Try parsing as float first
	if f, err := strconv.ParseFloat(gasStr, 64); err == nil {
		// Convert to GAS fractions (1 GAS = 10^8 fractions)
		return int64(f * 100000000), nil
	}

	// Try parsing as integer
	return strconv.ParseInt(gasStr, 10, 64)
}

// =============================================================================
// Account Creation Helpers
// =============================================================================

// AccountFromPrivateKey creates a neo-go wallet account from a private key hex string.
func AccountFromPrivateKey(privateKeyHex string) (*wallet.Account, error) {
	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	privateKey, err := keys.NewPrivateKeyFromBytes(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("create private key: %w", err)
	}

	return wallet.NewAccountFromPrivateKey(privateKey), nil
}

// AccountFromWIF creates a neo-go wallet account from a WIF string.
func AccountFromWIF(wif string) (*wallet.Account, error) {
	return wallet.NewAccountFromWIF(wif)
}

// =============================================================================
// Script Hash Utilities
// =============================================================================

// ParseScriptHash parses a script hash from hex string (handles 0x prefix and endianness).
func ParseScriptHash(hashStr string) (util.Uint160, error) {
	// Remove 0x prefix if present
	if len(hashStr) >= 2 && hashStr[:2] == "0x" {
		hashStr = hashStr[2:]
	}

	// Neo uses little-endian for script hashes in RPC
	return util.Uint160DecodeStringLE(hashStr)
}

// =============================================================================
// Transaction Broadcast
// =============================================================================

// BroadcastTx broadcasts a signed transaction and returns the transaction hash.
func (b *TxBuilder) BroadcastTx(ctx context.Context, tx *transaction.Transaction) (util.Uint256, error) {
	txBytes := tx.Bytes()
	txBase64 := base64.StdEncoding.EncodeToString(txBytes)

	result, err := b.client.Call(ctx, "sendrawtransaction", []interface{}{txBase64})
	if err != nil {
		return util.Uint256{}, fmt.Errorf("broadcast transaction: %w", err)
	}

	// Neo RPC returns {"hash": "0x..."} on success
	var response struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		// Some nodes return just true/false
		var success bool
		if json.Unmarshal(result, &success) == nil && success {
			return tx.Hash(), nil
		}
		return util.Uint256{}, fmt.Errorf("parse broadcast response: %w", err)
	}

	if response.Hash == "" {
		// If hash is empty but no error, use computed hash
		return tx.Hash(), nil
	}

	return util.Uint256DecodeStringLE(response.Hash[2:]) // Remove 0x prefix
}

// BroadcastAndWait broadcasts a transaction and waits for its application log.
func (b *TxBuilder) BroadcastAndWait(
	ctx context.Context,
	tx *transaction.Transaction,
	pollInterval, timeout time.Duration,
) (*ApplicationLog, error) {
	txHash, err := b.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, err
	}

	if timeout <= 0 {
		timeout = DefaultTxWaitTimeout
	}
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}

	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return b.client.WaitForApplicationLog(wctx, "0x"+txHash.StringLE(), pollInterval)
}
