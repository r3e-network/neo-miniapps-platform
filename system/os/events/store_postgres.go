// Package events provides PostgreSQL implementation of RequestStore.
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// PostgresRequestStore implements RequestStore using PostgreSQL.
type PostgresRequestStore struct {
	db *sql.DB
}

// NewPostgresRequestStore creates a new PostgreSQL request store.
func NewPostgresRequestStore(db *sql.DB) *PostgresRequestStore {
	return &PostgresRequestStore{db: db}
}

// EnsureSchema creates the required tables if they don't exist.
func (s *PostgresRequestStore) EnsureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS service_requests (
			id TEXT PRIMARY KEY,
			external_id TEXT,
			account_id TEXT NOT NULL,
			service_type TEXT NOT NULL,
			service_id TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			payload JSONB,
			result JSONB,
			error TEXT,
			fee BIGINT DEFAULT 0,
			fee_id TEXT,
			tx_hash TEXT,
			callback_hash TEXT,
			metadata JSONB,
			attempts INTEGER DEFAULT 0,
			max_attempts INTEGER DEFAULT 3,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			completed_at TIMESTAMPTZ
		);

		CREATE INDEX IF NOT EXISTS idx_service_requests_account_id ON service_requests(account_id);
		CREATE INDEX IF NOT EXISTS idx_service_requests_external_id ON service_requests(external_id) WHERE external_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_service_requests_status ON service_requests(status);
		CREATE INDEX IF NOT EXISTS idx_service_requests_service_type ON service_requests(service_type);
		CREATE INDEX IF NOT EXISTS idx_service_requests_created_at ON service_requests(created_at);
	`)
	return err
}

// Create stores a new request.
func (s *PostgresRequestStore) Create(ctx context.Context, req *Request) error {
	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	metadata, err := json.Marshal(req.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO service_requests (
			id, external_id, account_id, service_type, service_id,
			status, payload, fee, fee_id, tx_hash, callback_hash,
			metadata, attempts, max_attempts, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16
		)
	`,
		req.ID, core.ToNullString(req.ExternalID), req.AccountID, req.ServiceType, core.ToNullString(req.ServiceID),
		req.Status, payload, req.Fee, core.ToNullString(req.FeeID), core.ToNullString(req.TxHash), core.ToNullString(req.CallbackHash),
		metadata, req.Attempts, req.MaxAttempts, req.CreatedAt, req.UpdatedAt,
	)
	return err
}

// Get retrieves a request by ID.
func (s *PostgresRequestStore) Get(ctx context.Context, id string) (*Request, error) {
	return s.scanRequest(ctx, `
		SELECT id, external_id, account_id, service_type, service_id,
			status, payload, result, error, fee, fee_id, tx_hash, callback_hash,
			metadata, attempts, max_attempts, created_at, updated_at, completed_at
		FROM service_requests
		WHERE id = $1
	`, id)
}

// GetByExternalID retrieves a request by external ID.
func (s *PostgresRequestStore) GetByExternalID(ctx context.Context, externalID string) (*Request, error) {
	return s.scanRequest(ctx, `
		SELECT id, external_id, account_id, service_type, service_id,
			status, payload, result, error, fee, fee_id, tx_hash, callback_hash,
			metadata, attempts, max_attempts, created_at, updated_at, completed_at
		FROM service_requests
		WHERE external_id = $1
	`, externalID)
}

// Update updates an existing request.
func (s *PostgresRequestStore) Update(ctx context.Context, req *Request) error {
	result, err := json.Marshal(req.Result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	metadata, err := json.Marshal(req.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	var completedAt sql.NullTime
	if req.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *req.CompletedAt, Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE service_requests SET
			status = $2,
			result = $3,
			error = $4,
			fee_id = $5,
			metadata = $6,
			attempts = $7,
			updated_at = $8,
			completed_at = $9
		WHERE id = $1
	`,
		req.ID, req.Status, result, core.ToNullString(req.Error), core.ToNullString(req.FeeID),
		metadata, req.Attempts, req.UpdatedAt, completedAt,
	)
	return err
}

// List retrieves requests with filters.
func (s *PostgresRequestStore) List(ctx context.Context, accountID string, serviceType ServiceType, status RequestStatus, limit int) ([]*Request, error) {
	query := `
		SELECT id, external_id, account_id, service_type, service_id,
			status, payload, result, error, fee, fee_id, tx_hash, callback_hash,
			metadata, attempts, max_attempts, created_at, updated_at, completed_at
		FROM service_requests
		WHERE 1=1
	`
	args := []any{}
	argNum := 1

	if accountID != "" {
		query += fmt.Sprintf(" AND account_id = $%d", argNum)
		args = append(args, accountID)
		argNum++
	}

	if serviceType != "" {
		query += fmt.Sprintf(" AND service_type = $%d", argNum)
		args = append(args, serviceType)
		argNum++
	}

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, status)
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, limit)
	}

	return s.scanRequests(ctx, query, args...)
}

// ListPending retrieves pending requests for a service type.
func (s *PostgresRequestStore) ListPending(ctx context.Context, serviceType ServiceType, limit int) ([]*Request, error) {
	query := `
		SELECT id, external_id, account_id, service_type, service_id,
			status, payload, result, error, fee, fee_id, tx_hash, callback_hash,
			metadata, attempts, max_attempts, created_at, updated_at, completed_at
		FROM service_requests
		WHERE status = 'pending'
	`
	args := []any{}
	argNum := 1

	if serviceType != "" {
		query += fmt.Sprintf(" AND service_type = $%d", argNum)
		args = append(args, serviceType)
		argNum++
	}

	query += " ORDER BY created_at ASC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, limit)
	}

	return s.scanRequests(ctx, query, args...)
}

// scanRequest scans a single request from a query.
func (s *PostgresRequestStore) scanRequest(ctx context.Context, query string, args ...any) (*Request, error) {
	row := s.db.QueryRowContext(ctx, query, args...)

	var req Request
	var externalID, serviceID, feeID, txHash, callbackHash, errorStr sql.NullString
	var payload, result, metadata []byte
	var completedAt sql.NullTime

	err := row.Scan(
		&req.ID, &externalID, &req.AccountID, &req.ServiceType, &serviceID,
		&req.Status, &payload, &result, &errorStr, &req.Fee, &feeID, &txHash, &callbackHash,
		&metadata, &req.Attempts, &req.MaxAttempts, &req.CreatedAt, &req.UpdatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	req.ExternalID = externalID.String
	req.ServiceID = serviceID.String
	req.FeeID = feeID.String
	req.TxHash = txHash.String
	req.CallbackHash = callbackHash.String
	req.Error = errorStr.String

	if completedAt.Valid {
		req.CompletedAt = &completedAt.Time
	}

	if len(payload) > 0 {
		json.Unmarshal(payload, &req.Payload)
	}
	if len(result) > 0 {
		json.Unmarshal(result, &req.Result)
	}
	if len(metadata) > 0 {
		json.Unmarshal(metadata, &req.Metadata)
	}

	return &req, nil
}

// scanRequests scans multiple requests from a query.
func (s *PostgresRequestStore) scanRequests(ctx context.Context, query string, args ...any) ([]*Request, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*Request
	for rows.Next() {
		var req Request
		var externalID, serviceID, feeID, txHash, callbackHash, errorStr sql.NullString
		var payload, result, metadata []byte
		var completedAt sql.NullTime

		err := rows.Scan(
			&req.ID, &externalID, &req.AccountID, &req.ServiceType, &serviceID,
			&req.Status, &payload, &result, &errorStr, &req.Fee, &feeID, &txHash, &callbackHash,
			&metadata, &req.Attempts, &req.MaxAttempts, &req.CreatedAt, &req.UpdatedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		req.ExternalID = externalID.String
		req.ServiceID = serviceID.String
		req.FeeID = feeID.String
		req.TxHash = txHash.String
		req.CallbackHash = callbackHash.String
		req.Error = errorStr.String

		if completedAt.Valid {
			req.CompletedAt = &completedAt.Time
		}

		if len(payload) > 0 {
			json.Unmarshal(payload, &req.Payload)
		}
		if len(result) > 0 {
			json.Unmarshal(result, &req.Result)
		}
		if len(metadata) > 0 {
			json.Unmarshal(metadata, &req.Metadata)
		}

		requests = append(requests, &req)
	}

	return requests, rows.Err()
}

// Compile-time interface check
var _ RequestStore = (*PostgresRequestStore)(nil)
