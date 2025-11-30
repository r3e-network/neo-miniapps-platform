// Package datalink provides the Data Link Service as a ServicePackage.
package datalink

import (
	"context"

	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the persistence interface for datalink channels and deliveries.
// This interface is defined within the service package, following the principle
// that "everything of the service must be in service package".
type Store interface {
	CreateChannel(ctx context.Context, ch Channel) (Channel, error)
	UpdateChannel(ctx context.Context, ch Channel) (Channel, error)
	GetChannel(ctx context.Context, id string) (Channel, error)
	ListChannels(ctx context.Context, accountID string) ([]Channel, error)

	CreateDelivery(ctx context.Context, del Delivery) (Delivery, error)
	GetDelivery(ctx context.Context, id string) (Delivery, error)
	ListDeliveries(ctx context.Context, accountID string, limit int) ([]Delivery, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker

// WalletChecker is an alias for the canonical framework.WalletChecker interface.
type WalletChecker = framework.WalletChecker
