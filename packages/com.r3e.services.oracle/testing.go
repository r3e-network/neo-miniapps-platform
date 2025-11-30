package oracle

import (
	"context"
	"database/sql"

	domainOracle "github.com/R3E-Network/service_layer/domain/oracle"
	"github.com/R3E-Network/service_layer/pkg/storage"
)

// memoryStoreAdapter adapts pkg/storage interfaces to local oracle interfaces for testing.
type memoryStoreAdapter struct {
	oracleStore  storage.OracleStore
	accountStore storage.AccountStore
}

// NewMemoryStoreAdapter creates an adapter for testing with memory stores.
func NewMemoryStoreAdapter(oracleStore storage.OracleStore, accountStore storage.AccountStore) Store {
	return &memoryStoreAdapter{
		oracleStore:  oracleStore,
		accountStore: accountStore,
	}
}

func (a *memoryStoreAdapter) CreateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	result, err := a.oracleStore.CreateDataSource(ctx, toExternalDataSource(src))
	if err != nil {
		return DataSource{}, err
	}
	return fromExternalDataSource(result), nil
}

func (a *memoryStoreAdapter) UpdateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	result, err := a.oracleStore.UpdateDataSource(ctx, toExternalDataSource(src))
	if err != nil {
		return DataSource{}, err
	}
	return fromExternalDataSource(result), nil
}

func (a *memoryStoreAdapter) GetDataSource(ctx context.Context, id string) (DataSource, error) {
	result, err := a.oracleStore.GetDataSource(ctx, id)
	if err != nil {
		return DataSource{}, err
	}
	return fromExternalDataSource(result), nil
}

func (a *memoryStoreAdapter) ListDataSources(ctx context.Context, accountID string) ([]DataSource, error) {
	results, err := a.oracleStore.ListDataSources(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]DataSource, len(results))
	for i, r := range results {
		out[i] = fromExternalDataSource(r)
	}
	return out, nil
}

func (a *memoryStoreAdapter) CreateRequest(ctx context.Context, req Request) (Request, error) {
	result, err := a.oracleStore.CreateRequest(ctx, toExternalRequest(req))
	if err != nil {
		return Request{}, err
	}
	return fromExternalRequest(result), nil
}

func (a *memoryStoreAdapter) UpdateRequest(ctx context.Context, req Request) (Request, error) {
	result, err := a.oracleStore.UpdateRequest(ctx, toExternalRequest(req))
	if err != nil {
		return Request{}, err
	}
	return fromExternalRequest(result), nil
}

func (a *memoryStoreAdapter) GetRequest(ctx context.Context, id string) (Request, error) {
	result, err := a.oracleStore.GetRequest(ctx, id)
	if err != nil {
		return Request{}, err
	}
	return fromExternalRequest(result), nil
}

func (a *memoryStoreAdapter) ListRequests(ctx context.Context, accountID string, limit int, status string) ([]Request, error) {
	results, err := a.oracleStore.ListRequests(ctx, accountID, limit, status)
	if err != nil {
		return nil, err
	}
	out := make([]Request, len(results))
	for i, r := range results {
		out[i] = fromExternalRequest(r)
	}
	return out, nil
}

func (a *memoryStoreAdapter) ListPendingRequests(ctx context.Context) ([]Request, error) {
	results, err := a.oracleStore.ListPendingRequests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Request, len(results))
	for i, r := range results {
		out[i] = fromExternalRequest(r)
	}
	return out, nil
}

// memoryAccountChecker adapts storage.AccountStore to AccountChecker for testing.
type memoryAccountChecker struct {
	store storage.AccountStore
}

// NewMemoryAccountChecker creates an AccountChecker adapter for testing.
func NewMemoryAccountChecker(store storage.AccountStore) AccountChecker {
	return &memoryAccountChecker{store: store}
}

func (a *memoryAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	_, err := a.store.GetAccount(ctx, accountID)
	if err == sql.ErrNoRows {
		return err
	}
	return err
}

func (a *memoryAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	// memory.Store doesn't support tenants, return empty string
	return ""
}

// Type conversion helpers for adapting domain/oracle types to local types.

func toExternalDataSource(src DataSource) domainOracle.DataSource {
	return domainOracle.DataSource{
		ID:          src.ID,
		AccountID:   src.AccountID,
		Name:        src.Name,
		Description: src.Description,
		URL:         src.URL,
		Method:      src.Method,
		Headers:     src.Headers,
		Body:        src.Body,
		Enabled:     src.Enabled,
		CreatedAt:   src.CreatedAt,
		UpdatedAt:   src.UpdatedAt,
	}
}

func fromExternalDataSource(src domainOracle.DataSource) DataSource {
	return DataSource{
		ID:          src.ID,
		AccountID:   src.AccountID,
		Name:        src.Name,
		Description: src.Description,
		URL:         src.URL,
		Method:      src.Method,
		Headers:     src.Headers,
		Body:        src.Body,
		Enabled:     src.Enabled,
		CreatedAt:   src.CreatedAt,
		UpdatedAt:   src.UpdatedAt,
	}
}

func toExternalRequest(req Request) domainOracle.Request {
	return domainOracle.Request{
		ID:           req.ID,
		AccountID:    req.AccountID,
		DataSourceID: req.DataSourceID,
		Status:       domainOracle.RequestStatus(req.Status),
		Attempts:     req.Attempts,
		Fee:          req.Fee,
		Payload:      req.Payload,
		Result:       req.Result,
		Error:        req.Error,
		CreatedAt:    req.CreatedAt,
		UpdatedAt:    req.UpdatedAt,
		CompletedAt:  req.CompletedAt,
	}
}

func fromExternalRequest(req domainOracle.Request) Request {
	return Request{
		ID:           req.ID,
		AccountID:    req.AccountID,
		DataSourceID: req.DataSourceID,
		Status:       RequestStatus(req.Status),
		Attempts:     req.Attempts,
		Fee:          req.Fee,
		Payload:      req.Payload,
		Result:       req.Result,
		Error:        req.Error,
		CreatedAt:    req.CreatedAt,
		UpdatedAt:    req.UpdatedAt,
		CompletedAt:  req.CompletedAt,
	}
}
