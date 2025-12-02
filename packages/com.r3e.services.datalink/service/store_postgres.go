// Package datalink provides the Data Link Service as a ServicePackage.
package datalink

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// PostgresStore implements Store using PostgreSQL.
// This is the service-local store implementation that uses the generic
// database connection provided by the Service Engine.
type PostgresStore struct {
	db       *sql.DB
	accounts AccountChecker
}

// NewPostgresStore creates a new PostgreSQL-backed datalink store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

// CreateChannel creates a new datalink channel.
func (s *PostgresStore) CreateChannel(ctx context.Context, ch Channel) (Channel, error) {
	if ch.ID == "" {
		ch.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	ch.CreatedAt = now
	ch.UpdatedAt = now

	metaJSON, err := json.Marshal(ch.Metadata)
	if err != nil {
		return Channel{}, err
	}
	signerJSON, err := json.Marshal(ch.SignerSet)
	if err != nil {
		return Channel{}, err
	}
	tenant := s.accounts.AccountTenant(ctx, ch.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_datalink_channels
			(id, account_id, name, endpoint, auth_token, signer_set, status, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, ch.ID, ch.AccountID, ch.Name, ch.Endpoint, ch.AuthToken, signerJSON, ch.Status, metaJSON, tenant, ch.CreatedAt, ch.UpdatedAt)
	if err != nil {
		return Channel{}, err
	}
	return ch, nil
}

// UpdateChannel updates an existing datalink channel.
func (s *PostgresStore) UpdateChannel(ctx context.Context, ch Channel) (Channel, error) {
	existing, err := s.GetChannel(ctx, ch.ID)
	if err != nil {
		return Channel{}, err
	}
	ch.CreatedAt = existing.CreatedAt
	ch.UpdatedAt = time.Now().UTC()

	metaJSON, err := json.Marshal(ch.Metadata)
	if err != nil {
		return Channel{}, err
	}
	signerJSON, err := json.Marshal(ch.SignerSet)
	if err != nil {
		return Channel{}, err
	}
	tenant := s.accounts.AccountTenant(ctx, ch.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE chainlink_datalink_channels
		SET name = $2, endpoint = $3, auth_token = $4, signer_set = $5, status = $6, metadata = $7, tenant = $8, updated_at = $9
		WHERE id = $1
	`, ch.ID, ch.Name, ch.Endpoint, ch.AuthToken, signerJSON, ch.Status, metaJSON, tenant, ch.UpdatedAt)
	if err != nil {
		return Channel{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Channel{}, sql.ErrNoRows
	}
	return ch, nil
}

// GetChannel retrieves a datalink channel by ID.
func (s *PostgresStore) GetChannel(ctx context.Context, id string) (Channel, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, endpoint, auth_token, signer_set, status, metadata, created_at, updated_at
		FROM chainlink_datalink_channels
		WHERE id = $1
	`, id)
	return s.scanDataLinkChannel(row)
}

// ListChannels lists datalink channels for an account.
func (s *PostgresStore) ListChannels(ctx context.Context, accountID string) ([]Channel, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, endpoint, auth_token, signer_set, status, metadata, created_at, updated_at
		FROM chainlink_datalink_channels
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Channel
	for rows.Next() {
		ch, err := s.scanDataLinkChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// CreateDelivery creates a new datalink delivery.
func (s *PostgresStore) CreateDelivery(ctx context.Context, del Delivery) (Delivery, error) {
	if del.ID == "" {
		del.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	del.CreatedAt = now
	del.UpdatedAt = now

	payloadJSON, err := json.Marshal(del.Payload)
	if err != nil {
		return Delivery{}, err
	}
	metaJSON, err := json.Marshal(del.Metadata)
	if err != nil {
		return Delivery{}, err
	}
	tenant := s.accounts.AccountTenant(ctx, del.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_datalink_deliveries
			(id, account_id, channel_id, payload, attempts, status, error, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, del.ID, del.AccountID, del.ChannelID, payloadJSON, del.Attempts, del.Status, del.Error, metaJSON, tenant, del.CreatedAt, del.UpdatedAt)
	if err != nil {
		return Delivery{}, err
	}
	return del, nil
}

// GetDelivery retrieves a datalink delivery by ID.
func (s *PostgresStore) GetDelivery(ctx context.Context, id string) (Delivery, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, channel_id, payload, attempts, status, error, metadata, created_at, updated_at
		FROM chainlink_datalink_deliveries
		WHERE id = $1
	`, id)
	return s.scanDataLinkDelivery(row)
}

// ListDeliveries lists datalink deliveries for an account.
func (s *PostgresStore) ListDeliveries(ctx context.Context, accountID string, limit int) ([]Delivery, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, channel_id, payload, attempts, status, error, metadata, created_at, updated_at
		FROM chainlink_datalink_deliveries
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, accountID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dels []Delivery
	for rows.Next() {
		del, err := s.scanDataLinkDelivery(rows)
		if err != nil {
			return nil, err
		}
		dels = append(dels, del)
	}
	return dels, rows.Err()
}


func (s *PostgresStore) scanDataLinkChannel(scanner core.RowScanner) (Channel, error) {
	var (
		ch        Channel
		metaRaw   []byte
		signerRaw []byte
	)
	if err := scanner.Scan(&ch.ID, &ch.AccountID, &ch.Name, &ch.Endpoint, &ch.AuthToken, &signerRaw, &ch.Status, &metaRaw, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		return Channel{}, err
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &ch.Metadata)
	}
	if len(signerRaw) > 0 {
		_ = json.Unmarshal(signerRaw, &ch.SignerSet)
	}
	return ch, nil
}

func (s *PostgresStore) scanDataLinkDelivery(scanner core.RowScanner) (Delivery, error) {
	var (
		del     Delivery
		payload []byte
		metaRaw []byte
	)
	if err := scanner.Scan(&del.ID, &del.AccountID, &del.ChannelID, &payload, &del.Attempts, &del.Status, &del.Error, &metaRaw, &del.CreatedAt, &del.UpdatedAt); err != nil {
		return Delivery{}, err
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &del.Payload)
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &del.Metadata)
	}
	return del, nil
}
