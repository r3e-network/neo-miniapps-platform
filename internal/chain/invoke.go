package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// Contract Invocation Methods
// =============================================================================

// InvokeFunction invokes a contract function (read-only).
func (c *Client) InvokeFunction(ctx context.Context, scriptHash string, method string, params []ContractParam) (*InvokeResult, error) {
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
