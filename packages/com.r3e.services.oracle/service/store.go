// Package oracle provides oracle data source and request management.
// This file contains the storage interface that the service depends on.
// Following the Android OS pattern, the service defines its own storage contract
// and the system/infrastructure layer provides implementations.
package oracle

import (
	"context"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

// Store defines the persistence contract for oracle data sources and requests.
// Implementations are provided by the infrastructure layer (postgres, memory, etc.).
type Store interface {
	CreateDataSource(ctx context.Context, src DataSource) (DataSource, error)
	UpdateDataSource(ctx context.Context, src DataSource) (DataSource, error)
	GetDataSource(ctx context.Context, id string) (DataSource, error)
	ListDataSources(ctx context.Context, accountID string) ([]DataSource, error)

	CreateRequest(ctx context.Context, req Request) (Request, error)
	UpdateRequest(ctx context.Context, req Request) (Request, error)
	GetRequest(ctx context.Context, id string) (Request, error)
	ListRequests(ctx context.Context, accountID string, limit int, status string) ([]Request, error)
	ListPendingRequests(ctx context.Context) ([]Request, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
// Use framework.AccountChecker directly in new code.
type AccountChecker = framework.AccountChecker
