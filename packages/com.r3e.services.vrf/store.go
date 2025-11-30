// Package vrf provides the VRF Service as a ServicePackage.
package vrf

import (
	"context"

	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the persistence interface for VRF keys and requests.
// This interface is defined within the service package, following the principle
// that "everything of the service must be in service package".
type Store interface {
	// Key operations
	CreateKey(ctx context.Context, key Key) (Key, error)
	UpdateKey(ctx context.Context, key Key) (Key, error)
	GetKey(ctx context.Context, id string) (Key, error)
	ListKeys(ctx context.Context, accountID string) ([]Key, error)

	// Request operations
	CreateRequest(ctx context.Context, req Request) (Request, error)
	GetRequest(ctx context.Context, id string) (Request, error)
	ListRequests(ctx context.Context, accountID string, limit int) ([]Request, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker

// WalletChecker is an alias for the canonical framework.WalletChecker interface.
type WalletChecker = framework.WalletChecker
