// Package functions provides the Functions service as a ServicePackage.
package functions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PostgresStore implements Store using PostgreSQL.
// This is the service-local store implementation that uses the generic
// database connection provided by the Service Engine.
type PostgresStore struct {
	db       *sql.DB
	accounts AccountChecker
}

// NewPostgresStore creates a new PostgreSQL-backed function store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

// CreateFunction creates a new function definition.
func (s *PostgresStore) CreateFunction(ctx context.Context, def Definition) (Definition, error) {
	if def.AccountID == "" {
		return Definition{}, errors.New("account_id required")
	}
	if def.ID == "" {
		def.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	def.CreatedAt = now
	def.UpdatedAt = now

	secretsJSON, err := json.Marshal(def.Secrets)
	if err != nil {
		return Definition{}, err
	}
	tenant := s.accountTenant(ctx, def.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_functions (id, account_id, name, description, source, secrets, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, def.ID, def.AccountID, def.Name, def.Description, def.Source, secretsJSON, tenant, def.CreatedAt, def.UpdatedAt)
	if err != nil {
		return Definition{}, err
	}
	return def, nil
}

// UpdateFunction updates an existing function definition.
func (s *PostgresStore) UpdateFunction(ctx context.Context, def Definition) (Definition, error) {
	existing, err := s.GetFunction(ctx, def.ID)
	if err != nil {
		return Definition{}, err
	}

	def.AccountID = existing.AccountID
	def.CreatedAt = existing.CreatedAt
	def.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, def.AccountID)

	secretsJSON, err := json.Marshal(def.Secrets)
	if err != nil {
		return Definition{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_functions
		SET name = $2, description = $3, source = $4, secrets = $5, tenant = $6, updated_at = $7
		WHERE id = $1
	`, def.ID, def.Name, def.Description, def.Source, secretsJSON, tenant, def.UpdatedAt)
	if err != nil {
		return Definition{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Definition{}, sql.ErrNoRows
	}
	return def, nil
}

// GetFunction retrieves a function definition by ID.
func (s *PostgresStore) GetFunction(ctx context.Context, id string) (Definition, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, description, source, secrets, created_at, updated_at
		FROM app_functions
		WHERE id = $1
	`, id)

	var (
		def        Definition
		secretsRaw []byte
	)

	if err := row.Scan(&def.ID, &def.AccountID, &def.Name, &def.Description, &def.Source, &secretsRaw, &def.CreatedAt, &def.UpdatedAt); err != nil {
		return Definition{}, err
	}
	if len(secretsRaw) > 0 {
		_ = json.Unmarshal(secretsRaw, &def.Secrets)
	}
	return def, nil
}

// ListFunctions lists all function definitions for an account.
func (s *PostgresStore) ListFunctions(ctx context.Context, accountID string) ([]Definition, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, description, source, secrets, created_at, updated_at
		FROM app_functions
		WHERE ($1 = '' OR account_id = $1) AND ($2 = '' OR tenant = $2)
		ORDER BY created_at
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Definition
	for rows.Next() {
		var (
			def        Definition
			secretsRaw []byte
		)
		if err := rows.Scan(&def.ID, &def.AccountID, &def.Name, &def.Description, &def.Source, &secretsRaw, &def.CreatedAt, &def.UpdatedAt); err != nil {
			return nil, err
		}
		if len(secretsRaw) > 0 {
			_ = json.Unmarshal(secretsRaw, &def.Secrets)
		}
		result = append(result, def)
	}
	return result, rows.Err()
}

// CreateExecution creates a new function execution record.
func (s *PostgresStore) CreateExecution(ctx context.Context, exec Execution) (Execution, error) {
	if exec.ID == "" {
		exec.ID = uuid.NewString()
	}
	exec.StartedAt = exec.StartedAt.UTC()
	exec.CompletedAt = exec.CompletedAt.UTC()

	inputJSON, err := json.Marshal(exec.Input)
	if err != nil {
		return Execution{}, err
	}
	outputJSON, err := json.Marshal(exec.Output)
	if err != nil {
		return Execution{}, err
	}
	logsJSON, err := json.Marshal(exec.Logs)
	if err != nil {
		return Execution{}, err
	}
	actionsJSON, err := json.Marshal(exec.Actions)
	if err != nil {
		return Execution{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_function_executions
			(id, account_id, function_id, input, output, logs, actions, error, status, started_at, completed_at, duration_ns)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, exec.ID, exec.AccountID, exec.FunctionID, inputJSON, outputJSON, logsJSON, actionsJSON, toNullString(exec.Error), exec.Status, exec.StartedAt, toNullTime(exec.CompletedAt), exec.Duration.Nanoseconds())
	if err != nil {
		return Execution{}, err
	}
	return exec, nil
}

// GetExecution retrieves a function execution by ID.
func (s *PostgresStore) GetExecution(ctx context.Context, id string) (Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, function_id, input, output, logs, actions, error, status, started_at, completed_at, duration_ns
		FROM app_function_executions
		WHERE id = $1
	`, id)

	return scanFunctionExecution(row)
}

// ListFunctionExecutions lists execution history for a function.
func (s *PostgresStore) ListFunctionExecutions(ctx context.Context, functionID string, limit int) ([]Execution, error) {
	query := `
		SELECT id, account_id, function_id, input, output, logs, actions, error, status, started_at, completed_at, duration_ns
		FROM app_function_executions
		WHERE function_id = $1
		ORDER BY started_at DESC
	`
	args := []any{functionID}
	if limit > 0 {
		query += " LIMIT $2"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Execution
	for rows.Next() {
		exec, err := scanFunctionExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, exec)
	}
	return result, rows.Err()
}

// accountTenant returns the tenant for an account (empty if none).
func (s *PostgresStore) accountTenant(ctx context.Context, accountID string) string {
	if s.accounts == nil {
		return ""
	}
	return s.accounts.AccountTenant(ctx, accountID)
}

// Helper functions

// rowScanner abstracts sql.Row and sql.Rows for scanning.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanFunctionExecution scans a function execution from a row.
func scanFunctionExecution(scanner rowScanner) (Execution, error) {
	var (
		exec        Execution
		inputJSON   []byte
		outputJSON  []byte
		logsJSON    []byte
		actionsJSON []byte
		errorText   sql.NullString
		startedAt   time.Time
		completedAt sql.NullTime
		durationNS  sql.NullInt64
	)
	if err := scanner.Scan(&exec.ID, &exec.AccountID, &exec.FunctionID, &inputJSON, &outputJSON, &logsJSON, &actionsJSON, &errorText, &exec.Status, &startedAt, &completedAt, &durationNS); err != nil {
		return Execution{}, err
	}
	if len(inputJSON) > 0 {
		_ = json.Unmarshal(inputJSON, &exec.Input)
	}
	if len(outputJSON) > 0 {
		_ = json.Unmarshal(outputJSON, &exec.Output)
	}
	if len(logsJSON) > 0 {
		_ = json.Unmarshal(logsJSON, &exec.Logs)
	}
	if len(actionsJSON) > 0 {
		_ = json.Unmarshal(actionsJSON, &exec.Actions)
	}
	if errorText.Valid {
		exec.Error = errorText.String
	}
	exec.StartedAt = startedAt.UTC()
	if completedAt.Valid {
		exec.CompletedAt = completedAt.Time.UTC()
	}
	if durationNS.Valid {
		exec.Duration = time.Duration(durationNS.Int64)
	}
	return exec, nil
}

// toNullString converts a string to sql.NullString.
func toNullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

// toNullTime converts a time.Time to sql.NullTime.
func toNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}
