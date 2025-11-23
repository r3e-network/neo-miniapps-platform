package postgres

import (
	"context"
	"database/sql"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
)

// PriceFeedStore implementation

func (s *Store) CreatePriceFeed(ctx context.Context, feed pricefeed.Feed) (pricefeed.Feed, error) {
	if feed.ID == "" {
		feed.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	feed.CreatedAt = now
	feed.UpdatedAt = now
	tenant := s.accountTenant(ctx, feed.AccountID)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_price_feeds (id, account_id, base_asset, quote_asset, pair, update_interval, deviation_percent, heartbeat_interval, active, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, feed.ID, feed.AccountID, feed.BaseAsset, feed.QuoteAsset, feed.Pair, feed.UpdateInterval, feed.DeviationPercent, feed.Heartbeat, feed.Active, tenant, feed.CreatedAt, feed.UpdatedAt)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	return feed, nil
}

func (s *Store) UpdatePriceFeed(ctx context.Context, feed pricefeed.Feed) (pricefeed.Feed, error) {
	existing, err := s.GetPriceFeed(ctx, feed.ID)
	if err != nil {
		return pricefeed.Feed{}, err
	}

	feed.AccountID = existing.AccountID
	feed.BaseAsset = existing.BaseAsset
	feed.QuoteAsset = existing.QuoteAsset
	feed.Pair = existing.Pair
	feed.CreatedAt = existing.CreatedAt
	feed.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, feed.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_price_feeds
		SET update_interval = $2, deviation_percent = $3, heartbeat_interval = $4, active = $5, tenant = $6, updated_at = $7
		WHERE id = $1
	`, feed.ID, feed.UpdateInterval, feed.DeviationPercent, feed.Heartbeat, feed.Active, tenant, feed.UpdatedAt)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return pricefeed.Feed{}, sql.ErrNoRows
	}
	return feed, nil
}

func (s *Store) GetPriceFeed(ctx context.Context, id string) (pricefeed.Feed, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, base_asset, quote_asset, pair, update_interval, deviation_percent, heartbeat_interval, active, created_at, updated_at
		FROM app_price_feeds
		WHERE id = $1
	`, id)

	var feed pricefeed.Feed
	if err := row.Scan(&feed.ID, &feed.AccountID, &feed.BaseAsset, &feed.QuoteAsset, &feed.Pair, &feed.UpdateInterval, &feed.DeviationPercent, &feed.Heartbeat, &feed.Active, &feed.CreatedAt, &feed.UpdatedAt); err != nil {
		return pricefeed.Feed{}, err
	}
	return feed, nil
}

func (s *Store) ListPriceFeeds(ctx context.Context, accountID string) ([]pricefeed.Feed, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, base_asset, quote_asset, pair, update_interval, deviation_percent, heartbeat_interval, active, created_at, updated_at
		FROM app_price_feeds
		WHERE $1 = '' OR account_id = $1
		ORDER BY created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []pricefeed.Feed
	for rows.Next() {
		var feed pricefeed.Feed
		if err := rows.Scan(&feed.ID, &feed.AccountID, &feed.BaseAsset, &feed.QuoteAsset, &feed.Pair, &feed.UpdateInterval, &feed.DeviationPercent, &feed.Heartbeat, &feed.Active, &feed.CreatedAt, &feed.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, feed)
	}
	return result, rows.Err()
}

func (s *Store) CreatePriceSnapshot(ctx context.Context, snap pricefeed.Snapshot) (pricefeed.Snapshot, error) {
	if snap.ID == "" {
		snap.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	snap.CreatedAt = now
	if snap.CollectedAt.IsZero() {
		snap.CollectedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_price_feed_snapshots (id, feed_id, price, source, collected_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, snap.ID, snap.FeedID, snap.Price, snap.Source, snap.CollectedAt, snap.CreatedAt)
	if err != nil {
		return pricefeed.Snapshot{}, err
	}
	return snap, nil
}

func (s *Store) ListPriceSnapshots(ctx context.Context, feedID string) ([]pricefeed.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, feed_id, price, source, collected_at, created_at
		FROM app_price_feed_snapshots
		WHERE feed_id = $1
		ORDER BY collected_at DESC
	`, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []pricefeed.Snapshot
	for rows.Next() {
		var snap pricefeed.Snapshot
		if err := rows.Scan(&snap.ID, &snap.FeedID, &snap.Price, &snap.Source, &snap.CollectedAt, &snap.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, snap)
	}
	return result, rows.Err()
}

func (s *Store) CreatePriceRound(ctx context.Context, round pricefeed.Round) (pricefeed.Round, error) {
	if round.ID == "" {
		round.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if round.CreatedAt.IsZero() {
		round.CreatedAt = now
	}
	if round.StartedAt.IsZero() {
		round.StartedAt = now
	}
	if round.ClosedAt.IsZero() {
		round.ClosedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_price_feed_rounds (id, feed_id, round_id, aggregated_price, observation_count, started_at, closed_at, finalized, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, round.ID, round.FeedID, round.RoundID, round.AggregatedPrice, round.ObservationCount, round.StartedAt, round.ClosedAt, round.Finalized, round.CreatedAt)
	if err != nil {
		return pricefeed.Round{}, err
	}
	return round, nil
}

func (s *Store) GetLatestPriceRound(ctx context.Context, feedID string) (pricefeed.Round, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, feed_id, round_id, aggregated_price, observation_count, started_at, closed_at, finalized, created_at
		FROM app_price_feed_rounds
		WHERE feed_id = $1
		ORDER BY round_id DESC
		LIMIT 1
	`, feedID)

	var round pricefeed.Round
	if err := row.Scan(&round.ID, &round.FeedID, &round.RoundID, &round.AggregatedPrice, &round.ObservationCount, &round.StartedAt, &round.ClosedAt, &round.Finalized, &round.CreatedAt); err != nil {
		return pricefeed.Round{}, err
	}
	return round, nil
}

func (s *Store) ListPriceRounds(ctx context.Context, feedID string, limit int) ([]pricefeed.Round, error) {
	if limit <= 0 {
		limit = math.MaxInt32
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, feed_id, round_id, aggregated_price, observation_count, started_at, closed_at, finalized, created_at
		FROM app_price_feed_rounds
		WHERE feed_id = $1
		ORDER BY round_id DESC
		LIMIT $2
	`, feedID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []pricefeed.Round
	for rows.Next() {
		var round pricefeed.Round
		if err := rows.Scan(&round.ID, &round.FeedID, &round.RoundID, &round.AggregatedPrice, &round.ObservationCount, &round.StartedAt, &round.ClosedAt, &round.Finalized, &round.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, round)
	}
	return result, rows.Err()
}

func (s *Store) UpdatePriceRound(ctx context.Context, round pricefeed.Round) (pricefeed.Round, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, feed_id, round_id, aggregated_price, observation_count, started_at, closed_at, finalized, created_at
		FROM app_price_feed_rounds
		WHERE feed_id = $1 AND round_id = $2
	`, round.FeedID, round.RoundID)

	var existing pricefeed.Round
	if err := row.Scan(&existing.ID, &existing.FeedID, &existing.RoundID, &existing.AggregatedPrice, &existing.ObservationCount, &existing.StartedAt, &existing.ClosedAt, &existing.Finalized, &existing.CreatedAt); err != nil {
		return pricefeed.Round{}, err
	}

	round.ID = existing.ID
	round.CreatedAt = existing.CreatedAt

	_, err := s.db.ExecContext(ctx, `
		UPDATE app_price_feed_rounds
		SET aggregated_price = $3,
			observation_count = $4,
			started_at = $5,
			closed_at = $6,
			finalized = $7
		WHERE feed_id = $1 AND round_id = $2
	`, round.FeedID, round.RoundID, round.AggregatedPrice, round.ObservationCount, round.StartedAt, round.ClosedAt, round.Finalized)
	if err != nil {
		return pricefeed.Round{}, err
	}
	return round, nil
}

func (s *Store) CreatePriceObservation(ctx context.Context, obs pricefeed.Observation) (pricefeed.Observation, error) {
	if obs.ID == "" {
		obs.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	if obs.CollectedAt.IsZero() {
		obs.CollectedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_price_feed_observations (id, feed_id, round_id, source, price, collected_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, obs.ID, obs.FeedID, obs.RoundID, obs.Source, obs.Price, obs.CollectedAt, obs.CreatedAt)
	if err != nil {
		return pricefeed.Observation{}, err
	}
	return obs, nil
}

func (s *Store) ListPriceObservations(ctx context.Context, feedID string, roundID int64, limit int) ([]pricefeed.Observation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, feed_id, round_id, source, price, collected_at, created_at
		FROM app_price_feed_observations
		WHERE feed_id = $1 AND round_id = $2
		ORDER BY created_at
		LIMIT $3
	`, feedID, roundID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []pricefeed.Observation
	for rows.Next() {
		var obs pricefeed.Observation
		if err := rows.Scan(&obs.ID, &obs.FeedID, &obs.RoundID, &obs.Source, &obs.Price, &obs.CollectedAt, &obs.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, obs)
	}
	return result, rows.Err()
}
