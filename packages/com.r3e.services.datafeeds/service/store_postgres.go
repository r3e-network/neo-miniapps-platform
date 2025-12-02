package datafeeds

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// --- DataFeedStore ----------------------------------------------------------

// PostgresStore implements Store using PostgreSQL.
// This is the service-local store implementation that uses the generic
// database connection provided by the Service Engine.
type PostgresStore struct {
	db       *sql.DB
	accounts AccountChecker
}

// NewPostgresStore creates a new PostgreSQL-backed store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

func (s *PostgresStore) accountTenant(ctx context.Context, accountID string) string {
	return s.accounts.AccountTenant(ctx, accountID)
}


func (s *PostgresStore) CreateDataFeed(ctx context.Context, feed Feed) (Feed, error) {
	if feed.ID == "" {
		feed.ID = uuid.NewString()
	}
	if strings.TrimSpace(feed.Aggregation) == "" {
		feed.Aggregation = "median"
	}
	now := time.Now().UTC()
	feed.CreatedAt = now
	feed.UpdatedAt = now
	tenant := s.accountTenant(ctx, feed.AccountID)

	metaJSON, err := json.Marshal(feed.Metadata)
	if err != nil {
		return Feed{}, err
	}
	tagsJSON, err := json.Marshal(feed.Tags)
	if err != nil {
		return Feed{}, err
	}
	signerJSON, err := json.Marshal(feed.SignerSet)
	if err != nil {
		return Feed{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_data_feeds
			(id, account_id, pair, description, decimals, heartbeat_seconds, threshold_ppm, signer_set, aggregation, metadata, tags, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, feed.ID, feed.AccountID, feed.Pair, feed.Description, feed.Decimals, int64(feed.Heartbeat/time.Second), feed.ThresholdPPM, signerJSON, feed.Aggregation, metaJSON, tagsJSON, tenant, feed.CreatedAt, feed.UpdatedAt)
	if err != nil {
		return Feed{}, err
	}
	return feed, nil
}

func (s *PostgresStore) UpdateDataFeed(ctx context.Context, feed Feed) (Feed, error) {
	existing, err := s.GetDataFeed(ctx, feed.ID)
	if err != nil {
		return Feed{}, err
	}
	if strings.TrimSpace(feed.Aggregation) == "" {
		feed.Aggregation = existing.Aggregation
	}
	feed.CreatedAt = existing.CreatedAt
	feed.UpdatedAt = time.Now().UTC()

	metaJSON, err := json.Marshal(feed.Metadata)
	if err != nil {
		return Feed{}, err
	}
	tagsJSON, err := json.Marshal(feed.Tags)
	if err != nil {
		return Feed{}, err
	}
	signerJSON, err := json.Marshal(feed.SignerSet)
	if err != nil {
		return Feed{}, err
	}
	tenant := s.accountTenant(ctx, feed.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE chainlink_data_feeds
		SET pair = $2, description = $3, decimals = $4, heartbeat_seconds = $5, threshold_ppm = $6, signer_set = $7, aggregation = $8, metadata = $9, tags = $10, tenant = $11, updated_at = $12
		WHERE id = $1
	`, feed.ID, feed.Pair, feed.Description, feed.Decimals, int64(feed.Heartbeat/time.Second), feed.ThresholdPPM, signerJSON, feed.Aggregation, metaJSON, tagsJSON, tenant, feed.UpdatedAt)
	if err != nil {
		return Feed{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Feed{}, sql.ErrNoRows
	}
	return feed, nil
}

func (s *PostgresStore) GetDataFeed(ctx context.Context, id string) (Feed, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, pair, description, decimals, heartbeat_seconds, threshold_ppm, signer_set, aggregation, metadata, tags, created_at, updated_at
		FROM chainlink_data_feeds
		WHERE id = $1
	`, id)
	return scanDataFeed(row)
}

func (s *PostgresStore) ListDataFeeds(ctx context.Context, accountID string) ([]Feed, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, pair, description, decimals, heartbeat_seconds, threshold_ppm, signer_set, aggregation, metadata, tags, created_at, updated_at
		FROM chainlink_data_feeds
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		feed, err := scanDataFeed(rows)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

func (s *PostgresStore) CreateDataFeedUpdate(ctx context.Context, upd Update) (Update, error) {
	if upd.ID == "" {
		upd.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	upd.CreatedAt = now
	upd.UpdatedAt = now
	tenant := s.accountTenant(ctx, upd.AccountID)

	metaJSON, err := json.Marshal(upd.Metadata)
	if err != nil {
		return Update{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_data_feed_updates
			(id, feed_id, account_id, round_id, price, signer, ts, signature, status, error, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, upd.ID, upd.FeedID, upd.AccountID, upd.RoundID, upd.Value, upd.Signer, upd.Timestamp, upd.Signature, upd.Status, upd.Error, metaJSON, tenant, upd.CreatedAt, upd.UpdatedAt)
	if err != nil {
		return Update{}, err
	}
	return upd, nil
}

func (s *PostgresStore) ListDataFeedUpdates(ctx context.Context, feedID string, limit int) ([]Update, error) {
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

	var updates []Update
	for rows.Next() {
		upd, err := scanDataFeedUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, upd)
	}
	return updates, rows.Err()
}

func (s *PostgresStore) ListDataFeedUpdatesByRound(ctx context.Context, feedID string, roundID int64) ([]Update, error) {
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

	var updates []Update
	for rows.Next() {
		upd, err := scanDataFeedUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, upd)
	}
	return updates, rows.Err()
}

func (s *PostgresStore) GetLatestDataFeedUpdate(ctx context.Context, feedID string) (Update, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, feed_id, account_id, round_id, price, signer, ts, signature, status, error, metadata, created_at, updated_at
		FROM chainlink_data_feed_updates
		WHERE feed_id = $1
		ORDER BY round_id DESC
		LIMIT 1
	`, feedID)
	return scanDataFeedUpdate(row)
}

func scanDataFeed(scanner core.RowScanner) (Feed, error) {
	var (
		feed               Feed
		heartbeatSeconds   int64
		signerRaw, metaRaw []byte
		tagsRaw            []byte
	)
	if err := scanner.Scan(&feed.ID, &feed.AccountID, &feed.Pair, &feed.Description, &feed.Decimals, &heartbeatSeconds, &feed.ThresholdPPM, &signerRaw, &feed.Aggregation, &metaRaw, &tagsRaw, &feed.CreatedAt, &feed.UpdatedAt); err != nil {
		return Feed{}, err
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

func scanDataFeedUpdate(scanner core.RowScanner) (Update, error) {
	var (
		upd     Update
		metaRaw []byte
		ts      time.Time
	)
	if err := scanner.Scan(&upd.ID, &upd.FeedID, &upd.AccountID, &upd.RoundID, &upd.Value, &upd.Signer, &ts, &upd.Signature, &upd.Status, &upd.Error, &metaRaw, &upd.CreatedAt, &upd.UpdatedAt); err != nil {
		return Update{}, err
	}
	upd.Timestamp = ts.UTC()
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &upd.Metadata)
	}
	return upd, nil
}
