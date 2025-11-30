// Package datafeeds provides the DATAFEEDS Service as a ServicePackage.
package datafeeds

import (
	"context"

	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the persistence interface for datafeeds.
// Uses local type aliases which are compatible with domain types.
type Store interface {
	CreateDataFeed(ctx context.Context, feed Feed) (Feed, error)
	UpdateDataFeed(ctx context.Context, feed Feed) (Feed, error)
	GetDataFeed(ctx context.Context, id string) (Feed, error)
	ListDataFeeds(ctx context.Context, accountID string) ([]Feed, error)

	CreateDataFeedUpdate(ctx context.Context, upd Update) (Update, error)
	ListDataFeedUpdates(ctx context.Context, feedID string, limit int) ([]Update, error)
	ListDataFeedUpdatesByRound(ctx context.Context, feedID string, roundID int64) ([]Update, error)
	GetLatestDataFeedUpdate(ctx context.Context, feedID string) (Update, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker

// WalletChecker is an alias for the canonical framework.WalletChecker interface.
type WalletChecker = framework.WalletChecker
