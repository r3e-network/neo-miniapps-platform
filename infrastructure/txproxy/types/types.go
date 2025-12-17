// Package types provides shared request/response types for the TxProxy service.
//
// It exists so other services can depend on TxProxy without importing the
// service implementation package (avoids layering and import-cycle issues).
package types

import (
	"context"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
)

// InvokeRequest requests an allowlisted on-chain contract invocation.
type InvokeRequest struct {
	RequestID string `json:"request_id"`
	// Intent optionally enables additional policy gates.
	//
	// Supported values:
	// - "payments": enforce PaymentHub-only methods (GAS settlement)
	// - "governance": enforce Governance-only methods (NEO governance)
	Intent       string                `json:"intent,omitempty"`
	ContractHash string                `json:"contract_hash"`
	Method       string                `json:"method"`
	Params       []chain.ContractParam `json:"params,omitempty"`
	Wait         bool                  `json:"wait,omitempty"`
}

// InvokeResponse is returned by the TxProxy service when an invocation was accepted.
type InvokeResponse struct {
	RequestID string `json:"request_id"`
	TxHash    string `json:"tx_hash,omitempty"`
	VMState   string `json:"vm_state,omitempty"`
	Exception string `json:"exception,omitempty"`
}

// Invoker is the minimal client interface required by services that delegate
// chain writes to TxProxy.
type Invoker interface {
	Invoke(context.Context, *InvokeRequest) (*InvokeResponse, error)
}
