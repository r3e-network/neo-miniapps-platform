package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
)

// OracleStore implementation

func (s *Store) CreateDataSource(ctx context.Context, src oracle.DataSource) (oracle.DataSource, error) {
	if src.ID == "" {
		src.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	src.CreatedAt = now
	src.UpdatedAt = now

	headersJSON, err := json.Marshal(src.Headers)
	if err != nil {
		return oracle.DataSource{}, err
	}
	tenant := s.accountTenant(ctx, src.AccountID)

	_, err = s.db.ExecContext(ctx, `
        INSERT INTO app_oracle_sources (id, account_id, name, description, url, method, headers, body, enabled, tenant, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, src.ID, src.AccountID, src.Name, src.Description, src.URL, src.Method, headersJSON, src.Body, src.Enabled, tenant, src.CreatedAt, src.UpdatedAt)
	if err != nil {
		return oracle.DataSource{}, err
	}
	return src, nil
}

func (s *Store) UpdateDataSource(ctx context.Context, src oracle.DataSource) (oracle.DataSource, error) {
	existing, err := s.GetDataSource(ctx, src.ID)
	if err != nil {
		return oracle.DataSource{}, err
	}

	src.AccountID = existing.AccountID
	src.CreatedAt = existing.CreatedAt
	src.UpdatedAt = time.Now().UTC()

	headersJSON, err := json.Marshal(src.Headers)
	if err != nil {
		return oracle.DataSource{}, err
	}
	tenant := s.accountTenant(ctx, src.AccountID)

	result, err := s.db.ExecContext(ctx, `
        UPDATE app_oracle_sources
        SET name = $2, description = $3, url = $4, method = $5, headers = $6, body = $7, enabled = $8, tenant = $9, updated_at = $10
        WHERE id = $1
    `, src.ID, src.Name, src.Description, src.URL, src.Method, headersJSON, src.Body, src.Enabled, tenant, src.UpdatedAt)
	if err != nil {
		return oracle.DataSource{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return oracle.DataSource{}, sql.ErrNoRows
	}
	return src, nil
}

func (s *Store) GetDataSource(ctx context.Context, id string) (oracle.DataSource, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT id, account_id, name, description, url, method, headers, body, enabled, created_at, updated_at
        FROM app_oracle_sources
        WHERE id = $1
    `, id)

	var (
		src        oracle.DataSource
		headersRaw []byte
	)
	if err := row.Scan(&src.ID, &src.AccountID, &src.Name, &src.Description, &src.URL, &src.Method, &headersRaw, &src.Body, &src.Enabled, &src.CreatedAt, &src.UpdatedAt); err != nil {
		return oracle.DataSource{}, err
	}
	if len(headersRaw) > 0 {
		_ = json.Unmarshal(headersRaw, &src.Headers)
	}
	return src, nil
}

func (s *Store) ListDataSources(ctx context.Context, accountID string) ([]oracle.DataSource, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, account_id, name, description, url, method, headers, body, enabled, created_at, updated_at
        FROM app_oracle_sources
        WHERE $1 = '' OR account_id = $1
        ORDER BY created_at
    `, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []oracle.DataSource
	for rows.Next() {
		var (
			src        oracle.DataSource
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

func (s *Store) CreateRequest(ctx context.Context, req oracle.Request) (oracle.Request, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now
	tenant := s.accountTenant(ctx, req.AccountID)

	_, err := s.db.ExecContext(ctx, `
        INSERT INTO app_oracle_requests (id, account_id, data_source_id, status, attempts, payload, result, error, tenant, created_at, updated_at, completed_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, req.ID, req.AccountID, req.DataSourceID, req.Status, req.Attempts, req.Payload, req.Result, req.Error, tenant, req.CreatedAt, req.UpdatedAt, toNullTime(req.CompletedAt))
	if err != nil {
		return oracle.Request{}, err
	}
	return req, nil
}

func (s *Store) UpdateRequest(ctx context.Context, req oracle.Request) (oracle.Request, error) {
	existing, err := s.GetRequest(ctx, req.ID)
	if err != nil {
		return oracle.Request{}, err
	}

	req.AccountID = existing.AccountID
	req.DataSourceID = existing.DataSourceID
	req.CreatedAt = existing.CreatedAt
	req.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, req.AccountID)

	result, err := s.db.ExecContext(ctx, `
        UPDATE app_oracle_requests
        SET status = $2, attempts = $3, payload = $4, result = $5, error = $6, tenant = $7, updated_at = $8, completed_at = $9
        WHERE id = $1
    `, req.ID, req.Status, req.Attempts, req.Payload, req.Result, req.Error, tenant, req.UpdatedAt, toNullTime(req.CompletedAt))
	if err != nil {
		return oracle.Request{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return oracle.Request{}, sql.ErrNoRows
	}
	return req, nil
}

func (s *Store) GetRequest(ctx context.Context, id string) (oracle.Request, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT id, account_id, data_source_id, status, attempts, payload, result, error, created_at, updated_at, completed_at
        FROM app_oracle_requests
        WHERE id = $1
    `, id)

	var (
		req         oracle.Request
		completedAt sql.NullTime
	)
	if err := row.Scan(&req.ID, &req.AccountID, &req.DataSourceID, &req.Status, &req.Attempts, &req.Payload, &req.Result, &req.Error, &req.CreatedAt, &req.UpdatedAt, &completedAt); err != nil {
		return oracle.Request{}, err
	}
	if completedAt.Valid {
		req.CompletedAt = completedAt.Time.UTC()
	}
	return req, nil
}

func (s *Store) ListRequests(ctx context.Context, accountID string, limit int, status string) ([]oracle.Request, error) {
	max := limit
	if max <= 0 || max > 500 {
		max = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, data_source_id, status, attempts, payload, result, error, created_at, updated_at, completed_at
		FROM app_oracle_requests
        WHERE ($1 = '' OR account_id = $1) AND ($3 = '' OR status = $3)
        ORDER BY created_at DESC
        LIMIT $2
    `, accountID, max, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []oracle.Request
	for rows.Next() {
		var (
			req         oracle.Request
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

func (s *Store) ListPendingRequests(ctx context.Context) ([]oracle.Request, error) {
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

	var result []oracle.Request
	for rows.Next() {
		var (
			req         oracle.Request
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
