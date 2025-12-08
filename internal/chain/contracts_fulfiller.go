package chain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// TEE Fulfillment - Transaction Builder for Callbacks
// =============================================================================

// TEEFulfiller handles TEE callback transactions to the Gateway contract.
type TEEFulfiller struct {
	client       *Client
	gatewayHash  string
	wallet       *Wallet
	nonceCounter *big.Int
}

// NewTEEFulfiller creates a new TEE fulfiller.
func NewTEEFulfiller(client *Client, gatewayHash string, wallet *Wallet) *TEEFulfiller {
	return &TEEFulfiller{
		client:       client,
		gatewayHash:  gatewayHash,
		wallet:       wallet,
		nonceCounter: big.NewInt(0),
	}
}

// FulfillRequest fulfills a service request via the Gateway contract.
// This is called by TEE after processing a request.
func (t *TEEFulfiller) FulfillRequest(ctx context.Context, requestID *big.Int, result []byte) (string, error) {
	nonce := t.nextNonce()

	message := append(requestID.Bytes(), result...)
	message = append(message, nonce.Bytes()...)

	signature, err := t.wallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign fulfillment: %w", err)
	}

	params := []ContractParam{
		NewIntegerParam(requestID),
		NewByteArrayParam(result),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	invokeResult, err := t.client.InvokeFunction(ctx, t.gatewayHash, "fulfillRequest", params)
	if err != nil {
		return "", fmt.Errorf("invoke fulfillRequest: %w", err)
	}

	if invokeResult.State != "HALT" {
		return "", fmt.Errorf("fulfillRequest failed: %s", invokeResult.Exception)
	}

	return invokeResult.Tx, nil
}

// FailRequest marks a request as failed via the Gateway contract.
func (t *TEEFulfiller) FailRequest(ctx context.Context, requestID *big.Int, reason string) (string, error) {
	nonce := t.nextNonce()

	message := append(requestID.Bytes(), []byte(reason)...)
	message = append(message, nonce.Bytes()...)

	signature, err := t.wallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign failure: %w", err)
	}

	params := []ContractParam{
		NewIntegerParam(requestID),
		NewStringParam(reason),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	invokeResult, err := t.client.InvokeFunction(ctx, t.gatewayHash, "failRequest", params)
	if err != nil {
		return "", fmt.Errorf("invoke failRequest: %w", err)
	}

	if invokeResult.State != "HALT" {
		return "", fmt.Errorf("failRequest failed: %s", invokeResult.Exception)
	}

	return invokeResult.Tx, nil
}

// UpdateMerkleRoot updates the Merkle root for a mixer pool.
func (t *TEEFulfiller) UpdateMerkleRoot(ctx context.Context, mixerHash string, poolID, newRoot []byte, leafCount *big.Int) (string, error) {
	params := []ContractParam{
		NewByteArrayParam(poolID),
		NewByteArrayParam(newRoot),
		NewIntegerParam(leafCount),
	}

	invokeResult, err := t.client.InvokeFunction(ctx, mixerHash, "updateMerkleRoot", params)
	if err != nil {
		return "", fmt.Errorf("invoke updateMerkleRoot: %w", err)
	}

	if invokeResult.State != "HALT" {
		return "", fmt.Errorf("updateMerkleRoot failed: %s", invokeResult.Exception)
	}

	return invokeResult.Tx, nil
}

func (t *TEEFulfiller) nextNonce() *big.Int {
	t.nonceCounter.Add(t.nonceCounter, big.NewInt(1))
	return new(big.Int).Set(t.nonceCounter)
}

// =============================================================================
// TEE Fulfiller Extensions for DataFeeds and Automation
// =============================================================================

// UpdatePrice updates a price feed on-chain (DataFeeds push pattern).
func (t *TEEFulfiller) UpdatePrice(ctx context.Context, dataFeedsHash, feedID string, price *big.Int, timestamp uint64) (string, error) {
	nonce := t.nextNonce()

	message := append([]byte(feedID), price.Bytes()...)
	message = append(message, big.NewInt(int64(timestamp)).Bytes()...)
	message = append(message, nonce.Bytes()...)

	signature, err := t.wallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign price update: %w", err)
	}

	params := []ContractParam{
		NewStringParam(feedID),
		NewIntegerParam(price),
		NewIntegerParam(big.NewInt(int64(timestamp))),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	invokeResult, err := t.client.InvokeFunction(ctx, dataFeedsHash, "updatePrice", params)
	if err != nil {
		return "", fmt.Errorf("invoke updatePrice: %w", err)
	}

	if invokeResult.State != "HALT" {
		return "", fmt.Errorf("updatePrice failed: %s", invokeResult.Exception)
	}

	return invokeResult.Tx, nil
}

// UpdatePrices batch updates multiple price feeds (DataFeeds push pattern).
func (t *TEEFulfiller) UpdatePrices(ctx context.Context, dataFeedsHash string, feedIDs []string, prices []*big.Int, timestamps []uint64) (string, error) {
	if len(feedIDs) != len(prices) || len(feedIDs) != len(timestamps) {
		return "", fmt.Errorf("array length mismatch")
	}

	nonce := t.nextNonce()

	var message []byte
	for i := range feedIDs {
		message = append(message, []byte(feedIDs[i])...)
		message = append(message, prices[i].Bytes()...)
		message = append(message, big.NewInt(int64(timestamps[i])).Bytes()...)
	}
	message = append(message, nonce.Bytes()...)

	signature, err := t.wallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign batch price update: %w", err)
	}

	feedIDParams := make([]ContractParam, len(feedIDs))
	priceParams := make([]ContractParam, len(prices))
	timestampParams := make([]ContractParam, len(timestamps))

	for i := range feedIDs {
		feedIDParams[i] = NewStringParam(feedIDs[i])
		priceParams[i] = NewIntegerParam(prices[i])
		timestampParams[i] = NewIntegerParam(big.NewInt(int64(timestamps[i])))
	}

	params := []ContractParam{
		NewArrayParam(feedIDParams),
		NewArrayParam(priceParams),
		NewArrayParam(timestampParams),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	invokeResult, err := t.client.InvokeFunction(ctx, dataFeedsHash, "updatePrices", params)
	if err != nil {
		return "", fmt.Errorf("invoke updatePrices: %w", err)
	}

	if invokeResult.State != "HALT" {
		return "", fmt.Errorf("updatePrices failed: %s", invokeResult.Exception)
	}

	return invokeResult.Tx, nil
}

// ExecuteTrigger executes an automation trigger (Automation trigger pattern).
func (t *TEEFulfiller) ExecuteTrigger(ctx context.Context, automationHash string, triggerID *big.Int, executionData []byte) (string, error) {
	nonce := t.nextNonce()

	message := append(triggerID.Bytes(), executionData...)
	message = append(message, nonce.Bytes()...)

	signature, err := t.wallet.Sign(message)
	if err != nil {
		return "", fmt.Errorf("sign trigger execution: %w", err)
	}

	params := []ContractParam{
		NewIntegerParam(triggerID),
		NewByteArrayParam(executionData),
		NewIntegerParam(nonce),
		NewByteArrayParam(signature),
	}

	invokeResult, err := t.client.InvokeFunction(ctx, automationHash, "executeTrigger", params)
	if err != nil {
		return "", fmt.Errorf("invoke executeTrigger: %w", err)
	}

	if invokeResult.State != "HALT" {
		return "", fmt.Errorf("executeTrigger failed: %s", invokeResult.Exception)
	}

	return invokeResult.Tx, nil
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
func (f *FulfillmentSigner) SignFulfillment(requestID *big.Int, result []byte, nonce *big.Int) ([]byte, error) {
	message := append(requestID.Bytes(), result...)
	message = append(message, nonce.Bytes()...)
	return f.sign(message)
}

// SignFailure signs a failure message (requestId + reason + nonce).
func (f *FulfillmentSigner) SignFailure(requestID *big.Int, reason string, nonce *big.Int) ([]byte, error) {
	message := append(requestID.Bytes(), []byte(reason)...)
	message = append(message, nonce.Bytes()...)
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
