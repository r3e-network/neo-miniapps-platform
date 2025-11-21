package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domaindf "github.com/R3E-Network/service_layer/internal/app/domain/datafeeds"
	"github.com/google/uuid"
)

// --- DataFeedStore ----------------------------------------------------------

func (s *Store) CreateDataFeed(ctx context.Context, feed domaindf.Feed) (domaindf.Feed, error) {
	if feed.ID == "" {
		feed.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	feed.CreatedAt = now
	feed.UpdatedAt = now

	metaJSON, err := json.Marshal(feed.Metadata)
	if err != nil {
		return domaindf.Feed{}, err
	}
	tagsJSON, err := json.Marshal(feed.Tags)
	if err != nil {
		return domaindf.Feed{}, err
	}
	signerJSON, err := json.Marshal(feed.SignerSet)
	if err != nil {
		return domaindf.Feed{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_data_feeds
			(id, account_id, pair, description, decimals, heartbeat_seconds, threshold_ppm, signer_set, metadata, tags, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, feed.ID, feed.AccountID, feed.Pair, feed.Description, feed.Decimals, int64(feed.Heartbeat/time.Second), feed.ThresholdPPM, signerJSON, metaJSON, tagsJSON, feed.CreatedAt, feed.UpdatedAt)
	if err != nil {
		return domaindf.Feed{}, err
	}
	return feed, nil
}

func (s *Store) UpdateDataFeed(ctx context.Context, feed domaindf.Feed) (domaindf.Feed, error) {
	existing, err := s.GetDataFeed(ctx, feed.ID)
	if err != nil {
		return domaindf.Feed{}, err
	}
	feed.CreatedAt = existing.CreatedAt
	feed.UpdatedAt = time.Now().UTC()

	metaJSON, err := json.Marshal(feed.Metadata)
	if err != nil {
		return domaindf.Feed{}, err
	}
	tagsJSON, err := json.Marshal(feed.Tags)
	if err != nil {
		return domaindf.Feed{}, err
	}
	signerJSON, err := json.Marshal(feed.SignerSet)
	if err != nil {
		return domaindf.Feed{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE chainlink_data_feeds
		SET pair = $2, description = $3, decimals = $4, heartbeat_seconds = $5, threshold_ppm = $6, signer_set = $7, metadata = $8, tags = $9, updated_at = $10
		WHERE id = $1
	`, feed.ID, feed.Pair, feed.Description, feed.Decimals, int64(feed.Heartbeat/time.Second), feed.ThresholdPPM, signerJSON, metaJSON, tagsJSON, feed.UpdatedAt)
	if err != nil {
		return domaindf.Feed{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domaindf.Feed{}, sql.ErrNoRows
	}
	return feed, nil
}

func (s *Store) GetDataFeed(ctx context.Context, id string) (domaindf.Feed, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, pair, description, decimals, heartbeat_seconds, threshold_ppm, signer_set, metadata, tags, created_at, updated_at
		FROM chainlink_data_feeds
		WHERE id = $1
	`, id)
	return scanDataFeed(row)
}

func (s *Store) ListDataFeeds(ctx context.Context, accountID string) ([]domaindf.Feed, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, pair, description, decimals, heartbeat_seconds, threshold_ppm, signer_set, metadata, tags, created_at, updated_at
		FROM chainlink_data_feeds
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []domaindf.Feed
	for rows.Next() {
		feed, err := scanDataFeed(rows)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

func (s *Store) CreateDataFeedUpdate(ctx context.Context, upd domaindf.Update) (domaindf.Update, error) {
	if upd.ID == "" {
		upd.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	upd.CreatedAt = now
	upd.UpdatedAt = now

	metaJSON, err := json.Marshal(upd.Metadata)
	if err != nil {
		return domaindf.Update{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_data_feed_updates
			(id, feed_id, account_id, round_id, price, signer, ts, signature, status, error, metadata, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, upd.ID, upd.FeedID, upd.AccountID, upd.RoundID, upd.Price, upd.Signer, upd.Timestamp, upd.Signature, upd.Status, upd.Error, metaJSON, upd.CreatedAt, upd.UpdatedAt)
	if err != nil {
		return domaindf.Update{}, err
	}
	return upd, nil
}

func (s *Store) ListDataFeedUpdates(ctx context.Context, feedID string, limit int) ([]domaindf.Update, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, feed_id, account_id, round_id, price, signer, ts, signature, status, error, metadata, created_at, updated_at
		FROM chainlink_data_feed_updates
		WHERE feed_id = $1
		ORDER BY round_id DESC
		LIMIT $2
	`, feedID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []domaindf.Update
	for rows.Next() {
		upd, err := scanDataFeedUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, upd)
	}
	return updates, rows.Err()
}

func (s *Store) ListDataFeedUpdatesByRound(ctx context.Context, feedID string, roundID int64) ([]domaindf.Update, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, feed_id, account_id, round_id, price, signer, ts, signature, status, error, metadata, created_at, updated_at
		FROM chainlink_data_feed_updates
		WHERE feed_id = $1 AND round_id = $2
		ORDER BY created_at ASC
	`, feedID, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []domaindf.Update
	for rows.Next() {
		upd, err := scanDataFeedUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, upd)
	}
	return updates, rows.Err()
}

func (s *Store) GetLatestDataFeedUpdate(ctx context.Context, feedID string) (domaindf.Update, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, feed_id, account_id, round_id, price, signer, ts, signature, status, error, metadata, created_at, updated_at
		FROM chainlink_data_feed_updates
		WHERE feed_id = $1
		ORDER BY round_id DESC
		LIMIT 1
	`, feedID)
	return scanDataFeedUpdate(row)
}

func scanDataFeed(scanner rowScanner) (domaindf.Feed, error) {
	var (
		feed               domaindf.Feed
		heartbeatSeconds   int64
		signerRaw, metaRaw []byte
		tagsRaw            []byte
	)
	if err := scanner.Scan(&feed.ID, &feed.AccountID, &feed.Pair, &feed.Description, &feed.Decimals, &heartbeatSeconds, &feed.ThresholdPPM, &signerRaw, &metaRaw, &tagsRaw, &feed.CreatedAt, &feed.UpdatedAt); err != nil {
		return domaindf.Feed{}, err
	}
	feed.Heartbeat = time.Duration(heartbeatSeconds) * time.Second
	if len(signerRaw) > 0 {
		_ = json.Unmarshal(signerRaw, &feed.SignerSet)
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &feed.Metadata)
	}
	if len(tagsRaw) > 0 {
		_ = json.Unmarshal(tagsRaw, &feed.Tags)
	}
	return feed, nil
}

func scanDataFeedUpdate(scanner rowScanner) (domaindf.Update, error) {
	var (
		upd     domaindf.Update
		metaRaw []byte
		ts      time.Time
	)
	if err := scanner.Scan(&upd.ID, &upd.FeedID, &upd.AccountID, &upd.RoundID, &upd.Price, &upd.Signer, &ts, &upd.Signature, &upd.Status, &upd.Error, &metaRaw, &upd.CreatedAt, &upd.UpdatedAt); err != nil {
		return domaindf.Update{}, err
	}
	upd.Timestamp = ts.UTC()
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &upd.Metadata)
	}
	return upd, nil
}
