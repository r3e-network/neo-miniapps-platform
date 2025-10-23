package pricefeed

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages price feed definitions and price snapshots.
type Service struct {
	accounts storage.AccountStore
	store    storage.PriceFeedStore
	log      *logger.Logger
}

// New constructs a price feed service.
func New(accounts storage.AccountStore, store storage.PriceFeedStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("pricefeed")
	}
	return &Service{
		accounts: accounts,
		store:    store,
		log:      log,
	}
}

// CreateFeed registers a new price feed definition.
func (s *Service) CreateFeed(ctx context.Context, accountID, baseAsset, quoteAsset, updateInterval, heartbeat string, deviation float64) (pricefeed.Feed, error) {
	accountID = strings.TrimSpace(accountID)
	baseAsset = strings.TrimSpace(baseAsset)
	quoteAsset = strings.TrimSpace(quoteAsset)
	updateInterval = strings.TrimSpace(updateInterval)
	heartbeat = strings.TrimSpace(heartbeat)

	if accountID == "" {
		return pricefeed.Feed{}, fmt.Errorf("account_id is required")
	}
	if baseAsset == "" || quoteAsset == "" {
		return pricefeed.Feed{}, fmt.Errorf("base_asset and quote_asset are required")
	}
	if deviation <= 0 {
		return pricefeed.Feed{}, fmt.Errorf("deviation_percent must be positive")
	}
	if updateInterval == "" {
		updateInterval = "@every 1m"
	}
	if heartbeat == "" {
		heartbeat = "@every 10m"
	}

	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, accountID); err != nil {
			return pricefeed.Feed{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	pair := strings.ToUpper(baseAsset) + "/" + strings.ToUpper(quoteAsset)

	existing, err := s.store.ListPriceFeeds(ctx, accountID)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	for _, feed := range existing {
		if strings.EqualFold(feed.Pair, pair) {
			return pricefeed.Feed{}, fmt.Errorf("price feed for pair %s already exists", pair)
		}
	}

	feed := pricefeed.Feed{
		AccountID:        accountID,
		BaseAsset:        strings.ToUpper(baseAsset),
		QuoteAsset:       strings.ToUpper(quoteAsset),
		Pair:             pair,
		UpdateInterval:   updateInterval,
		Heartbeat:        heartbeat,
		DeviationPercent: deviation,
		Active:           true,
	}
	feed, err = s.store.CreatePriceFeed(ctx, feed)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	s.log.WithField("feed_id", feed.ID).
		WithField("account_id", accountID).
		WithField("pair", feed.Pair).
		Info("price feed created")
	return feed, nil
}

// UpdateFeed updates mutable fields on a feed.
func (s *Service) UpdateFeed(ctx context.Context, feedID string, interval, heartbeat *string, deviation *float64) (pricefeed.Feed, error) {
	feed, err := s.store.GetPriceFeed(ctx, feedID)
	if err != nil {
		return pricefeed.Feed{}, err
	}

	if interval != nil {
		if trimmed := strings.TrimSpace(*interval); trimmed != "" {
			feed.UpdateInterval = trimmed
		} else {
			return pricefeed.Feed{}, fmt.Errorf("update_interval cannot be empty")
		}
	}
	if heartbeat != nil {
		if trimmed := strings.TrimSpace(*heartbeat); trimmed != "" {
			feed.Heartbeat = trimmed
		} else {
			return pricefeed.Feed{}, fmt.Errorf("heartbeat_interval cannot be empty")
		}
	}
	if deviation != nil {
		if *deviation <= 0 {
			return pricefeed.Feed{}, fmt.Errorf("deviation_percent must be positive")
		}
		feed.DeviationPercent = *deviation
	}

	feed, err = s.store.UpdatePriceFeed(ctx, feed)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	s.log.WithField("feed_id", feed.ID).
		WithField("account_id", feed.AccountID).
		Info("price feed updated")
	return feed, nil
}

// SetActive toggles the active flag.
func (s *Service) SetActive(ctx context.Context, feedID string, active bool) (pricefeed.Feed, error) {
	feed, err := s.store.GetPriceFeed(ctx, feedID)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	if feed.Active == active {
		return feed, nil
	}

	feed.Active = active
	feed, err = s.store.UpdatePriceFeed(ctx, feed)
	if err != nil {
		return pricefeed.Feed{}, err
	}

	s.log.WithField("feed_id", feed.ID).
		WithField("account_id", feed.AccountID).
		WithField("active", active).
		Info("price feed state changed")
	return feed, nil
}

// RecordSnapshot stores a price observation.
func (s *Service) RecordSnapshot(ctx context.Context, feedID string, price float64, source string, collectedAt time.Time) (pricefeed.Snapshot, error) {
	if price <= 0 {
		return pricefeed.Snapshot{}, fmt.Errorf("price must be positive")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}

	if _, err := s.store.GetPriceFeed(ctx, feedID); err != nil {
		return pricefeed.Snapshot{}, err
	}

	snap := pricefeed.Snapshot{
		FeedID:      feedID,
		Price:       price,
		Source:      source,
		CollectedAt: collectedAt.UTC(),
	}
	if snap.CollectedAt.IsZero() {
		snap.CollectedAt = time.Now().UTC()
	}
	return s.store.CreatePriceSnapshot(ctx, snap)
}

// ListFeeds returns feeds for an account.
func (s *Service) ListFeeds(ctx context.Context, accountID string) ([]pricefeed.Feed, error) {
	return s.store.ListPriceFeeds(ctx, accountID)
}

// ListSnapshots returns recorded prices for a feed.
func (s *Service) ListSnapshots(ctx context.Context, feedID string) ([]pricefeed.Snapshot, error) {
	return s.store.ListPriceSnapshots(ctx, feedID)
}

// GetFeed retrieves a single feed by identifier.
func (s *Service) GetFeed(ctx context.Context, feedID string) (pricefeed.Feed, error) {
	return s.store.GetPriceFeed(ctx, feedID)
}
