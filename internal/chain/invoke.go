package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// =============================================================================
// Contract Invocation Methods
// =============================================================================

// InvokeFunction invokes a contract function (read-only).
func (c *Client) InvokeFunction(ctx context.Context, scriptHash, method string, params []ContractParam) (*InvokeResult, error) {
	args := []interface{}{scriptHash, method, params}
	result, err := c.Call(ctx, "invokefunction", args)
	if err != nil {
		return nil, err
	}

	var invokeResult InvokeResult
	if err := json.Unmarshal(result, &invokeResult); err != nil {
		return nil, err
	}
	return &invokeResult, nil
}

// InvokeScript invokes a script (read-only).
func (c *Client) InvokeScript(ctx context.Context, script string, signers []Signer) (*InvokeResult, error) {
	args := []interface{}{script}
	if len(signers) > 0 {
		args = append(args, signers)
	}

	result, err := c.Call(ctx, "invokescript", args)
	if err != nil {
		return nil, err
	}

	var invokeResult InvokeResult
	if err := json.Unmarshal(result, &invokeResult); err != nil {
		return nil, err
	}
	return &invokeResult, nil
}

// SendRawTransaction sends a signed transaction.
func (c *Client) SendRawTransaction(ctx context.Context, txHex string) (string, error) {
	result, err := c.Call(ctx, "sendrawtransaction", []interface{}{txHex})
	if err != nil {
		return "", err
	}

	var response struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", err
	}
	return response.Hash, nil
}

// WaitForApplicationLog polls for a transaction application log until it is available or context is done.
// A missing transaction is treated as transient and retried until the context deadline/timeout expires.
func (c *Client) WaitForApplicationLog(ctx context.Context, txHash string, pollInterval time.Duration) (*ApplicationLog, error) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			log, err := c.GetApplicationLog(ctx, txHash)
			if err != nil {
				if isNotFoundError(err) {
					continue
				}
				return nil, err
			}
			return log, nil
		}
	}
}

// DefaultTxWaitTimeout is the default timeout for waiting for transaction execution.
const DefaultTxWaitTimeout = 2 * time.Minute

// DefaultPollInterval is the default interval for polling transaction status.
const DefaultPollInterval = 2 * time.Second

// SendRawTransactionAndWait broadcasts a signed transaction and waits for its application log.
// If waitTimeout is 0, DefaultTxWaitTimeout (2 minutes) is used. pollInterval <=0 defaults to 2s.
func (c *Client) SendRawTransactionAndWait(ctx context.Context, txHex string, pollInterval, waitTimeout time.Duration) (*ApplicationLog, error) {
	txHash, err := c.SendRawTransaction(ctx, txHex)
	if err != nil {
		return nil, err
	}

	if waitTimeout <= 0 {
		waitTimeout = DefaultTxWaitTimeout
	}

	wctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	return c.WaitForApplicationLog(wctx, txHash, pollInterval)
}

// InvokeFunctionAndWait invokes a contract function and optionally waits for execution.
// Deprecated: This method only simulates the transaction and does NOT broadcast it.
// Use InvokeFunctionWithSignerAndWait for actual on-chain transactions.
// If wait is true, it waits for the transaction to be included in a block and returns the application log.
// If wait is false, it returns immediately after broadcasting with only the TxHash populated.
// Uses DefaultTxWaitTimeout (2 minutes) and DefaultPollInterval (2 seconds).
func (c *Client) InvokeFunctionAndWait(ctx context.Context, contractHash, method string, params []ContractParam, wait bool) (*TxResult, error) {
	invokeResult, err := c.InvokeFunction(ctx, contractHash, method, params)
	if err != nil {
		return nil, fmt.Errorf("invoke %s: %w", method, err)
	}

	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("%s failed: %s", method, invokeResult.Exception)
	}

	result := &TxResult{
		TxHash:  invokeResult.Tx,
		VMState: invokeResult.State,
	}

	if !wait {
		return result, nil
	}

	// Wait for transaction execution
	wctx, cancel := context.WithTimeout(ctx, DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := c.WaitForApplicationLog(wctx, invokeResult.Tx, DefaultPollInterval)
	if err != nil {
		return result, fmt.Errorf("wait for %s execution: %w", method, err)
	}

	result.AppLog = appLog

	// Update VMState from actual execution
	if len(appLog.Executions) > 0 {
		result.VMState = appLog.Executions[0].VMState
	}

	return result, nil
}

// =============================================================================
// Proper Transaction Building and Broadcasting
// =============================================================================

// InvokeFunctionWithSignerAndWait properly builds, signs, and broadcasts a transaction.
// This is the correct way to invoke contract functions that modify state.
// Parameters:
//   - ctx: context for RPC calls
//   - contractHash: target contract script hash (hex string with or without 0x prefix)
//   - method: contract method name
//   - params: contract parameters
//   - account: neo-go wallet account for signing
//   - signerScopes: witness scope for the signer (use transaction.CalledByEntry for most cases)
//   - wait: if true, waits for transaction confirmation
//
// Returns TxResult with transaction hash and application log (if wait=true).
func (c *Client) InvokeFunctionWithSignerAndWait(
	ctx context.Context,
	contractHash, method string,
	params []ContractParam,
	account *wallet.Account,
	signerScopes transaction.WitnessScope,
	wait bool,
) (*TxResult, error) {
	// 1. Simulate the invocation to get script and gas estimate
	invokeResult, err := c.InvokeFunctionWithSigners(ctx, contractHash, method, params, account.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("simulate %s: %w", method, err)
	}

	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("%s simulation failed: %s", method, invokeResult.Exception)
	}

	// 2. Build and sign the transaction
	txBuilder := NewTxBuilder(c, c.networkID)
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, account, signerScopes)
	if err != nil {
		return nil, fmt.Errorf("build transaction for %s: %w", method, err)
	}

	// 3. Broadcast the transaction
	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast %s: %w", method, err)
	}

	result := &TxResult{
		TxHash:  "0x" + txHash.StringLE(),
		VMState: invokeResult.State,
	}

	if !wait {
		return result, nil
	}

	// 4. Wait for transaction confirmation
	wctx, cancel := context.WithTimeout(ctx, DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := c.WaitForApplicationLog(wctx, result.TxHash, DefaultPollInterval)
	if err != nil {
		return result, fmt.Errorf("wait for %s execution: %w", method, err)
	}

	result.AppLog = appLog

	// Update VMState from actual execution
	if len(appLog.Executions) > 0 {
		result.VMState = appLog.Executions[0].VMState
	}

	return result, nil
}

// InvokeFunctionWithSigners simulates a contract invocation with signers.
// This is used to get accurate gas estimates before building the actual transaction.
func (c *Client) InvokeFunctionWithSigners(ctx context.Context, scriptHash, method string, params []ContractParam, signerHash interface{}) (*InvokeResult, error) {
	// Build signers array for the RPC call
	var signers []Signer
	switch v := signerHash.(type) {
	case string:
		signers = []Signer{{Account: v, Scopes: ScopeCalledByEntry}}
	default:
		// Assume it's a util.Uint160 or similar
		signers = []Signer{{Account: fmt.Sprintf("0x%s", v), Scopes: ScopeCalledByEntry}}
	}

	args := []interface{}{scriptHash, method, params, signers}
	result, err := c.Call(ctx, "invokefunction", args)
	if err != nil {
		return nil, err
	}

	var invokeResult InvokeResult
	if err := json.Unmarshal(result, &invokeResult); err != nil {
		return nil, err
	}
	return &invokeResult, nil
}

// =============================================================================
// Invoke Helpers (Read-Only)
// =============================================================================

func invokeFirstStackItem(ctx context.Context, client *Client, scriptHash, method string, params ...ContractParam) (StackItem, error) {
	result, err := client.InvokeFunction(ctx, scriptHash, method, params)
	if err != nil {
		return StackItem{}, fmt.Errorf("%s: invoke failed: %w", method, err)
	}
	if err := requireHalt(method, result); err != nil {
		return StackItem{}, err
	}
	return firstStackItem(method, result)
}

// InvokeBool invokes a method and parses the first stack item as a bool.
func InvokeBool(ctx context.Context, client *Client, scriptHash, method string, params ...ContractParam) (bool, error) {
	item, err := invokeFirstStackItem(ctx, client, scriptHash, method, params...)
	if err != nil {
		return false, err
	}
	value, err := ParseBoolean(item)
	if err != nil {
		return false, fmt.Errorf("%s: parse result: %w", method, err)
	}
	return value, nil
}

// InvokeInt invokes a method and parses the first stack item as an Integer.
func InvokeInt(ctx context.Context, client *Client, scriptHash, method string, params ...ContractParam) (*big.Int, error) {
	item, err := invokeFirstStackItem(ctx, client, scriptHash, method, params...)
	if err != nil {
		return nil, err
	}
	value, err := ParseInteger(item)
	if err != nil {
		return nil, fmt.Errorf("%s: parse result: %w", method, err)
	}
	return value, nil
}

// InvokeStruct invokes a method and parses the first stack item using the provided parser.
func InvokeStruct[T any](ctx context.Context, client *Client, scriptHash, method string, parser func(StackItem) (T, error), params ...ContractParam) (T, error) {
	var zero T
	item, err := invokeFirstStackItem(ctx, client, scriptHash, method, params...)
	if err != nil {
		return zero, err
	}
	value, err := parser(item)
	if err != nil {
		return zero, fmt.Errorf("%s: parse result: %w", method, err)
	}
	return value, nil
}
