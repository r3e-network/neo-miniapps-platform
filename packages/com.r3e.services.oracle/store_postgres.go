package oracle

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db       *sql.DB
	accounts AccountChecker
}

// NewPostgresStore creates a new PostgreSQL-backed oracle store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

// CreateDataSource creates a new data source.
func (s *PostgresStore) CreateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	if src.ID == "" {
		src.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	src.CreatedAt = now
	src.UpdatedAt = now

	headersJSON, err := json.Marshal(src.Headers)
	if err != nil {
		return DataSource{}, err
	}
	tenant := s.accounts.AccountTenant(ctx, src.AccountID)

	_, err = s.db.ExecContext(ctx, `
        INSERT INTO app_oracle_sources (id, account_id, name, description, url, method, headers, body, enabled, tenant, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, src.ID, src.AccountID, src.Name, src.Description, src.URL, src.Method, headersJSON, src.Body, src.Enabled, tenant, src.CreatedAt, src.UpdatedAt)
	if err != nil {
		return DataSource{}, err
	}
	return src, nil
}

// UpdateDataSource updates an existing data source.
func (s *PostgresStore) UpdateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	existing, err := s.GetDataSource(ctx, src.ID)
	if err != nil {
		return DataSource{}, err
	}

	src.AccountID = existing.AccountID
	src.CreatedAt = existing.CreatedAt
	src.UpdatedAt = time.Now().UTC()

	headersJSON, err := json.Marshal(src.Headers)
	if err != nil {
		return DataSource{}, err
	}
	tenant := s.accounts.AccountTenant(ctx, src.AccountID)

	result, err := s.db.ExecContext(ctx, `
        UPDATE app_oracle_sources
        SET name = $2, description = $3, url = $4, method = $5, headers = $6, body = $7, enabled = $8, tenant = $9, updated_at = $10
        WHERE id = $1
    `, src.ID, src.Name, src.Description, src.URL, src.Method, headersJSON, src.Body, src.Enabled, tenant, src.UpdatedAt)
	if err != nil {
		return DataSource{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return DataSource{}, sql.ErrNoRows
	}
	return src, nil
}

// GetDataSource retrieves a data source by ID.
func (s *PostgresStore) GetDataSource(ctx context.Context, id string) (DataSource, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT id, account_id, name, description, url, method, headers, body, enabled, created_at, updated_at
        FROM app_oracle_sources
        WHERE id = $1
    `, id)

	var (
		src        DataSource
		headersRaw []byte
	)
	if err := row.Scan(&src.ID, &src.AccountID, &src.Name, &src.Description, &src.URL, &src.Method, &headersRaw, &src.Body, &src.Enabled, &src.CreatedAt, &src.UpdatedAt); err != nil {
		return DataSource{}, err
	}
	if len(headersRaw) > 0 {
		_ = json.Unmarshal(headersRaw, &src.Headers)
	}
	return src, nil
}

// ListDataSources lists data sources for an account.
func (s *PostgresStore) ListDataSources(ctx context.Context, accountID string) ([]DataSource, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, account_id, name, description, url, method, headers, body, enabled, created_at, updated_at
        FROM app_oracle_sources
        WHERE ($1 = '' OR account_id = $1) AND ($2 = '' OR tenant = $2)
        ORDER BY created_at
    `, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DataSource
	for rows.Next() {
		var (
			src        DataSource
			headersRaw []byte
		)
		if err := rows.Scan(&src.ID, &src.AccountID, &src.Name, &src.Description, &src.URL, &src.Method, &headersRaw, &src.Body, &src.Enabled, &src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, err
		}
		if len(headersRaw) > 0 {
			_ = json.Unmarshal(headersRaw, &src.Headers)
		}
		result = append(result, src)
	}
	return result, rows.Err()
}

// CreateRequest creates a new oracle request.
func (s *PostgresStore) CreateRequest(ctx context.Context, req Request) (Request, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now
	tenant := s.accounts.AccountTenant(ctx, req.AccountID)

	_, err := s.db.ExecContext(ctx, `
        INSERT INTO app_oracle_requests (id, account_id, data_source_id, status, attempts, payload, result, error, tenant, created_at, updated_at, completed_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, req.ID, req.AccountID, req.DataSourceID, req.Status, req.Attempts, req.Payload, req.Result, req.Error, tenant, req.CreatedAt, req.UpdatedAt, toNullTime(req.CompletedAt))
	if err != nil {
		return Request{}, err
	}
	return req, nil
}

// UpdateRequest updates an existing oracle request.
func (s *PostgresStore) UpdateRequest(ctx context.Context, req Request) (Request, error) {
	existing, err := s.GetRequest(ctx, req.ID)
	if err != nil {
		return Request{}, err
	}

	req.AccountID = existing.AccountID
	req.DataSourceID = existing.DataSourceID
	req.CreatedAt = existing.CreatedAt
	req.UpdatedAt = time.Now().UTC()
	tenant := s.accounts.AccountTenant(ctx, req.AccountID)

	result, err := s.db.ExecContext(ctx, `
        UPDATE app_oracle_requests
        SET status = $2, attempts = $3, payload = $4, result = $5, error = $6, tenant = $7, updated_at = $8, completed_at = $9
        WHERE id = $1
    `, req.ID, req.Status, req.Attempts, req.Payload, req.Result, req.Error, tenant, req.UpdatedAt, toNullTime(req.CompletedAt))
	if err != nil {
		return Request{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Request{}, sql.ErrNoRows
	}
	return req, nil
}

// GetRequest retrieves an oracle request by ID.
func (s *PostgresStore) GetRequest(ctx context.Context, id string) (Request, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT id, account_id, data_source_id, status, attempts, payload, result, error, created_at, updated_at, completed_at
        FROM app_oracle_requests
        WHERE id = $1
    `, id)

	var (
		req         Request
		completedAt sql.NullTime
	)
	if err := row.Scan(&req.ID, &req.AccountID, &req.DataSourceID, &req.Status, &req.Attempts, &req.Payload, &req.Result, &req.Error, &req.CreatedAt, &req.UpdatedAt, &completedAt); err != nil {
		return Request{}, err
	}
	if completedAt.Valid {
		req.CompletedAt = completedAt.Time.UTC()
	}
	return req, nil
}

// ListRequests lists oracle requests for an account.
func (s *PostgresStore) ListRequests(ctx context.Context, accountID string, limit int, status string) ([]Request, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	max := limit
	if max <= 0 || max > 500 {
		max = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, data_source_id, status, attempts, payload, result, error, created_at, updated_at, completed_at
		FROM app_oracle_requests
        WHERE ($1 = '' OR account_id = $1) AND ($2 = '' OR tenant = $2) AND ($4 = '' OR status = $4)
        ORDER BY created_at DESC
        LIMIT $3
    `, accountID, tenant, max, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Request
	for rows.Next() {
		var (
			req         Request
			completedAt sql.NullTime
		)
		if err := rows.Scan(&req.ID, &req.AccountID, &req.DataSourceID, &req.Status, &req.Attempts, &req.Payload, &req.Result, &req.Error, &req.CreatedAt, &req.UpdatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			req.CompletedAt = completedAt.Time.UTC()
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

// ListPendingRequests lists all pending oracle requests.
func (s *PostgresStore) ListPendingRequests(ctx context.Context) ([]Request, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, data_source_id, status, attempts, payload, result, error, created_at, updated_at, completed_at
		FROM app_oracle_requests
		WHERE status IN ('pending','running')
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Request
	for rows.Next() {
		var (
			req         Request
			completedAt sql.NullTime
		)
		if err := rows.Scan(&req.ID, &req.AccountID, &req.DataSourceID, &req.Status, &req.Attempts, &req.Payload, &req.Result, &req.Error, &req.CreatedAt, &req.UpdatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			req.CompletedAt = completedAt.Time
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

// toNullTime converts time.Time to sql.NullTime.
func toNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}
