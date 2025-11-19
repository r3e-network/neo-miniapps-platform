package pricefeed

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages price feed definitions and price snapshots.
type Service struct {
	base           *core.Base
	store          storage.PriceFeedStore
	log            *logger.Logger
	hooks          core.ObservationHooks
	minSubmissions int
}

// New constructs a price feed service.
func New(accounts storage.AccountStore, store storage.PriceFeedStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("pricefeed")
	}
	return &Service{
		base:           core.NewBase(accounts),
		store:          store,
		log:            log,
		hooks:          core.NoopObservationHooks,
		minSubmissions: 1,
	}
}

// WithObservationHooks configures callbacks for observation submissions.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	if h.OnStart == nil && h.OnComplete == nil {
		s.hooks = core.NoopObservationHooks
		return
	}
	s.hooks = h
}

// SetMinimumSubmissions configures the number of observations required to finalise a round.
func (s *Service) SetMinimumSubmissions(n int) {
	if n < 1 {
		n = 1
	}
	s.minSubmissions = n
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

	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return pricefeed.Feed{}, fmt.Errorf("account validation failed: %w", err)
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
	_, snapshot, err := s.SubmitObservation(ctx, feedID, price, source, collectedAt)
	return snapshot, err
}

// SubmitObservation records an individual data point and finalises a new price round.
func (s *Service) SubmitObservation(ctx context.Context, feedID string, price float64, source string, collectedAt time.Time) (pricefeed.Round, pricefeed.Snapshot, error) {
	price = normalizePrice(price)
	if price <= 0 {
		return pricefeed.Round{}, pricefeed.Snapshot{}, fmt.Errorf("price must be positive")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}

	feed, err := s.store.GetPriceFeed(ctx, feedID)
	if err != nil {
		return pricefeed.Round{}, pricefeed.Snapshot{}, err
	}
	if !feed.Active {
		return pricefeed.Round{}, pricefeed.Snapshot{}, fmt.Errorf("price feed %s is not active", feedID)
	}

	collectedAt = collectedAt.UTC()
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}

	roundHistory, err := s.store.ListPriceRounds(ctx, feedID, 5)
	if err != nil {
		return pricefeed.Round{}, pricefeed.Snapshot{}, err
	}

	var (
		round        pricefeed.Round
		currentRound int64 = 1
		newRound     bool
		prevFinal    *pricefeed.Round
	)
	for _, r := range roundHistory {
		tmp := r
		if !r.Finalized && round.RoundID == 0 {
			round = tmp
			currentRound = tmp.RoundID
			continue
		}
		if r.Finalized {
			prevFinal = &tmp
			break
		}
	}
	if round.RoundID == 0 {
		if len(roundHistory) > 0 {
			currentRound = roundHistory[0].RoundID + 1
		}
		round = pricefeed.Round{
			FeedID:    feedID,
			RoundID:   currentRound,
			StartedAt: collectedAt,
			Finalized: false,
		}
		round, err = s.store.CreatePriceRound(ctx, round)
		if err != nil {
			return pricefeed.Round{}, pricefeed.Snapshot{}, err
		}
		newRound = true
	}

	attrs := map[string]string{"feed_id": feedID}
	finish := core.StartObservation(ctx, s.hooks, attrs)
	observation := pricefeed.Observation{
		FeedID:      feedID,
		RoundID:     currentRound,
		Source:      source,
		Price:       price,
		CollectedAt: collectedAt,
	}
	if _, err := s.store.CreatePriceObservation(ctx, observation); err != nil {
		finish(err)
		return pricefeed.Round{}, pricefeed.Snapshot{}, err
	}

	observations, err := s.store.ListPriceObservations(ctx, feedID, currentRound, core.MaxListLimit)
	if err != nil {
		finish(err)
		return pricefeed.Round{}, pricefeed.Snapshot{}, err
	}
	count := len(observations)
	if count == 0 {
		finish(nil)
		return round, pricefeed.Snapshot{}, nil
	}

	prices := make([]float64, 0, count)
	earliest := observations[0].CollectedAt
	latest := observations[0].CollectedAt
	for _, obs := range observations {
		prices = append(prices, obs.Price)
		if obs.CollectedAt.Before(earliest) {
			earliest = obs.CollectedAt
		}
		if obs.CollectedAt.After(latest) {
			latest = obs.CollectedAt
		}
	}

	round.StartedAt = earliest
	round.AggregatedPrice = median(prices)
	round.ObservationCount = count

	ready := count >= s.minSubmissions
	heartbeatDur, hbErr := parseInterval(feed.Heartbeat)
	if hbErr != nil {
		s.log.WithError(hbErr).
			WithField("heartbeat", feed.Heartbeat).
			Warn("parse heartbeat interval")
	}
	shouldPublish := ready && shouldPublishRound(round.AggregatedPrice, latest, prevFinal, feed.DeviationPercent, heartbeatDur)
	if shouldPublish {
		round.Finalized = true
		round.ClosedAt = latest
	} else {
		round.Finalized = false
		round.ClosedAt = time.Time{}
	}

	round, err = s.store.UpdatePriceRound(ctx, round)
	if err != nil {
		finish(err)
		return pricefeed.Round{}, pricefeed.Snapshot{}, err
	}

	if !shouldPublish {
		if newRound {
			s.log.WithField("feed_id", feedID).
				WithField("round_id", round.RoundID).
				Debug("price round opened")
		}
		finish(nil)
		return round, pricefeed.Snapshot{}, nil
	}

	snap := pricefeed.Snapshot{
		FeedID:      feedID,
		Price:       round.AggregatedPrice,
		Source:      source,
		CollectedAt: latest,
	}
	snap, err = s.store.CreatePriceSnapshot(ctx, snap)
	if err != nil {
		finish(err)
		return pricefeed.Round{}, pricefeed.Snapshot{}, err
	}

	s.log.WithField("feed_id", feedID).
		WithField("round_id", round.RoundID).
		WithField("price", round.AggregatedPrice).
		Info("price round finalized")
	finish(nil)
	return round, snap, nil
}

// LatestRound returns the most recent aggregated round for the feed.
func (s *Service) LatestRound(ctx context.Context, feedID string) (pricefeed.Round, error) {
	rounds, err := s.store.ListPriceRounds(ctx, feedID, 1)
	if err != nil {
		return pricefeed.Round{}, err
	}
	if len(rounds) == 0 {
		return pricefeed.Round{}, fmt.Errorf("no rounds for feed %s", feedID)
	}
	return rounds[0], nil
}

// ListRounds returns recent aggregated rounds in descending order.
func (s *Service) ListRounds(ctx context.Context, feedID string, limit int) ([]pricefeed.Round, error) {
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListPriceRounds(ctx, feedID, clamped)
}

// ListObservations returns the recorded submissions for a round.
func (s *Service) ListObservations(ctx context.Context, accountID, feedID string, roundID int64, limit int) ([]pricefeed.Observation, error) {
	feed, err := s.store.GetPriceFeed(ctx, feedID)
	if err != nil {
		return nil, err
	}
	if feed.AccountID != accountID {
		return nil, fmt.Errorf("feed %s does not belong to account %s", feedID, accountID)
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListPriceObservations(ctx, feedID, roundID, clamped)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "pricefeed",
		Domain:       "pricefeed",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"feeds", "rounds", "observations"},
	}
}

func normalizePrice(price float64) float64 {
	if price < 0 {
		return -price
	}
	return price
}

func parseInterval(spec string) (time.Duration, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, nil
	}
	if strings.HasPrefix(spec, "@every") {
		durSpec := strings.TrimSpace(spec[len("@every"):])
		return time.ParseDuration(durSpec)
	}
	return time.ParseDuration(spec)
}

func shouldPublishRound(newPrice float64, latest time.Time, prev *pricefeed.Round, deviation float64, heartbeat time.Duration) bool {
	if prev == nil {
		return true
	}
	if heartbeat > 0 && !prev.ClosedAt.IsZero() && latest.Sub(prev.ClosedAt) >= heartbeat {
		return true
	}
	if deviation <= 0 {
		return true
	}
	if prev.AggregatedPrice == 0 {
		return true
	}
	change := 100 * math.Abs(newPrice-prev.AggregatedPrice) / math.Abs(prev.AggregatedPrice)
	return change >= deviation
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
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
