// Package ccip provides the CCIP Service as a ServicePackage.
package ccip

import (
	"context"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

// Store defines the persistence interface for ccip.
// This interface is defined within the service package, following the principle
// that "everything of the service must be in service package".
type Store interface {
	CreateLane(ctx context.Context, lane Lane) (Lane, error)
	UpdateLane(ctx context.Context, lane Lane) (Lane, error)
	GetLane(ctx context.Context, id string) (Lane, error)
	ListLanes(ctx context.Context, accountID string) ([]Lane, error)

	CreateMessage(ctx context.Context, msg Message) (Message, error)
	UpdateMessage(ctx context.Context, msg Message) (Message, error)
	GetMessage(ctx context.Context, id string) (Message, error)
	ListMessages(ctx context.Context, accountID string, limit int) ([]Message, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
// Use framework.AccountChecker directly in new code.
type AccountChecker = framework.AccountChecker

// WalletChecker is an alias for the canonical framework.WalletChecker interface.
// Use framework.WalletChecker directly in new code.
type WalletChecker = framework.WalletChecker
