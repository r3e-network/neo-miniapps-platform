package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domainlink "github.com/R3E-Network/service_layer/internal/app/domain/datalink"
	"github.com/google/uuid"
)

// --- DataLinkStore ---------------------------------------------------------

func (s *Store) CreateChannel(ctx context.Context, ch domainlink.Channel) (domainlink.Channel, error) {
	if ch.ID == "" {
		ch.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	ch.CreatedAt = now
	ch.UpdatedAt = now

	metaJSON, err := json.Marshal(ch.Metadata)
	if err != nil {
		return domainlink.Channel{}, err
	}
	signerJSON, err := json.Marshal(ch.SignerSet)
	if err != nil {
		return domainlink.Channel{}, err
	}
	tenant := s.accountTenant(ctx, ch.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_datalink_channels
			(id, account_id, name, endpoint, auth_token, signer_set, status, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, ch.ID, ch.AccountID, ch.Name, ch.Endpoint, ch.AuthToken, signerJSON, ch.Status, metaJSON, tenant, ch.CreatedAt, ch.UpdatedAt)
	if err != nil {
		return domainlink.Channel{}, err
	}
	return ch, nil
}

func (s *Store) UpdateChannel(ctx context.Context, ch domainlink.Channel) (domainlink.Channel, error) {
	existing, err := s.GetChannel(ctx, ch.ID)
	if err != nil {
		return domainlink.Channel{}, err
	}
	ch.CreatedAt = existing.CreatedAt
	ch.UpdatedAt = time.Now().UTC()

	metaJSON, err := json.Marshal(ch.Metadata)
	if err != nil {
		return domainlink.Channel{}, err
	}
	signerJSON, err := json.Marshal(ch.SignerSet)
	if err != nil {
		return domainlink.Channel{}, err
	}
	tenant := s.accountTenant(ctx, ch.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE chainlink_datalink_channels
		SET name = $2, endpoint = $3, auth_token = $4, signer_set = $5, status = $6, metadata = $7, tenant = $8, updated_at = $9
		WHERE id = $1
	`, ch.ID, ch.Name, ch.Endpoint, ch.AuthToken, signerJSON, ch.Status, metaJSON, tenant, ch.UpdatedAt)
	if err != nil {
		return domainlink.Channel{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domainlink.Channel{}, sql.ErrNoRows
	}
	return ch, nil
}

func (s *Store) GetChannel(ctx context.Context, id string) (domainlink.Channel, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, endpoint, auth_token, signer_set, status, metadata, created_at, updated_at
		FROM chainlink_datalink_channels
		WHERE id = $1
	`, id)
	return scanDataLinkChannel(row)
}

func (s *Store) ListChannels(ctx context.Context, accountID string) ([]domainlink.Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, endpoint, auth_token, signer_set, status, metadata, created_at, updated_at
		FROM chainlink_datalink_channels
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domainlink.Channel
	for rows.Next() {
		ch, err := scanDataLinkChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (s *Store) CreateDelivery(ctx context.Context, del domainlink.Delivery) (domainlink.Delivery, error) {
	if del.ID == "" {
		del.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	del.CreatedAt = now
	del.UpdatedAt = now

	payloadJSON, err := json.Marshal(del.Payload)
	if err != nil {
		return domainlink.Delivery{}, err
	}
	metaJSON, err := json.Marshal(del.Metadata)
	if err != nil {
		return domainlink.Delivery{}, err
	}
	tenant := s.accountTenant(ctx, del.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_datalink_deliveries
			(id, account_id, channel_id, payload, attempts, status, error, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, del.ID, del.AccountID, del.ChannelID, payloadJSON, del.Attempts, del.Status, del.Error, metaJSON, tenant, del.CreatedAt, del.UpdatedAt)
	if err != nil {
		return domainlink.Delivery{}, err
	}
	return del, nil
}

func (s *Store) GetDelivery(ctx context.Context, id string) (domainlink.Delivery, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, channel_id, payload, attempts, status, error, metadata, created_at, updated_at
		FROM chainlink_datalink_deliveries
		WHERE id = $1
	`, id)
	return scanDataLinkDelivery(row)
}

func (s *Store) ListDeliveries(ctx context.Context, accountID string, limit int) ([]domainlink.Delivery, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, channel_id, payload, attempts, status, error, metadata, created_at, updated_at
		FROM chainlink_datalink_deliveries
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dels []domainlink.Delivery
	for rows.Next() {
		del, err := scanDataLinkDelivery(rows)
		if err != nil {
			return nil, err
		}
		dels = append(dels, del)
	}
	return dels, rows.Err()
}

func scanDataLinkChannel(scanner rowScanner) (domainlink.Channel, error) {
	var (
		ch        domainlink.Channel
		metaRaw   []byte
		signerRaw []byte
	)
	if err := scanner.Scan(&ch.ID, &ch.AccountID, &ch.Name, &ch.Endpoint, &ch.AuthToken, &signerRaw, &ch.Status, &metaRaw, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		return domainlink.Channel{}, err
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &ch.Metadata)
	}
	if len(signerRaw) > 0 {
		_ = json.Unmarshal(signerRaw, &ch.SignerSet)
	}
	return ch, nil
}

func scanDataLinkDelivery(scanner rowScanner) (domainlink.Delivery, error) {
	var (
		del     domainlink.Delivery
		payload []byte
		metaRaw []byte
	)
	if err := scanner.Scan(&del.ID, &del.AccountID, &del.ChannelID, &payload, &del.Attempts, &del.Status, &del.Error, &metaRaw, &del.CreatedAt, &del.UpdatedAt); err != nil {
		return domainlink.Delivery{}, err
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &del.Payload)
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &del.Metadata)
	}
	return del, nil
}
