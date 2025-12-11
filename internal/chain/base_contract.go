// Package chain provides base contract wrapper for service-specific contracts.
package chain

import (
	"context"
	"fmt"
	"math/big"
)

// =============================================================================
// Base Contract Wrapper
// =============================================================================

// BaseContract provides common contract interaction patterns.
// Service-specific contracts embed this to reduce boilerplate.
//
// Usage:
//
//	type MyServiceContract struct {
//	    *chain.BaseContract
//	}
//
//	func NewMyServiceContract(client *Client, hash string, wallet *Wallet) *MyServiceContract {
//	    return &MyServiceContract{
//	        BaseContract: chain.NewBaseContract(client, hash, wallet),
//	    }
//	}
//
//	func (c *MyServiceContract) GetValue(ctx context.Context, key string) (*big.Int, error) {
//	    return c.InvokeInteger(ctx, "getValue", chain.NewStringParam(key))
//	}
type BaseContract struct {
	client       *Client
	contractHash string
	wallet       *Wallet
}

// NewBaseContract creates a new base contract wrapper.
func NewBaseContract(client *Client, contractHash string, wallet *Wallet) *BaseContract {
	return &BaseContract{
		client:       client,
		contractHash: contractHash,
		wallet:       wallet,
	}
}

// Client returns the chain client.
func (b *BaseContract) Client() *Client {
	return b.client
}

// ContractHash returns the contract hash.
func (b *BaseContract) ContractHash() string {
	return b.contractHash
}

// Wallet returns the wallet for signing.
func (b *BaseContract) Wallet() *Wallet {
	return b.wallet
}

// =============================================================================
// Safe Invocation Methods
// =============================================================================

// InvokeRaw invokes a contract method and returns the raw result.
// Use this when you need custom result parsing.
func (b *BaseContract) InvokeRaw(ctx context.Context, method string, params ...ContractParam) (*InvokeResult, error) {
	result, err := b.client.InvokeFunction(ctx, b.contractHash, method, params)
	if err != nil {
		return nil, fmt.Errorf("%s: invoke failed: %w", method, err)
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("%s: execution failed: %s", method, result.Exception)
	}
	return result, nil
}

// InvokeInteger invokes a method and returns the result as *big.Int.
func (b *BaseContract) InvokeInteger(ctx context.Context, method string, params ...ContractParam) (*big.Int, error) {
	result, err := b.InvokeRaw(ctx, method, params...)
	if err != nil {
		return nil, err
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("%s: no result", method)
	}
	return ParseInteger(result.Stack[0])
}

// InvokeBoolean invokes a method and returns the result as bool.
func (b *BaseContract) InvokeBoolean(ctx context.Context, method string, params ...ContractParam) (bool, error) {
	result, err := b.InvokeRaw(ctx, method, params...)
	if err != nil {
		return false, err
	}
	if len(result.Stack) == 0 {
		return false, fmt.Errorf("%s: no result", method)
	}
	return ParseBoolean(result.Stack[0])
}

// InvokeString invokes a method and returns the result as string.
func (b *BaseContract) InvokeString(ctx context.Context, method string, params ...ContractParam) (string, error) {
	result, err := b.InvokeRaw(ctx, method, params...)
	if err != nil {
		return "", err
	}
	if len(result.Stack) == 0 {
		return "", fmt.Errorf("%s: no result", method)
	}
	return ParseString(result.Stack[0])
}

// InvokeByteArray invokes a method and returns the result as []byte.
func (b *BaseContract) InvokeByteArray(ctx context.Context, method string, params ...ContractParam) ([]byte, error) {
	result, err := b.InvokeRaw(ctx, method, params...)
	if err != nil {
		return nil, err
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("%s: no result", method)
	}
	return ParseByteArray(result.Stack[0])
}

// InvokeUint64 invokes a method and returns the result as uint64.
func (b *BaseContract) InvokeUint64(ctx context.Context, method string, params ...ContractParam) (uint64, error) {
	result, err := b.InvokeInteger(ctx, method, params...)
	if err != nil {
		return 0, err
	}
	return result.Uint64(), nil
}

// InvokeVoid invokes a method that doesn't return a value.
// Useful for state-changing methods where we only care about success/failure.
func (b *BaseContract) InvokeVoid(ctx context.Context, method string, params ...ContractParam) error {
	_, err := b.InvokeRaw(ctx, method, params...)
	return err
}

// =============================================================================
// Custom Result Parsing
// =============================================================================

// InvokeAndParse invokes a method and parses the result using a custom parser.
// Use this when the result needs complex parsing (e.g., struct types).
//
// Example:
//
//	func (c *MyContract) GetConfig(ctx context.Context, id string) (*Config, error) {
//	    return chain.InvokeAndParse(c.BaseContract, ctx, "getConfig",
//	        func(item StackItem) (*Config, error) {
//	            return parseConfig(item)
//	        },
//	        chain.NewStringParam(id))
//	}
func InvokeAndParse[T any](b *BaseContract, ctx context.Context, method string, parser func(StackItem) (T, error), params ...ContractParam) (T, error) {
	var zero T
	result, err := b.InvokeRaw(ctx, method, params...)
	if err != nil {
		return zero, err
	}
	if len(result.Stack) == 0 {
		return zero, fmt.Errorf("%s: no result", method)
	}
	return parser(result.Stack[0])
}

// =============================================================================
// Array Result Parsing
// =============================================================================

// InvokeArray invokes a method and parses the result as an array.
// Each element is parsed using the provided parser function.
func InvokeArray[T any](b *BaseContract, ctx context.Context, method string, parser func(StackItem) (T, error), params ...ContractParam) ([]T, error) {
	result, err := b.InvokeRaw(ctx, method, params...)
	if err != nil {
		return nil, err
	}
	if len(result.Stack) == 0 {
		return nil, fmt.Errorf("%s: no result", method)
	}

	items, err := ParseArray(result.Stack[0])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}

	results := make([]T, len(items))
	for i, item := range items {
		parsed, err := parser(item)
		if err != nil {
			return nil, fmt.Errorf("%s: parse item %d: %w", method, i, err)
		}
		results[i] = parsed
	}
	return results, nil
}
