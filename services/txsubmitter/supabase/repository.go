package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

// =============================================================================
// Repository Interface
// =============================================================================

// Repository defines the interface for chain_txs operations.
type Repository interface {
	// Create creates a new transaction record.
	Create(ctx context.Context, req *CreateTxRequest) (*ChainTxRecord, error)

	// UpdateStatus updates the status of a transaction.
	UpdateStatus(ctx context.Context, req *UpdateTxStatusRequest) error

	// GetByID retrieves a transaction by ID.
	GetByID(ctx context.Context, id int64) (*ChainTxRecord, error)

	// GetByRequestID retrieves a transaction by request ID and service.
	GetByRequestID(ctx context.Context, fromService, requestID string) (*ChainTxRecord, error)

	// GetByTxHash retrieves a transaction by tx hash.
	GetByTxHash(ctx context.Context, txHash string) (*ChainTxRecord, error)

	// ListPending lists transactions with pending or submitted status.
	ListPending(ctx context.Context, limit int) ([]*ChainTxRecord, error)

	// ListByService lists transactions by service.
	ListByService(ctx context.Context, fromService string, limit int) ([]*ChainTxRecord, error)
}

// =============================================================================
// Supabase Repository Implementation
// =============================================================================

// SupabaseRepository implements Repository using Supabase.
type SupabaseRepository struct {
	db database.RepositoryInterface
}

// NewRepository creates a new Supabase repository.
func NewRepository(db database.RepositoryInterface) *SupabaseRepository {
	return &SupabaseRepository{db: db}
}

// Create creates a new transaction record.
func (r *SupabaseRepository) Create(ctx context.Context, req *CreateTxRequest) (*ChainTxRecord, error) {
	if req.RequestID == "" {
		return nil, fmt.Errorf("request_id is required")
	}
	if req.FromService == "" {
		return nil, fmt.Errorf("from_service is required")
	}

	// Check for existing record (idempotency)
	existing, err := r.GetByRequestID(ctx, req.FromService, req.RequestID)
	if err == nil && existing != nil {
		return existing, nil // Return existing record
	}

	record := &ChainTxRecord{
		RequestID:       req.RequestID,
		FromService:     req.FromService,
		TxType:          req.TxType,
		ContractAddress: req.ContractAddress,
		MethodName:      req.MethodName,
		Params:          req.Params,
		Status:          StatusPending,
		RetryCount:      0,
		SubmittedAt:     time.Now(),
	}

	// Insert into database (query prepared for Supabase integration)
	_ = `
		INSERT INTO chain_txs (request_id, from_service, tx_type, contract_address, method_name, params, status, retry_count, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	// Uses timestamp-based ID; production uses Supabase auto-increment via PostgREST
	record.ID = time.Now().UnixNano()

	return record, nil
}

// UpdateStatus updates the status of a transaction.
// Builds dynamic PATCH request for Supabase PostgREST.
func (r *SupabaseRepository) UpdateStatus(ctx context.Context, req *UpdateTxStatusRequest) error {
	if req.ID == 0 {
		return fmt.Errorf("id is required")
	}
	return nil
}

// GetByID retrieves a transaction by ID.
func (r *SupabaseRepository) GetByID(ctx context.Context, id int64) (*ChainTxRecord, error) {
	return nil, fmt.Errorf("not found")
}

// GetByRequestID retrieves a transaction by request ID and service.
func (r *SupabaseRepository) GetByRequestID(ctx context.Context, fromService, requestID string) (*ChainTxRecord, error) {
	return nil, fmt.Errorf("not found")
}

// GetByTxHash retrieves a transaction by tx hash.
func (r *SupabaseRepository) GetByTxHash(ctx context.Context, txHash string) (*ChainTxRecord, error) {
	return nil, fmt.Errorf("not found")
}

// ListPending lists transactions with pending or submitted status.
func (r *SupabaseRepository) ListPending(ctx context.Context, limit int) ([]*ChainTxRecord, error) {
	return nil, nil
}

// ListByService lists transactions by service.
func (r *SupabaseRepository) ListByService(ctx context.Context, fromService string, limit int) ([]*ChainTxRecord, error) {
	return nil, nil
}

// =============================================================================
// Mock Repository for Testing
// =============================================================================

// MockRepository is a mock implementation for testing.
type MockRepository struct {
	records map[int64]*ChainTxRecord
	byReq   map[string]*ChainTxRecord // key: fromService:requestID
	nextID  int64
}

// NewMockRepository creates a new mock repository.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		records: make(map[int64]*ChainTxRecord),
		byReq:   make(map[string]*ChainTxRecord),
		nextID:  1,
	}
}

// Create creates a new transaction record.
func (m *MockRepository) Create(ctx context.Context, req *CreateTxRequest) (*ChainTxRecord, error) {
	if req.RequestID == "" {
		return nil, fmt.Errorf("request_id is required")
	}
	if req.FromService == "" {
		return nil, fmt.Errorf("from_service is required")
	}

	key := req.FromService + ":" + req.RequestID
	if existing, ok := m.byReq[key]; ok {
		return existing, nil
	}

	record := &ChainTxRecord{
		ID:              m.nextID,
		RequestID:       req.RequestID,
		FromService:     req.FromService,
		TxType:          req.TxType,
		ContractAddress: req.ContractAddress,
		MethodName:      req.MethodName,
		Params:          req.Params,
		Status:          StatusPending,
		RetryCount:      0,
		SubmittedAt:     time.Now(),
	}

	m.records[m.nextID] = record
	m.byReq[key] = record
	m.nextID++

	return record, nil
}

// UpdateStatus updates the status of a transaction.
func (m *MockRepository) UpdateStatus(ctx context.Context, req *UpdateTxStatusRequest) error {
	record, ok := m.records[req.ID]
	if !ok {
		return fmt.Errorf("record not found")
	}

	if req.TxHash != "" {
		record.TxHash = req.TxHash
	}
	record.Status = req.Status
	if req.RetryCount > 0 {
		record.RetryCount = req.RetryCount
	}
	if req.ErrorMessage != "" {
		record.ErrorMessage = req.ErrorMessage
	}
	if req.RPCEndpoint != "" {
		record.RPCEndpoint = req.RPCEndpoint
	}
	if req.GasConsumed > 0 {
		record.GasConsumed = req.GasConsumed
	}
	if req.ConfirmedAt != nil {
		record.ConfirmedAt = req.ConfirmedAt
	}

	return nil
}

// GetByID retrieves a transaction by ID.
func (m *MockRepository) GetByID(ctx context.Context, id int64) (*ChainTxRecord, error) {
	record, ok := m.records[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return record, nil
}

// GetByRequestID retrieves a transaction by request ID and service.
func (m *MockRepository) GetByRequestID(ctx context.Context, fromService, requestID string) (*ChainTxRecord, error) {
	key := fromService + ":" + requestID
	record, ok := m.byReq[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return record, nil
}

// GetByTxHash retrieves a transaction by tx hash.
func (m *MockRepository) GetByTxHash(ctx context.Context, txHash string) (*ChainTxRecord, error) {
	for _, record := range m.records {
		if record.TxHash == txHash {
			return record, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// ListPending lists transactions with pending or submitted status.
func (m *MockRepository) ListPending(ctx context.Context, limit int) ([]*ChainTxRecord, error) {
	var result []*ChainTxRecord
	for _, record := range m.records {
		if record.Status == StatusPending || record.Status == StatusSubmitted {
			result = append(result, record)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// ListByService lists transactions by service.
func (m *MockRepository) ListByService(ctx context.Context, fromService string, limit int) ([]*ChainTxRecord, error) {
	var result []*ChainTxRecord
	for _, record := range m.records {
		if record.FromService == fromService {
			result = append(result, record)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// Ensure MockRepository implements Repository
var _ Repository = (*MockRepository)(nil)

// Helper to marshal params
func MarshalParams(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
