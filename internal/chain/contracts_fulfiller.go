package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/wallet"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// TEE Fulfillment - Transaction Builder for Callbacks
// =============================================================================

// TEEFulfiller handles TEE callback transactions to the Gateway contract.
// It properly builds, signs, and broadcasts transactions to the Neo N3 blockchain.
type TEEFulfiller struct {
	client       *Client
	gatewayHash  string
	account      *wallet.Account // neo-go wallet account for signing
	legacyWallet *Wallet         // Legacy wallet for message signing (not tx signing)
	nonceCounter *big.Int
	nonceMu      sync.Mutex // Protects nonceCounter for concurrent access
}

// NewTEEFulfiller creates a new TEE fulfiller.
// The privateKeyHex should be a hex-encoded private key (without 0x prefix).
func NewTEEFulfiller(client *Client, gatewayHash, privateKeyHex string) (*TEEFulfiller, error) {
	// Create neo-go account for transaction signing
	account, err := AccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	// Create legacy wallet for message signing (used in contract verification)
	legacyWallet, err := NewWallet(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("create legacy wallet: %w", err)
	}

	return &TEEFulfiller{
		client:       client,
		gatewayHash:  gatewayHash,
		account:      account,
		legacyWallet: legacyWallet,
		nonceCounter: big.NewInt(0),
	}, nil
}

// NewTEEFulfillerFromWallet creates a TEE fulfiller from an existing Wallet.
// Deprecated: Use NewTEEFulfiller with private key hex instead.
func NewTEEFulfillerFromWallet(client *Client, gatewayHash string, w *Wallet) (*TEEFulfiller, error) {
	// Extract private key from wallet and create account
	// Note: This is a compatibility shim - the wallet's private key bytes are needed
	privateKeyHex := hex.EncodeToString(w.privateKey.D.Bytes())
	account, err := AccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	return &TEEFulfiller{
		client:       client,
		gatewayHash:  gatewayHash,
		account:      account,
		legacyWallet: w,
		nonceCounter: big.NewInt(0),
	}, nil
}

// FulfillRequest fulfills a service request via the Gateway contract.
// This is called by TEE after processing a request.
// Returns the transaction hash after waiting for execution (2 minute timeout).
func (t *TEEFulfiller) FulfillRequest(ctx context.Context, requestID *big.Int, result []byte) (string, error) {
	nonce := t.nextNonce()

	// Use little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts
	message := append(bigIntToLittleEndian(requestID), result...)
	message = append(message, bigIntToLittleEndian(nonce)...)

	signature, err := t.legacyWallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign fulfillment: %w", err)
	}

	params := []ContractParam{
		NewIntegerParam(requestID),
		NewByteArrayParam(result),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	// Use proper transaction building and broadcasting
	txResult, err := t.client.InvokeFunctionWithSignerAndWait(
		ctx,
		t.gatewayHash,
		"fulfillRequest",
		params,
		t.account,
		transaction.CalledByEntry,
		true,
	)
	if err != nil {
		return "", err
	}

	return txResult.TxHash, nil
}

// FailRequest marks a request as failed via the Gateway contract.
// Returns the transaction hash after waiting for execution (2 minute timeout).
func (t *TEEFulfiller) FailRequest(ctx context.Context, requestID *big.Int, reason string) (string, error) {
	nonce := t.nextNonce()

	// Use little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts
	message := append(bigIntToLittleEndian(requestID), []byte(reason)...)
	message = append(message, bigIntToLittleEndian(nonce)...)

	signature, err := t.legacyWallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign failure: %w", err)
	}

	params := []ContractParam{
		NewIntegerParam(requestID),
		NewStringParam(reason),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	txResult, err := t.client.InvokeFunctionWithSignerAndWait(
		ctx,
		t.gatewayHash,
		"failRequest",
		params,
		t.account,
		transaction.CalledByEntry,
		true,
	)
	if err != nil {
		return "", err
	}

	return txResult.TxHash, nil
}

// ResolveDispute submits completion proof to resolve a neovault dispute on-chain.
// This is called ONLY when a user disputes a mix request.
// Normal flow has ZERO on-chain transactions.
// Returns the transaction hash after waiting for execution (2 minute timeout).
func (t *TEEFulfiller) ResolveDispute(ctx context.Context, neovaultHash string, requestHash, outputsHash []byte) (string, error) {
	nonce := t.nextNonce()

	// Use little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts
	nonceLE := bigIntToLittleEndian(nonce)
	message := make([]byte, 0, len(requestHash)+len(outputsHash)+len(nonceLE))
	message = append(message, requestHash...)
	message = append(message, outputsHash...)
	message = append(message, nonceLE...)

	signature, err := t.legacyWallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign dispute resolution: %w", err)
	}

	params := []ContractParam{
		NewByteArrayParam(requestHash),
		NewByteArrayParam(outputsHash),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	txResult, err := t.client.InvokeFunctionWithSignerAndWait(
		ctx,
		neovaultHash,
		"resolveDispute",
		params,
		t.account,
		transaction.CalledByEntry,
		true,
	)
	if err != nil {
		return "", err
	}

	return txResult.TxHash, nil
}

func (t *TEEFulfiller) nextNonce() *big.Int {
	t.nonceMu.Lock()
	defer t.nonceMu.Unlock()
	t.nonceCounter.Add(t.nonceCounter, big.NewInt(1))
	return new(big.Int).Set(t.nonceCounter)
}

// =============================================================================
// TEE Fulfiller Extensions for NeoFeeds and NeoFlow
// =============================================================================

// UpdatePrice updates a price feed on-chain (NeoFeeds push pattern).
// Returns the transaction hash after waiting for execution (2 minute timeout).
func (t *TEEFulfiller) UpdatePrice(ctx context.Context, neoFeedsHash, feedID string, price *big.Int, timestamp uint64) (string, error) {
	nonce := t.nextNonce()

	// Use little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts
	message := append([]byte(feedID), bigIntToLittleEndian(price)...)
	ts, err := safeInt64FromUint64(timestamp)
	if err != nil {
		return "", err
	}
	message = append(message, bigIntToLittleEndian(big.NewInt(ts))...)
	message = append(message, bigIntToLittleEndian(nonce)...)

	signature, err := t.legacyWallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign price update: %w", err)
	}

	params := []ContractParam{
		NewStringParam(feedID),
		NewIntegerParam(price),
		NewIntegerParam(big.NewInt(ts)),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	txResult, err := t.client.InvokeFunctionWithSignerAndWait(
		ctx,
		neoFeedsHash,
		"updatePrice",
		params,
		t.account,
		transaction.CalledByEntry,
		true,
	)
	if err != nil {
		return "", err
	}

	return txResult.TxHash, nil
}

// UpdatePrices batch updates multiple price feeds (NeoFeeds push pattern).
// Returns the transaction hash after waiting for execution (2 minute timeout).
func (t *TEEFulfiller) UpdatePrices(ctx context.Context, neoFeedsHash string, feedIDs []string, prices []*big.Int, timestamps []uint64) (string, error) {
	if len(feedIDs) != len(prices) || len(feedIDs) != len(timestamps) {
		return "", fmt.Errorf("array length mismatch")
	}

	nonce := t.nextNonce()

	// Use little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts
	var message []byte
	for i := range feedIDs {
		message = append(message, []byte(feedIDs[i])...)
		message = append(message, bigIntToLittleEndian(prices[i])...)
		ts, err := safeInt64FromUint64(timestamps[i])
		if err != nil {
			return "", err
		}
		message = append(message, bigIntToLittleEndian(big.NewInt(ts))...)
	}
	message = append(message, bigIntToLittleEndian(nonce)...)

	signature, err := t.legacyWallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign batch price update: %w", err)
	}

	feedIDParams := make([]ContractParam, len(feedIDs))
	priceParams := make([]ContractParam, len(prices))
	timestampParams := make([]ContractParam, len(timestamps))

	for i := range feedIDs {
		feedIDParams[i] = NewStringParam(feedIDs[i])
		priceParams[i] = NewIntegerParam(prices[i])
		ts, tsErr := safeInt64FromUint64(timestamps[i])
		if tsErr != nil {
			return "", tsErr
		}
		timestampParams[i] = NewIntegerParam(big.NewInt(ts))
	}

	params := []ContractParam{
		NewArrayParam(feedIDParams),
		NewArrayParam(priceParams),
		NewArrayParam(timestampParams),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	txResult, err := t.client.InvokeFunctionWithSignerAndWait(
		ctx,
		neoFeedsHash,
		"updatePrices",
		params,
		t.account,
		transaction.CalledByEntry,
		true,
	)
	if err != nil {
		return "", err
	}

	return txResult.TxHash, nil
}

// ExecuteTrigger executes an neoflow trigger (NeoFlow trigger pattern).
// Returns the transaction hash after waiting for execution (2 minute timeout).
func (t *TEEFulfiller) ExecuteTrigger(ctx context.Context, neoflowHash string, triggerID *big.Int, executionData []byte) (string, error) {
	nonce := t.nextNonce()

	// Use little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts
	message := append(bigIntToLittleEndian(triggerID), executionData...)
	message = append(message, bigIntToLittleEndian(nonce)...)

	signature, err := t.legacyWallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign trigger execution: %w", err)
	}

	params := []ContractParam{
		NewIntegerParam(triggerID),
		NewByteArrayParam(executionData),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	txResult, err := t.client.InvokeFunctionWithSignerAndWait(
		ctx,
		neoflowHash,
		"executeTrigger",
		params,
		t.account,
		transaction.CalledByEntry,
		true,
	)
	if err != nil {
		return "", err
	}

	return txResult.TxHash, nil
}

// =============================================================================
// TEE Master Key Anchoring
// =============================================================================

// SetTEEMasterKey anchors the TEE master key to the Gateway contract.
// This is called during initial setup to register the TEE's master public key.
// Returns the transaction result after waiting for execution.
func (t *TEEFulfiller) SetTEEMasterKey(ctx context.Context, pubKey, pubKeyHash, attestHash []byte) (*TxResult, error) {
	nonce := t.nextNonce()

	// Use little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts
	nonceLE := bigIntToLittleEndian(nonce)
	message := make([]byte, 0, len(pubKey)+len(pubKeyHash)+len(attestHash)+len(nonceLE))
	message = append(message, pubKey...)
	message = append(message, pubKeyHash...)
	message = append(message, attestHash...)
	message = append(message, nonceLE...)

	signature, err := t.legacyWallet.Sign(message)
	if err != nil {
		return nil, fmt.Errorf("sign master key: %w", err)
	}

	params := []ContractParam{
		NewByteArrayParam(pubKey),
		NewByteArrayParam(pubKeyHash),
		NewByteArrayParam(attestHash),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	txResult, err := t.client.InvokeFunctionWithSignerAndWait(
		ctx,
		t.gatewayHash,
		"setTEEMasterKey",
		params,
		t.account,
		transaction.CalledByEntry,
		true,
	)
	if err != nil {
		return nil, err
	}

	return txResult, nil
}

func safeInt64FromUint64(v uint64) (int64, error) {
	if v > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("value %d overflows int64", v)
	}
	return int64(v), nil
}

// =============================================================================
// Legacy FulfillmentSigner (for backward compatibility)
// =============================================================================

// FulfillmentSigner provides TEE signing for contract fulfillment.
type FulfillmentSigner struct {
	privateKey []byte
}

// NewFulfillmentSigner creates a new fulfillment signer.
func NewFulfillmentSigner(privateKey []byte) *FulfillmentSigner {
	return &FulfillmentSigner{privateKey: privateKey}
}

// SignFulfillment signs a fulfillment message (requestId + result + nonce).
// Uses little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts.
func (f *FulfillmentSigner) SignFulfillment(requestID *big.Int, result []byte, nonce *big.Int) ([]byte, error) {
	message := append(bigIntToLittleEndian(requestID), result...)
	message = append(message, bigIntToLittleEndian(nonce)...)
	return f.sign(message)
}

// SignFailure signs a failure message (requestId + reason + nonce).
// Uses little-endian encoding to match .NET BigInteger.ToByteArray() in Neo N3 contracts.
func (f *FulfillmentSigner) SignFailure(requestID *big.Int, reason string, nonce *big.Int) ([]byte, error) {
	message := append(bigIntToLittleEndian(requestID), []byte(reason)...)
	message = append(message, bigIntToLittleEndian(nonce)...)
	return f.sign(message)
}

func (f *FulfillmentSigner) sign(message []byte) ([]byte, error) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	keyPair.PrivateKey.D = new(big.Int).SetBytes(f.privateKey)
	keyPair.PrivateKey.PublicKey.X, keyPair.PrivateKey.PublicKey.Y =
		keyPair.PrivateKey.Curve.ScalarBaseMult(f.privateKey)

	return crypto.Sign(keyPair.PrivateKey, message)
}

// bigIntToLittleEndian converts a big.Int to little-endian byte array,
// matching .NET's BigInteger.ToByteArray() format used by Neo N3 contracts.
// This is critical for signature verification compatibility.
func bigIntToLittleEndian(n *big.Int) []byte {
	if n.Sign() == 0 {
		return []byte{0}
	}

	// Get big-endian bytes
	beBytes := n.Bytes()

	// Reverse to little-endian
	leBytes := make([]byte, len(beBytes))
	for i := 0; i < len(beBytes); i++ {
		leBytes[i] = beBytes[len(beBytes)-1-i]
	}

	// .NET BigInteger adds a 0x00 byte if the high bit is set (to indicate positive)
	// For positive numbers, if the high bit of the last byte (most significant in LE) is set,
	// we need to append 0x00
	if n.Sign() > 0 && leBytes[len(leBytes)-1]&0x80 != 0 {
		leBytes = append(leBytes, 0x00)
	}

	return leBytes
}
