package chain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
)

// AutomationTask mirrors the platform AutomationAnchor.Task struct layout.
// Fields are returned by the contract in this order:
// (task_id, target, method, trigger, gas_limit, enabled).
type AutomationTask struct {
	TaskID   []byte
	Target   string
	Method   string
	Trigger  []byte
	GasLimit *big.Int
	Enabled  bool
}

// AutomationAnchorContract is a minimal wrapper for the platform AutomationAnchor contract.
// It registers tasks (admin) and records executions (updater) with nonce-based anti-replay.
type AutomationAnchorContract struct {
	client *Client
	hash   string
}

func NewAutomationAnchorContract(client *Client, hash string) *AutomationAnchorContract {
	return &AutomationAnchorContract{
		client: client,
		hash:   hash,
	}
}

func (c *AutomationAnchorContract) Hash() string {
	if c == nil {
		return ""
	}
	return c.hash
}

// GetTask returns the current task definition for taskID.
func (c *AutomationAnchorContract) GetTask(ctx context.Context, taskID []byte) (*AutomationTask, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("automationanchor: client not configured")
	}
	if c.hash == "" {
		return nil, fmt.Errorf("automationanchor: contract hash not configured")
	}
	if len(taskID) == 0 {
		return nil, fmt.Errorf("automationanchor: taskID required")
	}

	res, err := c.client.InvokeFunction(ctx, c.hash, "getTask", []ContractParam{NewByteArrayParam(taskID)})
	if err != nil {
		return nil, err
	}
	if res == nil || len(res.Stack) == 0 {
		return nil, fmt.Errorf("automationanchor: empty stack")
	}

	items, err := ParseArray(res.Stack[0])
	if err != nil {
		return nil, err
	}
	if len(items) < 6 {
		return nil, fmt.Errorf("automationanchor: expected 6 fields, got %d", len(items))
	}

	decodedTaskID, err := ParseByteArray(items[0])
	if err != nil {
		return nil, fmt.Errorf("automationanchor: parse taskId: %w", err)
	}

	target := ""
	switch items[1].Type {
	case "Null":
		// ok (task not found / not set)
	default:
		parsed, parseErr := ParseHash160(items[1])
		if parseErr != nil {
			return nil, fmt.Errorf("automationanchor: parse target: %w", parseErr)
		}
		target = parsed
	}

	method, err := ParseStringFromItem(items[2])
	if err != nil {
		return nil, fmt.Errorf("automationanchor: parse method: %w", err)
	}

	trigger, err := ParseByteArray(items[3])
	if err != nil {
		return nil, fmt.Errorf("automationanchor: parse trigger: %w", err)
	}

	gasLimit, err := ParseInteger(items[4])
	if err != nil {
		return nil, fmt.Errorf("automationanchor: parse gasLimit: %w", err)
	}

	enabled, err := ParseBoolean(items[5])
	if err != nil {
		return nil, fmt.Errorf("automationanchor: parse enabled: %w", err)
	}

	return &AutomationTask{
		TaskID:   decodedTaskID,
		Target:   target,
		Method:   method,
		Trigger:  trigger,
		GasLimit: gasLimit,
		Enabled:  enabled,
	}, nil
}

// IsNonceUsed checks whether a nonce has already been used for a task.
func (c *AutomationAnchorContract) IsNonceUsed(ctx context.Context, taskID []byte, nonce *big.Int) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("automationanchor: client not configured")
	}
	if c.hash == "" {
		return false, fmt.Errorf("automationanchor: contract hash not configured")
	}
	if len(taskID) == 0 {
		return false, fmt.Errorf("automationanchor: taskID required")
	}
	if nonce == nil || nonce.Sign() < 0 {
		return false, fmt.Errorf("automationanchor: invalid nonce")
	}

	res, err := c.client.InvokeFunction(ctx, c.hash, "isNonceUsed", []ContractParam{
		NewByteArrayParam(taskID),
		NewIntegerParam(nonce),
	})
	if err != nil {
		return false, err
	}
	if res == nil || len(res.Stack) == 0 {
		return false, fmt.Errorf("automationanchor: empty stack")
	}

	return ParseBoolean(res.Stack[0])
}

// MarkExecuted records an execution for a task and marks the nonce as used (updater-only).
func (c *AutomationAnchorContract) MarkExecuted(
	ctx context.Context,
	signer TxSigner,
	taskID []byte,
	nonce *big.Int,
	txHash []byte,
	wait bool,
) (*TxResult, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("automationanchor: client not configured")
	}
	if c.hash == "" {
		return nil, fmt.Errorf("automationanchor: contract hash not configured")
	}
	if signer == nil {
		return nil, fmt.Errorf("automationanchor: signer not configured")
	}
	if len(taskID) == 0 {
		return nil, fmt.Errorf("automationanchor: taskID required")
	}
	if nonce == nil || nonce.Sign() < 0 {
		return nil, fmt.Errorf("automationanchor: invalid nonce")
	}
	if len(txHash) == 0 {
		return nil, fmt.Errorf("automationanchor: txHash required")
	}

	params := []ContractParam{
		NewByteArrayParam(taskID),
		NewIntegerParam(nonce),
		NewByteArrayParam(txHash),
	}

	return c.client.InvokeFunctionWithSignerAndWait(
		ctx,
		c.hash,
		"markExecuted",
		params,
		signer,
		transaction.CalledByEntry,
		wait,
	)
}

// SetUpdater sets the updater address (admin-only).
func (c *AutomationAnchorContract) SetUpdater(ctx context.Context, signer TxSigner, updater string, wait bool) (*TxResult, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("automationanchor: client not configured")
	}
	if c.hash == "" {
		return nil, fmt.Errorf("automationanchor: contract hash not configured")
	}
	if signer == nil {
		return nil, fmt.Errorf("automationanchor: signer not configured")
	}
	if updater == "" {
		return nil, fmt.Errorf("automationanchor: updater required")
	}

	return c.client.InvokeFunctionWithSignerAndWait(
		ctx,
		c.hash,
		"setUpdater",
		[]ContractParam{NewHash160Param(updater)},
		signer,
		transaction.CalledByEntry,
		wait,
	)
}
