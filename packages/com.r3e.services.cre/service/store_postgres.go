package cre

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// PostgresStore implements Store using PostgreSQL.
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


func (s *PostgresStore) CreatePlaybook(ctx context.Context, pb Playbook) (Playbook, error) {
	if pb.ID == "" {
		pb.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	pb.CreatedAt = now
	pb.UpdatedAt = now
	tenant := s.accountTenant(ctx, pb.AccountID)

	stepsJSON, err := json.Marshal(pb.Steps)
	if err != nil {
		return Playbook{}, err
	}
	tagsJSON, err := json.Marshal(pb.Tags)
	if err != nil {
		return Playbook{}, err
	}
	metaJSON, err := json.Marshal(pb.Metadata)
	if err != nil {
		return Playbook{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_cre_playbooks (id, account_id, name, description, steps, tags, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, pb.ID, pb.AccountID, pb.Name, pb.Description, stepsJSON, tagsJSON, metaJSON, tenant, pb.CreatedAt, pb.UpdatedAt)
	if err != nil {
		return Playbook{}, err
	}
	return pb, nil
}

func (s *PostgresStore) UpdatePlaybook(ctx context.Context, pb Playbook) (Playbook, error) {
	existing, err := s.GetPlaybook(ctx, pb.ID)
	if err != nil {
		return Playbook{}, err
	}
	pb.CreatedAt = existing.CreatedAt
	pb.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, pb.AccountID)

	stepsJSON, err := json.Marshal(pb.Steps)
	if err != nil {
		return Playbook{}, err
	}
	tagsJSON, err := json.Marshal(pb.Tags)
	if err != nil {
		return Playbook{}, err
	}
	metaJSON, err := json.Marshal(pb.Metadata)
	if err != nil {
		return Playbook{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_cre_playbooks
		SET name = $2, description = $3, steps = $4, tags = $5, metadata = $6, tenant = $7, updated_at = $8
		WHERE id = $1
	`, pb.ID, pb.Name, pb.Description, stepsJSON, tagsJSON, metaJSON, tenant, pb.UpdatedAt)
	if err != nil {
		return Playbook{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Playbook{}, sql.ErrNoRows
	}
	return pb, nil
}

func (s *PostgresStore) GetPlaybook(ctx context.Context, id string) (Playbook, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, description, steps, tags, metadata, created_at, updated_at
		FROM app_cre_playbooks
		WHERE id = $1
	`, id)

	return scanPlaybook(row)
}

func (s *PostgresStore) ListPlaybooks(ctx context.Context, accountID string) ([]Playbook, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, description, steps, tags, metadata, created_at, updated_at
		FROM app_cre_playbooks
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Playbook
	for rows.Next() {
		pb, err := scanPlaybook(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, pb)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateRun(ctx context.Context, run Run) (Run, error) {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	run.CreatedAt = now
	run.UpdatedAt = now
	tenant := s.accountTenant(ctx, run.AccountID)

	paramsJSON, err := json.Marshal(run.Parameters)
	if err != nil {
		return Run{}, err
	}
	tagsJSON, err := json.Marshal(run.Tags)
	if err != nil {
		return Run{}, err
	}
	resultsJSON, err := json.Marshal(run.Results)
	if err != nil {
		return Run{}, err
	}
	metaJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return Run{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_cre_runs (id, account_id, playbook_id, executor_id, status, parameters, tags, results, metadata, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, run.ID, run.AccountID, run.PlaybookID, core.ToNullString(run.ExecutorID), run.Status, paramsJSON, tagsJSON, resultsJSON, metaJSON, tenant, run.CreatedAt, run.UpdatedAt, core.ToNullTime(core.PtrTime(run.CompletedAt)))
	if err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *PostgresStore) UpdateRun(ctx context.Context, run Run) (Run, error) {
	existing, err := s.GetRun(ctx, run.ID)
	if err != nil {
		return Run{}, err
	}
	run.CreatedAt = existing.CreatedAt
	run.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, run.AccountID)

	paramsJSON, err := json.Marshal(run.Parameters)
	if err != nil {
		return Run{}, err
	}
	tagsJSON, err := json.Marshal(run.Tags)
	if err != nil {
		return Run{}, err
	}
	resultsJSON, err := json.Marshal(run.Results)
	if err != nil {
		return Run{}, err
	}
	metaJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return Run{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_cre_runs
		SET status = $2, executor_id = $3, parameters = $4, tags = $5, results = $6, metadata = $7, tenant = $8, updated_at = $9, completed_at = $10
		WHERE id = $1
	`, run.ID, run.Status, core.ToNullString(run.ExecutorID), paramsJSON, tagsJSON, resultsJSON, metaJSON, tenant, run.UpdatedAt, core.ToNullTime(core.PtrTime(run.CompletedAt)))
	if err != nil {
		return Run{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Run{}, sql.ErrNoRows
	}
	return run, nil
}

func (s *PostgresStore) GetRun(ctx context.Context, id string) (Run, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, playbook_id, executor_id, status, parameters, tags, results, metadata, created_at, updated_at, completed_at
		FROM app_cre_runs
		WHERE id = $1
	`, id)
	return scanRun(row)
}

func (s *PostgresStore) ListRuns(ctx context.Context, accountID string, limit int) ([]Run, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, playbook_id, executor_id, status, parameters, tags, results, metadata, created_at, updated_at, completed_at
		FROM app_cre_runs
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, accountID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateExecutor(ctx context.Context, exec Executor) (Executor, error) {
	if exec.ID == "" {
		exec.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	exec.CreatedAt = now
	exec.UpdatedAt = now

	metaJSON, err := json.Marshal(exec.Metadata)
	if err != nil {
		return Executor{}, err
	}
	tagsJSON, err := json.Marshal(exec.Tags)
	if err != nil {
		return Executor{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_cre_executors (id, account_id, name, type, endpoint, metadata, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, exec.ID, exec.AccountID, exec.Name, exec.Type, exec.Endpoint, metaJSON, tagsJSON, exec.CreatedAt, exec.UpdatedAt)
	if err != nil {
		return Executor{}, err
	}
	return exec, nil
}

func (s *PostgresStore) UpdateExecutor(ctx context.Context, exec Executor) (Executor, error) {
	existing, err := s.GetExecutor(ctx, exec.ID)
	if err != nil {
		return Executor{}, err
	}
	exec.CreatedAt = existing.CreatedAt
	exec.UpdatedAt = time.Now().UTC()

	metaJSON, err := json.Marshal(exec.Metadata)
	if err != nil {
		return Executor{}, err
	}
	tagsJSON, err := json.Marshal(exec.Tags)
	if err != nil {
		return Executor{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_cre_executors
		SET name = $2, type = $3, endpoint = $4, metadata = $5, tags = $6, updated_at = $7
		WHERE id = $1
	`, exec.ID, exec.Name, exec.Type, exec.Endpoint, metaJSON, tagsJSON, exec.UpdatedAt)
	if err != nil {
		return Executor{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Executor{}, sql.ErrNoRows
	}
	return exec, nil
}

func (s *PostgresStore) GetExecutor(ctx context.Context, id string) (Executor, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, type, endpoint, metadata, tags, created_at, updated_at
		FROM app_cre_executors
		WHERE id = $1
	`, id)
	return scanExecutor(row)
}

func (s *PostgresStore) ListExecutors(ctx context.Context, accountID string) ([]Executor, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, type, endpoint, metadata, tags, created_at, updated_at
		FROM app_cre_executors
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Executor
	for rows.Next() {
		exec, err := scanExecutor(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, exec)
	}
	return result, rows.Err()
}


func scanPlaybook(scanner core.RowScanner) (Playbook, error) {
	var (
		pb          Playbook
		stepsJSON   []byte
		tagsJSON    []byte
		metaJSON    []byte
		createdAt   time.Time
		updatedAt   time.Time
		description sql.NullString
	)
	if err := scanner.Scan(&pb.ID, &pb.AccountID, &pb.Name, &description, &stepsJSON, &tagsJSON, &metaJSON, &createdAt, &updatedAt); err != nil {
		return Playbook{}, err
	}
	if description.Valid {
		pb.Description = description.String
	}
	if len(stepsJSON) > 0 {
		if err := json.Unmarshal(stepsJSON, &pb.Steps); err != nil {
			return Playbook{}, err
		}
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &pb.Tags)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &pb.Metadata)
	}
	pb.CreatedAt = createdAt.UTC()
	pb.UpdatedAt = updatedAt.UTC()
	return pb, nil
}

func scanRun(scanner core.RowScanner) (Run, error) {
	var (
		run         Run
		executorID  sql.NullString
		paramsJSON  []byte
		tagsJSON    []byte
		resultsJSON []byte
		metaJSON    []byte
		createdAt   time.Time
		updatedAt   time.Time
		completedAt sql.NullTime
	)
	if err := scanner.Scan(&run.ID, &run.AccountID, &run.PlaybookID, &executorID, &run.Status, &paramsJSON, &tagsJSON, &resultsJSON, &metaJSON, &createdAt, &updatedAt, &completedAt); err != nil {
		return Run{}, err
	}
	if executorID.Valid {
		run.ExecutorID = executorID.String
	}
	if len(paramsJSON) > 0 {
		_ = json.Unmarshal(paramsJSON, &run.Parameters)
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &run.Tags)
	}
	if len(resultsJSON) > 0 {
		_ = json.Unmarshal(resultsJSON, &run.Results)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &run.Metadata)
	}
	run.CreatedAt = createdAt.UTC()
	run.UpdatedAt = updatedAt.UTC()
	if completedAt.Valid {
		c := completedAt.Time.UTC()
		run.CompletedAt = &c
	}
	return run, nil
}

func scanExecutor(scanner core.RowScanner) (Executor, error) {
	var (
		exec      Executor
		metaJSON  []byte
		tagsJSON  []byte
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&exec.ID, &exec.AccountID, &exec.Name, &exec.Type, &exec.Endpoint, &metaJSON, &tagsJSON, &createdAt, &updatedAt); err != nil {
		return Executor{}, err
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &exec.Metadata)
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &exec.Tags)
	}
	exec.CreatedAt = createdAt.UTC()
	exec.UpdatedAt = updatedAt.UTC()
	return exec, nil
}
