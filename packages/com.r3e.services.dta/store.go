// Package dta provides the DTA Service as a ServicePackage.
package dta

import (
	"context"

	
	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the persistence interface for 
type Store interface {
	CreateProduct(ctx context.Context, product Product) (Product, error)
	UpdateProduct(ctx context.Context, product Product) (Product, error)
	GetProduct(ctx context.Context, id string) (Product, error)
	ListProducts(ctx context.Context, accountID string) ([]Product, error)

	CreateOrder(ctx context.Context, order Order) (Order, error)
	GetOrder(ctx context.Context, id string) (Order, error)
	ListOrders(ctx context.Context, accountID string, limit int) ([]Order, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker

// WalletChecker is an alias for the canonical framework.WalletChecker interface.
type WalletChecker = framework.WalletChecker
