// Package neofeeds provides price feed aggregation service.
package neofeeds

import (
	"context"
	"fmt"
	"math/big"

	txclient "github.com/R3E-Network/service_layer/services/txsubmitter/client"
)

// =============================================================================
// TxSubmitter Adapter
// =============================================================================

// TxSubmitterAdapter wraps TxSubmitter client for NeoFeeds chain operations.
// This replaces direct TEEFulfiller usage for centralized chain writes.
type TxSubmitterAdapter struct {
	client       *txclient.Client
	neoFeedsHash string
}

// NewTxSubmitterAdapter creates a new TxSubmitter adapter for NeoFeeds.
func NewTxSubmitterAdapter(client *txclient.Client, neoFeedsHash string) *TxSubmitterAdapter {
	return &TxSubmitterAdapter{
		client:       client,
		neoFeedsHash: neoFeedsHash,
	}
}

// UpdatePrices submits a batch price update via TxSubmitter.
func (a *TxSubmitterAdapter) UpdatePrices(ctx context.Context, feedIDs []string, prices []*big.Int, timestamps []uint64) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("txsubmitter client not configured")
	}

	// Convert prices to strings for JSON serialization
	priceStrs := make([]string, len(prices))
	for i, p := range prices {
		priceStrs[i] = p.String()
	}

	resp, err := a.client.UpdatePrices(ctx, feedIDs, priceStrs, timestamps)
	if err != nil {
		return "", fmt.Errorf("update prices via txsubmitter: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("txsubmitter error: %s", resp.Error)
	}

	return resp.TxHash, nil
}

// UpdatePrice submits a single price update via TxSubmitter.
func (a *TxSubmitterAdapter) UpdatePrice(ctx context.Context, feedID string, price *big.Int, timestamp uint64) (string, error) {
	return a.UpdatePrices(ctx, []string{feedID}, []*big.Int{price}, []uint64{timestamp})
}

// =============================================================================
// Service Integration
// =============================================================================

// SetTxSubmitterClient sets the TxSubmitter client for chain operations.
// This enables migration from direct TEEFulfiller to centralized TxSubmitter.
func (s *Service) SetTxSubmitterClient(client *txclient.Client) {
	s.txSubmitterAdapter = NewTxSubmitterAdapter(client, s.neoFeedsHash)
}

// pushPricesToChainViaTxSubmitter pushes prices using TxSubmitter.
func (s *Service) pushPricesToChainViaTxSubmitter(ctx context.Context) {
	if s.txSubmitterAdapter == nil {
		// Fallback to legacy TEEFulfiller if TxSubmitter not configured
		s.pushPricesToChain(ctx)
		return
	}

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

	txHash, err := s.txSubmitterAdapter.UpdatePrices(ctx, feedIDs, prices, timestamps)
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
			"feeds": len(feedIDs),
		}).Warn("failed to push prices via TxSubmitter")
		return
	}

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"feeds":   len(feedIDs),
		"tx_hash": txHash,
	}).Info("prices pushed via TxSubmitter")
}

// PushSinglePriceViaTxSubmitter pushes a single price update via TxSubmitter.
func (s *Service) PushSinglePriceViaTxSubmitter(ctx context.Context, feedID string) error {
	if s.txSubmitterAdapter == nil {
		return s.PushSinglePrice(ctx, feedID)
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

	_, err = s.txSubmitterAdapter.UpdatePrice(ctx, feedID, big.NewInt(price.Price), uint64(timestampMillis))
	return err
}
