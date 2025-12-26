// Package supabase provides NeoRequests-specific database operations.
package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/R3E-Network/service_layer/infrastructure/database"
)

const (
	miniappsTable        = "miniapps"
	serviceRequestsTable = "service_requests"
	chainTxsTable        = "chain_txs"
	contractEventsTable  = "contract_events"
	processedEventsTable = "processed_events"
)

// RepositoryInterface defines NeoRequests data access methods.
type RepositoryInterface interface {
	GetMiniApp(ctx context.Context, appID string) (*MiniApp, error)
	CreateServiceRequest(ctx context.Context, req *ServiceRequest) error
	UpdateServiceRequest(ctx context.Context, req *ServiceRequest) error
	CreateChainTx(ctx context.Context, tx *ChainTx) error
	UpdateChainTx(ctx context.Context, tx *ChainTx) error
	CreateContractEvent(ctx context.Context, event *ContractEvent) error
	HasProcessedEvent(ctx context.Context, chainID, txHash string, logIndex int) (bool, error)
	CreateProcessedEvent(ctx context.Context, event *ProcessedEvent) error
	MarkProcessedEvent(ctx context.Context, event *ProcessedEvent) (bool, error)
	CreateNotification(ctx context.Context, n *Notification) error
}

// Ensure Repository implements RepositoryInterface.
var _ RepositoryInterface = (*Repository)(nil)

// Repository provides NeoRequests-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new NeoRequests repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// GetMiniApp retrieves a MiniApp manifest row by app_id.
func (r *Repository) GetMiniApp(ctx context.Context, appID string) (*MiniApp, error) {
	if appID == "" {
		return nil, fmt.Errorf("app_id cannot be empty")
	}

	query := database.NewQuery().
		Eq("app_id", appID).
		Limit(1).
		Build()

	rows, err := database.GenericListWithQuery[MiniApp](r.base, ctx, miniappsTable, query)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, database.NewNotFoundError(miniappsTable, appID)
	}
	return &rows[0], nil
}

// CreateServiceRequest inserts a new service request.
func (r *Repository) CreateServiceRequest(ctx context.Context, req *ServiceRequest) error {
	if req == nil {
		return fmt.Errorf("service request cannot be nil")
	}
	if req.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	if req.ServiceType == "" {
		return fmt.Errorf("service_type cannot be empty")
	}

	return database.GenericCreate(r.base, ctx, serviceRequestsTable, req, func(rows []ServiceRequest) {
		if len(rows) > 0 {
			*req = rows[0]
		}
	})
}

// UpdateServiceRequest updates an existing service request by id.
func (r *Repository) UpdateServiceRequest(ctx context.Context, req *ServiceRequest) error {
	if req == nil {
		return fmt.Errorf("service request cannot be nil")
	}
	if req.ID == "" {
		return fmt.Errorf("service request id cannot be empty")
	}
	return database.GenericUpdate(r.base, ctx, serviceRequestsTable, "id", req.ID, req)
}

// CreateChainTx inserts a new chain_txs row.
func (r *Repository) CreateChainTx(ctx context.Context, tx *ChainTx) error {
	if tx == nil {
		return fmt.Errorf("chain tx cannot be nil")
	}
	if tx.RequestID == "" {
		return fmt.Errorf("request_id cannot be empty")
	}
	if tx.FromService == "" {
		return fmt.Errorf("from_service cannot be empty")
	}
	if tx.ContractAddress == "" || tx.MethodName == "" {
		return fmt.Errorf("contract_address and method_name required")
	}

	return database.GenericCreate(r.base, ctx, chainTxsTable, tx, func(rows []ChainTx) {
		if len(rows) > 0 {
			*tx = rows[0]
		}
	})
}

// UpdateChainTx updates an existing chain_txs row by id.
func (r *Repository) UpdateChainTx(ctx context.Context, tx *ChainTx) error {
	if tx == nil {
		return fmt.Errorf("chain tx cannot be nil")
	}
	if tx.ID == 0 {
		return fmt.Errorf("chain tx id cannot be empty")
	}
	return database.GenericUpdate(r.base, ctx, chainTxsTable, "id", strconv.FormatInt(tx.ID, 10), tx)
}

// CreateContractEvent inserts a contract event row.
func (r *Repository) CreateContractEvent(ctx context.Context, event *ContractEvent) error {
	if event == nil {
		return fmt.Errorf("contract event cannot be nil")
	}
	if event.TxHash == "" || event.ContractHash == "" || event.EventName == "" {
		return fmt.Errorf("contract event missing required fields")
	}
	return database.GenericCreate(r.base, ctx, contractEventsTable, event, nil)
}

// HasProcessedEvent checks if the event was already processed.
func (r *Repository) HasProcessedEvent(ctx context.Context, chainID, txHash string, logIndex int) (bool, error) {
	if chainID == "" || txHash == "" {
		return false, fmt.Errorf("chain_id and tx_hash required")
	}
	query := database.NewQuery().
		Eq("chain_id", chainID).
		Eq("tx_hash", txHash).
		Eq("log_index", strconv.Itoa(logIndex)).
		Limit(1).
		Build()

	rows, err := database.GenericListWithQuery[ProcessedEvent](r.base, ctx, processedEventsTable, query)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// CreateProcessedEvent inserts a processed_events row.
func (r *Repository) CreateProcessedEvent(ctx context.Context, event *ProcessedEvent) error {
	if event == nil {
		return fmt.Errorf("processed event cannot be nil")
	}
	if event.ChainID == "" || event.TxHash == "" {
		return fmt.Errorf("processed event missing chain_id or tx_hash")
	}
	return database.GenericCreate(r.base, ctx, processedEventsTable, event, nil)
}

func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "409")
}

// MarkProcessedEvent attempts to insert a processed event and returns true if inserted.
// If the event already exists, it returns false.
func (r *Repository) MarkProcessedEvent(ctx context.Context, event *ProcessedEvent) (bool, error) {
	if event == nil {
		return false, fmt.Errorf("processed event cannot be nil")
	}

	exists, err := r.HasProcessedEvent(ctx, event.ChainID, event.TxHash, event.LogIndex)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	if err := r.CreateProcessedEvent(ctx, event); err != nil {
		if isDuplicateError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// MarshalParams marshals params to JSON.
func MarshalParams(params any) json.RawMessage {
	if params == nil {
		return json.RawMessage("null")
	}
	data, err := json.Marshal(params)
	if err != nil {
		return json.RawMessage("null")
	}
	return data
}

// CreateNotification inserts a notification record.
func (r *Repository) CreateNotification(ctx context.Context, n *Notification) error {
	if n == nil {
		return fmt.Errorf("notification cannot be nil")
	}
	if n.AppID == "" {
		return fmt.Errorf("notification app_id cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, "miniapp_notifications", n, nil)
}
