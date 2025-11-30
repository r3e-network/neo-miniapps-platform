package oracle

import (
	"context"

	domainOracle "github.com/R3E-Network/service_layer/domain/oracle"
	"github.com/R3E-Network/service_layer/pkg/storage"
)

// legacyStoreAdapter adapts storage.OracleStore to local Store interface.
type legacyStoreAdapter struct {
	store    storage.OracleStore
	accounts AccountChecker
}

// NewLegacyStoreAdapter creates an adapter from storage.OracleStore to local Store.
func NewLegacyStoreAdapter(store storage.OracleStore, accounts AccountChecker) Store {
	return &legacyStoreAdapter{store: store, accounts: accounts}
}

func (a *legacyStoreAdapter) CreateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	result, err := a.store.CreateDataSource(ctx, toExtDataSource(src))
	if err != nil {
		return DataSource{}, err
	}
	return fromExtDataSource(result), nil
}

func (a *legacyStoreAdapter) UpdateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	result, err := a.store.UpdateDataSource(ctx, toExtDataSource(src))
	if err != nil {
		return DataSource{}, err
	}
	return fromExtDataSource(result), nil
}

func (a *legacyStoreAdapter) GetDataSource(ctx context.Context, id string) (DataSource, error) {
	result, err := a.store.GetDataSource(ctx, id)
	if err != nil {
		return DataSource{}, err
	}
	return fromExtDataSource(result), nil
}

func (a *legacyStoreAdapter) ListDataSources(ctx context.Context, accountID string) ([]DataSource, error) {
	results, err := a.store.ListDataSources(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]DataSource, len(results))
	for i, r := range results {
		out[i] = fromExtDataSource(r)
	}
	return out, nil
}

func (a *legacyStoreAdapter) CreateRequest(ctx context.Context, req Request) (Request, error) {
	result, err := a.store.CreateRequest(ctx, toExtRequest(req))
	if err != nil {
		return Request{}, err
	}
	return fromExtRequest(result), nil
}

func (a *legacyStoreAdapter) UpdateRequest(ctx context.Context, req Request) (Request, error) {
	result, err := a.store.UpdateRequest(ctx, toExtRequest(req))
	if err != nil {
		return Request{}, err
	}
	return fromExtRequest(result), nil
}

func (a *legacyStoreAdapter) GetRequest(ctx context.Context, id string) (Request, error) {
	result, err := a.store.GetRequest(ctx, id)
	if err != nil {
		return Request{}, err
	}
	return fromExtRequest(result), nil
}

func (a *legacyStoreAdapter) ListRequests(ctx context.Context, accountID string, limit int, status string) ([]Request, error) {
	results, err := a.store.ListRequests(ctx, accountID, limit, status)
	if err != nil {
		return nil, err
	}
	out := make([]Request, len(results))
	for i, r := range results {
		out[i] = fromExtRequest(r)
	}
	return out, nil
}

func (a *legacyStoreAdapter) ListPendingRequests(ctx context.Context) ([]Request, error) {
	results, err := a.store.ListPendingRequests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Request, len(results))
	for i, r := range results {
		out[i] = fromExtRequest(r)
	}
	return out, nil
}

// Type conversion helpers for legacy adapter

func toExtDataSource(src DataSource) domainOracle.DataSource {
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

func fromExtDataSource(src domainOracle.DataSource) DataSource {
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

func toExtRequest(req Request) domainOracle.Request {
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

func fromExtRequest(req domainOracle.Request) Request {
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
