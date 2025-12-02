// Package vrf provides the VRF Service as a ServicePackage.
package vrf

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
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

// NewPostgresStore creates a new PostgreSQL-backed VRF store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

// CreateKey creates a new VRF key.
func (s *PostgresStore) CreateKey(ctx context.Context, key Key) (Key, error) {
	if key.ID == "" {
		key.ID = uuid.NewString()
	}
	key.WalletAddress = normalizeWallet(key.WalletAddress)
	now := time.Now().UTC()
	key.CreatedAt = now
	key.UpdatedAt = now
	tenant := s.accounts.AccountTenant(ctx, key.AccountID)

	metaJSON, err := json.Marshal(key.Metadata)
	if err != nil {
		return Key{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_vrf_keys (id, account_id, public_key, label, status, wallet_address, attestation, metadata, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, key.ID, key.AccountID, key.PublicKey, key.Label, key.Status, key.WalletAddress, key.Attestation, metaJSON, tenant, key.CreatedAt, key.UpdatedAt)
	if err != nil {
		return Key{}, err
	}
	return key, nil
}

// UpdateKey updates an existing VRF key.
func (s *PostgresStore) UpdateKey(ctx context.Context, key Key) (Key, error) {
	existing, err := s.GetKey(ctx, key.ID)
	if err != nil {
		return Key{}, err
	}
	key.AccountID = existing.AccountID
	key.WalletAddress = normalizeWallet(key.WalletAddress)
	key.CreatedAt = existing.CreatedAt
	key.UpdatedAt = time.Now().UTC()
	tenant := s.accounts.AccountTenant(ctx, key.AccountID)

	metaJSON, err := json.Marshal(key.Metadata)
	if err != nil {
		return Key{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_vrf_keys
		SET public_key = $2, label = $3, status = $4, wallet_address = $5, attestation = $6, metadata = $7, tenant = $8, updated_at = $9
		WHERE id = $1
	`, key.ID, key.PublicKey, key.Label, key.Status, key.WalletAddress, key.Attestation, metaJSON, tenant, key.UpdatedAt)
	if err != nil {
		return Key{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Key{}, sql.ErrNoRows
	}
	return key, nil
}

// GetKey retrieves a VRF key by ID.
func (s *PostgresStore) GetKey(ctx context.Context, id string) (Key, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, public_key, label, status, wallet_address, attestation, metadata, created_at, updated_at
		FROM app_vrf_keys
		WHERE id = $1
	`, id)
	return s.scanKey(row)
}

// ListKeys lists VRF keys for an account.
func (s *PostgresStore) ListKeys(ctx context.Context, accountID string) ([]Key, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, public_key, label, status, wallet_address, attestation, metadata, created_at, updated_at
		FROM app_vrf_keys
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Key
	for rows.Next() {
		k, err := s.scanKey(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

// CreateRequest creates a new VRF request.
func (s *PostgresStore) CreateRequest(ctx context.Context, req Request) (Request, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now
	tenant := s.accounts.AccountTenant(ctx, req.AccountID)

	metaJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return Request{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_vrf_requests (id, account_id, key_id, consumer, seed, status, result, error, metadata, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, req.ID, req.AccountID, req.KeyID, req.Consumer, req.Seed, req.Status, req.Result, req.Error, metaJSON, tenant, req.CreatedAt, req.UpdatedAt)
	if err != nil {
		return Request{}, err
	}
	return req, nil
}

// GetRequest retrieves a VRF request by ID.
func (s *PostgresStore) GetRequest(ctx context.Context, id string) (Request, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, key_id, consumer, seed, status, result, error, metadata, created_at, updated_at
		FROM app_vrf_requests
		WHERE id = $1
	`, id)
	return s.scanRequest(row)
}

// ListRequests lists VRF requests for an account.
func (s *PostgresStore) ListRequests(ctx context.Context, accountID string, limit int) ([]Request, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, key_id, consumer, seed, status, result, error, metadata, created_at, updated_at
		FROM app_vrf_requests
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, accountID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Request
	for rows.Next() {
		req, err := s.scanRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, req)
	}
	return result, rows.Err()
}


func (s *PostgresStore) scanKey(scanner core.RowScanner) (Key, error) {
	var (
		key       Key
		metaJSON  []byte
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&key.ID, &key.AccountID, &key.PublicKey, &key.Label, &key.Status, &key.WalletAddress, &key.Attestation, &metaJSON, &createdAt, &updatedAt); err != nil {
		return Key{}, err
	}
	key.WalletAddress = normalizeWallet(key.WalletAddress)
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &key.Metadata)
	}
	key.CreatedAt = createdAt.UTC()
	key.UpdatedAt = updatedAt.UTC()
	return key, nil
}

func (s *PostgresStore) scanRequest(scanner core.RowScanner) (Request, error) {
	var (
		req       Request
		metaJSON  []byte
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&req.ID, &req.AccountID, &req.KeyID, &req.Consumer, &req.Seed, &req.Status, &req.Result, &req.Error, &metaJSON, &createdAt, &updatedAt); err != nil {
		return Request{}, err
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &req.Metadata)
	}
	req.CreatedAt = createdAt.UTC()
	req.UpdatedAt = updatedAt.UTC()
	return req, nil
}

func normalizeWallet(wallet string) string {
	return strings.ToLower(strings.TrimSpace(wallet))
}
