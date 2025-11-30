package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/google/uuid"
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db       *sql.DB
	accounts AccountChecker
}

// NewPostgresStore creates a new PostgreSQL-backed automation store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

func (s *PostgresStore) CreateAutomationJob(ctx context.Context, job Job) (Job, error) {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	tenant := s.accounts.AccountTenant(ctx, job.AccountID)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_automation_jobs (id, account_id, function_id, name, description, schedule, enabled, last_run, next_run, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, job.ID, job.AccountID, job.FunctionID, job.Name, job.Description, job.Schedule, job.Enabled, toNullTime(job.LastRun), toNullTime(job.NextRun), tenant, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *PostgresStore) UpdateAutomationJob(ctx context.Context, job Job) (Job, error) {
	existing, err := s.GetAutomationJob(ctx, job.ID)
	if err != nil {
		return Job{}, err
	}

	job.CreatedAt = existing.CreatedAt
	job.UpdatedAt = time.Now().UTC()
	tenant := s.accounts.AccountTenant(ctx, job.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_automation_jobs
		SET name = $2, description = $3, schedule = $4, enabled = $5, last_run = $6, next_run = $7, tenant = $8, updated_at = $9
		WHERE id = $1
	`, job.ID, job.Name, job.Description, job.Schedule, job.Enabled, toNullTime(job.LastRun), toNullTime(job.NextRun), tenant, job.UpdatedAt)
	if err != nil {
		return Job{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Job{}, sql.ErrNoRows
	}
	return job, nil
}

func (s *PostgresStore) GetAutomationJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, function_id, name, description, schedule, enabled, last_run, next_run, created_at, updated_at
		FROM app_automation_jobs
		WHERE id = $1
	`, id)

	var (
		job     Job
		lastRun sql.NullTime
		nextRun sql.NullTime
	)
	if err := row.Scan(&job.ID, &job.AccountID, &job.FunctionID, &job.Name, &job.Description, &job.Schedule, &job.Enabled, &lastRun, &nextRun, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return Job{}, err
	}
	if lastRun.Valid {
		job.LastRun = lastRun.Time.UTC()
	}
	if nextRun.Valid {
		job.NextRun = nextRun.Time.UTC()
	}
	return job, nil
}

func (s *PostgresStore) ListAutomationJobs(ctx context.Context, accountID string) ([]Job, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, function_id, name, description, schedule, enabled, last_run, next_run, created_at, updated_at
		FROM app_automation_jobs
		WHERE ($1 = '' OR account_id = $1) AND ($2 = '' OR tenant = $2)
		ORDER BY created_at
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Job
	for rows.Next() {
		var (
			job     Job
			lastRun sql.NullTime
			nextRun sql.NullTime
		)
		if err := rows.Scan(&job.ID, &job.AccountID, &job.FunctionID, &job.Name, &job.Description, &job.Schedule, &job.Enabled, &lastRun, &nextRun, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		if lastRun.Valid {
			job.LastRun = lastRun.Time.UTC()
		}
		if nextRun.Valid {
			job.NextRun = nextRun.Time.UTC()
		}
		result = append(result, job)
	}
	return result, rows.Err()
}

func toNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

// GetAccount retrieves an account by ID (implements AccountStore).
func (s *PostgresStore) GetAccount(ctx context.Context, id string) (account.Account, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, owner, metadata, tenant, created_at, updated_at
		FROM app_accounts
		WHERE id = $1
	`, id)

	var (
		acct        account.Account
		metadataRaw []byte
		tenant      sql.NullString
	)

	if err := row.Scan(&acct.ID, &acct.Owner, &metadataRaw, &tenant, &acct.CreatedAt, &acct.UpdatedAt); err != nil {
		return account.Account{}, err
	}

	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &acct.Metadata)
	}
	if tenant.Valid {
		if acct.Metadata == nil {
			acct.Metadata = map[string]string{}
		}
		acct.Metadata["tenant"] = tenant.String
	}

	return acct, nil
}

// GetFunction retrieves a function by ID (implements FunctionStore).
func (s *PostgresStore) GetFunction(ctx context.Context, id string) (function.Definition, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, description, source, secrets, created_at, updated_at
		FROM app_functions
		WHERE id = $1
	`, id)

	var (
		def        function.Definition
		secretsRaw []byte
	)

	if err := row.Scan(&def.ID, &def.AccountID, &def.Name, &def.Description, &def.Source, &secretsRaw, &def.CreatedAt, &def.UpdatedAt); err != nil {
		return function.Definition{}, err
	}
	if len(secretsRaw) > 0 {
		_ = json.Unmarshal(secretsRaw, &def.Secrets)
	}
	return def, nil
}
