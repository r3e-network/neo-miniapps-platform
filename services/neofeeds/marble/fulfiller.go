// Package neofeeds provides chain push logic for the price feed aggregation service.
package neofeeds

import (
	"context"
	"fmt"
	"math/big"
)

// =============================================================================
// Chain Push Logic (Push/Auto-Update Pattern)
// =============================================================================

// DefaultFeeds defines the default price feeds (for backward compatibility).
var DefaultFeeds = []string{
	"BTC/USD",
	"ETH/USD",
	"NEO/USD",
	"GAS/USD",
	"NEO/GAS",
}

// pushPricesToChain fetches all configured prices and pushes them on-chain.
func (s *Service) pushPricesToChain(ctx context.Context) {
	enabledFeeds := s.GetEnabledFeeds()
	if len(enabledFeeds) == 0 {
		return
	}

	feedIDs := make([]string, 0, len(enabledFeeds))
	prices := make([]*big.Int, 0, len(enabledFeeds))
	timestamps := make([]uint64, 0, len(enabledFeeds))

	for i := range enabledFeeds {
		feed := &enabledFeeds[i]
		pair := feed.Pair
		if pair == "" {
			pair = feedIDToPair(feed.ID)
		}

		price, err := s.GetPrice(ctx, pair)
		if err != nil {
			continue
		}

		timestampMillis := price.Timestamp.UnixMilli()
		if timestampMillis < 0 {
			continue
		}

		feedIDs = append(feedIDs, feed.ID)
		prices = append(prices, big.NewInt(price.Price))
		timestamps = append(timestamps, uint64(timestampMillis))
	}

	if len(feedIDs) == 0 {
		return
	}

	if _, err := s.teeFulfiller.UpdatePrices(ctx, s.neoFeedsHash, feedIDs, prices, timestamps); err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
			"feeds": len(feedIDs),
		}).Warn("failed to push prices on-chain")
	}
}

// PushSinglePrice pushes a single price update on-chain.
func (s *Service) PushSinglePrice(ctx context.Context, feedID string) error {
	if s.teeFulfiller == nil || s.neoFeedsHash == "" {
		return fmt.Errorf("chain push not configured")
	}

	pair := feedIDToPair(feedID)
	price, err := s.GetPrice(ctx, pair)
	if err != nil {
		return fmt.Errorf("get price: %w", err)
	}

	timestampMillis := price.Timestamp.UnixMilli()
	if timestampMillis < 0 {
		return fmt.Errorf("invalid timestamp for feed %s", feedID)
	}

	_, err = s.teeFulfiller.UpdatePrice(
		ctx,
		s.neoFeedsHash,
		feedID,
		big.NewInt(price.Price),
		uint64(timestampMillis),
	)
	return err
}

// feedIDToPair converts a feed ID to a trading pair format.
// e.g., "BTC/USD" -> "BTCUSD"
func feedIDToPair(feedID string) string {
	pair := ""
	for _, c := range feedID {
		if c != '/' {
			pair += string(c)
		}
	}
	return pair
}
