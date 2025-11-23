package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/app/domain/vrf"
)

// VRFStore implementation

func (s *Store) CreateVRFKey(ctx context.Context, key vrf.Key) (vrf.Key, error) {
	if key.ID == "" {
		key.ID = uuid.NewString()
	}
	key.WalletAddress = normalizeWallet(key.WalletAddress)
	now := time.Now().UTC()
	key.CreatedAt = now
	key.UpdatedAt = now
	tenant := s.accountTenant(ctx, key.AccountID)

	metaJSON, err := json.Marshal(key.Metadata)
	if err != nil {
		return vrf.Key{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_vrf_keys (id, account_id, public_key, label, status, wallet_address, attestation, metadata, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, key.ID, key.AccountID, key.PublicKey, key.Label, key.Status, key.WalletAddress, key.Attestation, metaJSON, tenant, key.CreatedAt, key.UpdatedAt)
	if err != nil {
		return vrf.Key{}, err
	}
	return key, nil
}

func (s *Store) UpdateVRFKey(ctx context.Context, key vrf.Key) (vrf.Key, error) {
	existing, err := s.GetVRFKey(ctx, key.ID)
	if err != nil {
		return vrf.Key{}, err
	}
	key.AccountID = existing.AccountID
	key.WalletAddress = normalizeWallet(key.WalletAddress)
	key.CreatedAt = existing.CreatedAt
	key.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, key.AccountID)

	metaJSON, err := json.Marshal(key.Metadata)
	if err != nil {
		return vrf.Key{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_vrf_keys
		SET public_key = $2, label = $3, status = $4, wallet_address = $5, attestation = $6, metadata = $7, tenant = $8, updated_at = $9
		WHERE id = $1
	`, key.ID, key.PublicKey, key.Label, key.Status, key.WalletAddress, key.Attestation, metaJSON, tenant, key.UpdatedAt)
	if err != nil {
		return vrf.Key{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return vrf.Key{}, sql.ErrNoRows
	}
	return key, nil
}

func (s *Store) GetVRFKey(ctx context.Context, id string) (vrf.Key, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, public_key, label, status, wallet_address, attestation, metadata, created_at, updated_at
		FROM app_vrf_keys
		WHERE id = $1
	`, id)
	return scanVRFKey(row)
}

func (s *Store) ListVRFKeys(ctx context.Context, accountID string) ([]vrf.Key, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, public_key, label, status, wallet_address, attestation, metadata, created_at, updated_at
		FROM app_vrf_keys
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []vrf.Key
	for rows.Next() {
		k, err := scanVRFKey(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

func (s *Store) CreateVRFRequest(ctx context.Context, req vrf.Request) (vrf.Request, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now
	tenant := s.accountTenant(ctx, req.AccountID)

	metaJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return vrf.Request{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_vrf_requests (id, account_id, key_id, consumer, seed, status, result, error, metadata, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, req.ID, req.AccountID, req.KeyID, req.Consumer, req.Seed, req.Status, req.Result, req.Error, metaJSON, tenant, req.CreatedAt, req.UpdatedAt)
	if err != nil {
		return vrf.Request{}, err
	}
	return req, nil
}

func (s *Store) GetVRFRequest(ctx context.Context, id string) (vrf.Request, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, key_id, consumer, seed, status, result, error, metadata, created_at, updated_at
		FROM app_vrf_requests
		WHERE id = $1
	`, id)
	return scanVRFRequest(row)
}

func (s *Store) ListVRFRequests(ctx context.Context, accountID string, limit int) ([]vrf.Request, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, key_id, consumer, seed, status, result, error, metadata, created_at, updated_at
		FROM app_vrf_requests
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []vrf.Request
	for rows.Next() {
		req, err := scanVRFRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

func scanVRFKey(scanner rowScanner) (vrf.Key, error) {
	var (
		key       vrf.Key
		metaJSON  []byte
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&key.ID, &key.AccountID, &key.PublicKey, &key.Label, &key.Status, &key.WalletAddress, &key.Attestation, &metaJSON, &createdAt, &updatedAt); err != nil {
		return vrf.Key{}, err
	}
	key.WalletAddress = normalizeWallet(key.WalletAddress)
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &key.Metadata)
	}
	key.CreatedAt = createdAt.UTC()
	key.UpdatedAt = updatedAt.UTC()
	return key, nil
}

func scanVRFRequest(scanner rowScanner) (vrf.Request, error) {
	var (
		req       vrf.Request
		metaJSON  []byte
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&req.ID, &req.AccountID, &req.KeyID, &req.Consumer, &req.Seed, &req.Status, &req.Result, &req.Error, &metaJSON, &createdAt, &updatedAt); err != nil {
		return vrf.Request{}, err
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &req.Metadata)
	}
	req.CreatedAt = createdAt.UTC()
	req.UpdatedAt = updatedAt.UTC()
	return req, nil
}
