package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
)

// AutomationStore implementation

func (s *Store) CreateAutomationJob(ctx context.Context, job automation.Job) (automation.Job, error) {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	tenant := s.accountTenant(ctx, job.AccountID)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_automation_jobs (id, account_id, function_id, name, description, schedule, enabled, last_run, next_run, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, job.ID, job.AccountID, job.FunctionID, job.Name, job.Description, job.Schedule, job.Enabled, toNullTime(job.LastRun), toNullTime(job.NextRun), tenant, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return automation.Job{}, err
	}
	return job, nil
}

func (s *Store) UpdateAutomationJob(ctx context.Context, job automation.Job) (automation.Job, error) {
	existing, err := s.GetAutomationJob(ctx, job.ID)
	if err != nil {
		return automation.Job{}, err
	}

	job.CreatedAt = existing.CreatedAt
	job.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, job.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_automation_jobs
		SET name = $2, description = $3, schedule = $4, enabled = $5, last_run = $6, next_run = $7, tenant = $8, updated_at = $9
		WHERE id = $1
	`, job.ID, job.Name, job.Description, job.Schedule, job.Enabled, toNullTime(job.LastRun), toNullTime(job.NextRun), tenant, job.UpdatedAt)
	if err != nil {
		return automation.Job{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return automation.Job{}, sql.ErrNoRows
	}
	return job, nil
}

func (s *Store) GetAutomationJob(ctx context.Context, id string) (automation.Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, function_id, name, description, schedule, enabled, last_run, next_run, created_at, updated_at
		FROM app_automation_jobs
		WHERE id = $1
	`, id)

	var (
		job     automation.Job
		lastRun sql.NullTime
		nextRun sql.NullTime
	)
	if err := row.Scan(&job.ID, &job.AccountID, &job.FunctionID, &job.Name, &job.Description, &job.Schedule, &job.Enabled, &lastRun, &nextRun, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return automation.Job{}, err
	}
	if lastRun.Valid {
		job.LastRun = lastRun.Time.UTC()
	}
	if nextRun.Valid {
		job.NextRun = nextRun.Time.UTC()
	}
	return job, nil
}

func (s *Store) ListAutomationJobs(ctx context.Context, accountID string) ([]automation.Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, function_id, name, description, schedule, enabled, last_run, next_run, created_at, updated_at
		FROM app_automation_jobs
		WHERE $1 = '' OR account_id = $1
		ORDER BY created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []automation.Job
	for rows.Next() {
		var (
			job     automation.Job
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
