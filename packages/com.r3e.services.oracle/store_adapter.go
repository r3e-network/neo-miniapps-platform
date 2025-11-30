// Package oracle provides oracle data source and request management.
// This file contains the storage adapter that bridges the external storage
// implementation (using domain/oracle types) with the service's local types.
// This follows the Android OS pattern where the system provides storage
// implementations and services adapt them to their local contracts.
package oracle

import (
	"context"

	"github.com/R3E-Network/service_layer/pkg/storage"
	domain "github.com/R3E-Network/service_layer/domain/oracle"
)

// StoreAdapter wraps a storage.OracleStore and adapts it to the local Store interface.
// This enables the service to use its own domain types while leveraging the
// system-provided storage implementation.
type StoreAdapter struct {
	store storage.OracleStore
}

// NewStoreAdapter creates a new adapter wrapping the given storage.OracleStore.
func NewStoreAdapter(store storage.OracleStore) *StoreAdapter {
	return &StoreAdapter{store: store}
}

// CreateDataSource adapts the call to the underlying store.
func (a *StoreAdapter) CreateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	result, err := a.store.CreateDataSource(ctx, toExternalDataSource(src))
	if err != nil {
		return DataSource{}, err
	}
	return fromExternalDataSource(result), nil
}

// UpdateDataSource adapts the call to the underlying store.
func (a *StoreAdapter) UpdateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	result, err := a.store.UpdateDataSource(ctx, toExternalDataSource(src))
	if err != nil {
		return DataSource{}, err
	}
	return fromExternalDataSource(result), nil
}

// GetDataSource adapts the call to the underlying store.
func (a *StoreAdapter) GetDataSource(ctx context.Context, id string) (DataSource, error) {
	result, err := a.store.GetDataSource(ctx, id)
	if err != nil {
		return DataSource{}, err
	}
	return fromExternalDataSource(result), nil
}

// ListDataSources adapts the call to the underlying store.
func (a *StoreAdapter) ListDataSources(ctx context.Context, accountID string) ([]DataSource, error) {
	results, err := a.store.ListDataSources(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]DataSource, len(results))
	for i, r := range results {
		out[i] = fromExternalDataSource(r)
	}
	return out, nil
}

// CreateRequest adapts the call to the underlying store.
func (a *StoreAdapter) CreateRequest(ctx context.Context, req Request) (Request, error) {
	result, err := a.store.CreateRequest(ctx, toExternalRequest(req))
	if err != nil {
		return Request{}, err
	}
	return fromExternalRequest(result), nil
}

// UpdateRequest adapts the call to the underlying store.
func (a *StoreAdapter) UpdateRequest(ctx context.Context, req Request) (Request, error) {
	result, err := a.store.UpdateRequest(ctx, toExternalRequest(req))
	if err != nil {
		return Request{}, err
	}
	return fromExternalRequest(result), nil
}

// GetRequest adapts the call to the underlying store.
func (a *StoreAdapter) GetRequest(ctx context.Context, id string) (Request, error) {
	result, err := a.store.GetRequest(ctx, id)
	if err != nil {
		return Request{}, err
	}
	return fromExternalRequest(result), nil
}

// ListRequests adapts the call to the underlying store.
func (a *StoreAdapter) ListRequests(ctx context.Context, accountID string, limit int, status string) ([]Request, error) {
	results, err := a.store.ListRequests(ctx, accountID, limit, status)
	if err != nil {
		return nil, err
	}
	out := make([]Request, len(results))
	for i, r := range results {
		out[i] = fromExternalRequest(r)
	}
	return out, nil
}

// ListPendingRequests adapts the call to the underlying store.
func (a *StoreAdapter) ListPendingRequests(ctx context.Context) ([]Request, error) {
	results, err := a.store.ListPendingRequests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Request, len(results))
	for i, r := range results {
		out[i] = fromExternalRequest(r)
	}
	return out, nil
}

// Type conversion helpers

func toExternalDataSource(src DataSource) domain.DataSource {
	return domain.DataSource{
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

func fromExternalDataSource(src domain.DataSource) DataSource {
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

func toExternalRequest(req Request) domain.Request {
	return domain.Request{
		ID:           req.ID,
		AccountID:    req.AccountID,
		DataSourceID: req.DataSourceID,
		Status:       domain.RequestStatus(req.Status),
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

func fromExternalRequest(req domain.Request) Request {
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
