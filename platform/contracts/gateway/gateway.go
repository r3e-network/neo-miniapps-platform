// Package gateway provides the ServiceLayer Gateway contract interface.
// The Gateway contract is the central entry point for all service requests.
// All user requests and service callbacks MUST go through the Gateway.
package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/contracts/types"
)

// =============================================================================
// Gateway Contract Interface
// =============================================================================

// Gateway represents the ServiceLayer Gateway contract.
// It acts as the single entry point for all service interactions.
type Gateway interface {
	// Request submits a service request through the gateway.
	// Returns the request ID.
	Request(ctx context.Context, req *ServiceRequestParams) (string, error)

	// Callback delivers a service response back to the user contract.
	// Only callable by the service layer (TEE-signed).
	Callback(ctx context.Context, resp *types.ServiceResponse) error

	// GetRequest retrieves a request by ID.
	GetRequest(ctx context.Context, requestID string) (*types.ServiceRequest, error)

	// GetPendingRequests retrieves all pending requests for a service.
	GetPendingRequests(ctx context.Context, serviceID types.ServiceID) ([]*types.ServiceRequest, error)

	// RegisterService registers a service contract with the gateway.
	// Only callable by admin.
	RegisterService(ctx context.Context, serviceID types.ServiceID, contractHash types.ScriptHash) error

	// GetServiceContract returns the contract address for a service.
	GetServiceContract(ctx context.Context, serviceID types.ServiceID) (types.ScriptHash, error)

	// DepositGas deposits GAS for a contract to use for service requests.
	DepositGas(ctx context.Context, contract types.ScriptHash, amount int64) error

	// WithdrawGas withdraws GAS from a contract's balance.
	WithdrawGas(ctx context.Context, contract types.ScriptHash, amount int64) error

	// GetGasBalance returns the GAS balance for a contract.
	GetGasBalance(ctx context.Context, contract types.ScriptHash) (int64, error)

	// SetServiceLayerAddress sets the authorized service layer address.
	// Only callable by admin.
	SetServiceLayerAddress(ctx context.Context, address types.ScriptHash) error

	// GetServiceLayerAddress returns the authorized service layer address.
	GetServiceLayerAddress(ctx context.Context) (types.ScriptHash, error)

	// ContractHash returns the gateway contract hash.
	ContractHash() types.ScriptHash
}

// ServiceRequestParams contains parameters for a service request.
type ServiceRequestParams struct {
	// ServiceID identifies which service to invoke
	ServiceID types.ServiceID `json:"service_id"`

	// CallbackContract is where to send the result
	CallbackContract types.ScriptHash `json:"callback_contract"`

	// CallbackMethod is the method to call with the result
	CallbackMethod string `json:"callback_method"`

	// Payload contains service-specific request data
	Payload []byte `json:"payload"`

	// GasLimit is the maximum GAS to use for this request
	GasLimit int64 `json:"gas_limit"`
}

// =============================================================================
// Gateway Events
// =============================================================================

// ServiceRequestEvent is emitted when a new service request is created.
type ServiceRequestEvent struct {
	RequestID        string            `json:"request_id"`
	ServiceID        types.ServiceID   `json:"service_id"`
	Requester        types.ScriptHash  `json:"requester"`
	CallbackContract types.ScriptHash  `json:"callback_contract"`
	CallbackMethod   string            `json:"callback_method"`
	Payload          []byte            `json:"payload"`
	GasDeposit       int64             `json:"gas_deposit"`
	BlockHeight      uint32            `json:"block_height"`
	TxHash           types.UInt256     `json:"tx_hash"`
}

// ServiceResponseEvent is emitted when a service response is delivered.
type ServiceResponseEvent struct {
	RequestID   string          `json:"request_id"`
	ServiceID   types.ServiceID `json:"service_id"`
	Success     bool            `json:"success"`
	GasUsed     int64           `json:"gas_used"`
	BlockHeight uint32          `json:"block_height"`
	TxHash      types.UInt256   `json:"tx_hash"`
}

// =============================================================================
// In-Memory Gateway Implementation (for testing/simulation)
// =============================================================================

// MemoryGateway is an in-memory implementation of the Gateway interface.
type MemoryGateway struct {
	mu sync.RWMutex

	contractHash        types.ScriptHash
	serviceLayerAddress types.ScriptHash
	services            map[types.ServiceID]types.ScriptHash
	requests            map[string]*types.ServiceRequest
	gasBalances         map[types.ScriptHash]int64
	requestCounter      uint64

	// Event handlers
	onRequest  func(*ServiceRequestEvent)
	onResponse func(*ServiceResponseEvent)
}

// NewMemoryGateway creates a new in-memory gateway.
func NewMemoryGateway(contractHash types.ScriptHash) *MemoryGateway {
	return &MemoryGateway{
		contractHash: contractHash,
		services:     make(map[types.ServiceID]types.ScriptHash),
		requests:     make(map[string]*types.ServiceRequest),
		gasBalances:  make(map[types.ScriptHash]int64),
	}
}

// SetEventHandlers sets the event handlers for the gateway.
func (g *MemoryGateway) SetEventHandlers(onRequest func(*ServiceRequestEvent), onResponse func(*ServiceResponseEvent)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onRequest = onRequest
	g.onResponse = onResponse
}

// Request implements Gateway.Request.
func (g *MemoryGateway) Request(ctx context.Context, req *ServiceRequestParams) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Validate service is registered
	if _, ok := g.services[req.ServiceID]; !ok {
		return "", fmt.Errorf("service not registered: %s", req.ServiceID)
	}

	// Check gas balance
	balance := g.gasBalances[req.CallbackContract]
	if balance < req.GasLimit {
		return "", fmt.Errorf("insufficient gas balance: have %d, need %d", balance, req.GasLimit)
	}

	// Generate request ID
	g.requestCounter++
	requestID := g.generateRequestID(req.CallbackContract, g.requestCounter)

	// Create request
	now := time.Now()
	request := &types.ServiceRequest{
		RequestID:        requestID,
		ServiceID:        req.ServiceID,
		Requester:        req.CallbackContract,
		CallbackContract: req.CallbackContract,
		CallbackMethod:   req.CallbackMethod,
		Payload:          req.Payload,
		GasDeposit:       req.GasLimit,
		Status:           types.RequestStatusPending,
		CreatedAt:        now,
	}

	// Reserve gas
	g.gasBalances[req.CallbackContract] -= req.GasLimit

	// Store request
	g.requests[requestID] = request

	// Emit event
	if g.onRequest != nil {
		g.onRequest(&ServiceRequestEvent{
			RequestID:        requestID,
			ServiceID:        req.ServiceID,
			Requester:        req.CallbackContract,
			CallbackContract: req.CallbackContract,
			CallbackMethod:   req.CallbackMethod,
			Payload:          req.Payload,
			GasDeposit:       req.GasLimit,
		})
	}

	return requestID, nil
}

// Callback implements Gateway.Callback.
func (g *MemoryGateway) Callback(ctx context.Context, resp *types.ServiceResponse) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Get request
	request, ok := g.requests[resp.RequestID]
	if !ok {
		return fmt.Errorf("request not found: %s", resp.RequestID)
	}

	// Verify request is pending
	if request.Status != types.RequestStatusPending {
		return fmt.Errorf("request not pending: %s", request.Status)
	}

	// Update request status
	now := time.Now()
	request.ProcessedAt = &now
	if resp.Success {
		request.Status = types.RequestStatusProcessed
	} else {
		request.Status = types.RequestStatusFailed
	}

	// Refund unused gas
	unusedGas := request.GasDeposit - resp.GasUsed
	if unusedGas > 0 {
		g.gasBalances[request.CallbackContract] += unusedGas
	}

	// Emit event
	if g.onResponse != nil {
		g.onResponse(&ServiceResponseEvent{
			RequestID: resp.RequestID,
			ServiceID: resp.ServiceID,
			Success:   resp.Success,
			GasUsed:   resp.GasUsed,
		})
	}

	return nil
}

// GetRequest implements Gateway.GetRequest.
func (g *MemoryGateway) GetRequest(ctx context.Context, requestID string) (*types.ServiceRequest, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	request, ok := g.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("request not found: %s", requestID)
	}
	return request, nil
}

// GetPendingRequests implements Gateway.GetPendingRequests.
func (g *MemoryGateway) GetPendingRequests(ctx context.Context, serviceID types.ServiceID) ([]*types.ServiceRequest, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var pending []*types.ServiceRequest
	for _, req := range g.requests {
		if req.ServiceID == serviceID && req.Status == types.RequestStatusPending {
			pending = append(pending, req)
		}
	}
	return pending, nil
}

// RegisterService implements Gateway.RegisterService.
func (g *MemoryGateway) RegisterService(ctx context.Context, serviceID types.ServiceID, contractHash types.ScriptHash) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.services[serviceID] = contractHash
	return nil
}

// GetServiceContract implements Gateway.GetServiceContract.
func (g *MemoryGateway) GetServiceContract(ctx context.Context, serviceID types.ServiceID) (types.ScriptHash, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	hash, ok := g.services[serviceID]
	if !ok {
		return types.ScriptHash{}, fmt.Errorf("service not registered: %s", serviceID)
	}
	return hash, nil
}

// DepositGas implements Gateway.DepositGas.
func (g *MemoryGateway) DepositGas(ctx context.Context, contract types.ScriptHash, amount int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if amount <= 0 {
		return fmt.Errorf("invalid amount: %d", amount)
	}

	g.gasBalances[contract] += amount
	return nil
}

// WithdrawGas implements Gateway.WithdrawGas.
func (g *MemoryGateway) WithdrawGas(ctx context.Context, contract types.ScriptHash, amount int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if amount <= 0 {
		return fmt.Errorf("invalid amount: %d", amount)
	}

	balance := g.gasBalances[contract]
	if balance < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", balance, amount)
	}

	g.gasBalances[contract] -= amount
	return nil
}

// GetGasBalance implements Gateway.GetGasBalance.
func (g *MemoryGateway) GetGasBalance(ctx context.Context, contract types.ScriptHash) (int64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.gasBalances[contract], nil
}

// SetServiceLayerAddress implements Gateway.SetServiceLayerAddress.
func (g *MemoryGateway) SetServiceLayerAddress(ctx context.Context, address types.ScriptHash) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.serviceLayerAddress = address
	return nil
}

// GetServiceLayerAddress implements Gateway.GetServiceLayerAddress.
func (g *MemoryGateway) GetServiceLayerAddress(ctx context.Context) (types.ScriptHash, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.serviceLayerAddress, nil
}

// ContractHash implements Gateway.ContractHash.
func (g *MemoryGateway) ContractHash() types.ScriptHash {
	return g.contractHash
}

// generateRequestID generates a unique request ID.
func (g *MemoryGateway) generateRequestID(requester types.ScriptHash, counter uint64) string {
	data := make([]byte, 28)
	copy(data[:20], requester[:])
	binary.BigEndian.PutUint64(data[20:], counter)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:16])
}
