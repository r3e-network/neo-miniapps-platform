package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/google/uuid"
)

// Store implements the storage interfaces backed by PostgreSQL.
type Store struct {
	db *sql.DB
}

var _ storage.AccountStore = (*Store)(nil)
var _ storage.FunctionStore = (*Store)(nil)
var _ storage.TriggerStore = (*Store)(nil)
var _ storage.GasBankStore = (*Store)(nil)
var _ storage.AutomationStore = (*Store)(nil)
var _ storage.PriceFeedStore = (*Store)(nil)
var _ storage.DataFeedStore = (*Store)(nil)
var _ storage.DataLinkStore = (*Store)(nil)
var _ storage.DataStreamStore = (*Store)(nil)
var _ storage.OracleStore = (*Store)(nil)
var _ storage.SecretStore = (*Store)(nil)
var _ storage.CREStore = (*Store)(nil)
var _ storage.VRFStore = (*Store)(nil)
var _ storage.WorkspaceWalletStore = (*Store)(nil)
var _ storage.CCIPStore = (*Store)(nil)
var _ storage.DTAStore = (*Store)(nil)
var _ storage.ConfidentialStore = (*Store)(nil)

// New creates a Store using the provided database handle.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// --- AccountStore -----------------------------------------------------------

func (s *Store) CreateAccount(ctx context.Context, acct account.Account) (account.Account, error) {
	if acct.ID == "" {
		acct.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now
	tenant := tenantFromMetadata(acct.Metadata)

	metadataJSON, err := json.Marshal(acct.Metadata)
	if err != nil {
		return account.Account{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_accounts (id, owner, metadata, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, acct.ID, acct.Owner, metadataJSON, tenant, acct.CreatedAt, acct.UpdatedAt)
	if err != nil {
		return account.Account{}, err
	}
	return acct, nil
}

func (s *Store) UpdateAccount(ctx context.Context, acct account.Account) (account.Account, error) {
	existing, err := s.GetAccount(ctx, acct.ID)
	if err != nil {
		return account.Account{}, err
	}

	acct.CreatedAt = existing.CreatedAt
	acct.UpdatedAt = time.Now().UTC()
	tenant := tenantFromMetadata(acct.Metadata)

	metadataJSON, err := json.Marshal(acct.Metadata)
	if err != nil {
		return account.Account{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_accounts
		SET owner = $2, metadata = $3, tenant = $4, updated_at = $5
		WHERE id = $1
	`, acct.ID, acct.Owner, metadataJSON, tenant, acct.UpdatedAt)
	if err != nil {
		return account.Account{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return account.Account{}, sql.ErrNoRows
	}
	return acct, nil
}

func (s *Store) GetAccount(ctx context.Context, id string) (account.Account, error) {
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

func (s *Store) ListAccounts(ctx context.Context) ([]account.Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner, metadata, tenant, created_at, updated_at
		FROM app_accounts
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []account.Account
	for rows.Next() {
		var (
			acct        account.Account
			metadataRaw []byte
			tenant      sql.NullString
		)

		if err := rows.Scan(&acct.ID, &acct.Owner, &metadataRaw, &tenant, &acct.CreatedAt, &acct.UpdatedAt); err != nil {
			return nil, err
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
		result = append(result, acct)
	}
	return result, rows.Err()
}

func tenantFromMetadata(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	return meta["tenant"]
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM app_accounts WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// --- FunctionStore ----------------------------------------------------------

func (s *Store) CreateFunction(ctx context.Context, def function.Definition) (function.Definition, error) {
	if def.AccountID == "" {
		return function.Definition{}, errors.New("account_id required")
	}
	if def.ID == "" {
		def.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	def.CreatedAt = now
	def.UpdatedAt = now

	secretsJSON, err := json.Marshal(def.Secrets)
	if err != nil {
		return function.Definition{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_functions (id, account_id, name, description, source, secrets, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, def.ID, def.AccountID, def.Name, def.Description, def.Source, secretsJSON, def.CreatedAt, def.UpdatedAt)
	if err != nil {
		return function.Definition{}, err
	}
	return def, nil
}

func (s *Store) UpdateFunction(ctx context.Context, def function.Definition) (function.Definition, error) {
	existing, err := s.GetFunction(ctx, def.ID)
	if err != nil {
		return function.Definition{}, err
	}

	def.AccountID = existing.AccountID
	def.CreatedAt = existing.CreatedAt
	def.UpdatedAt = time.Now().UTC()

	secretsJSON, err := json.Marshal(def.Secrets)
	if err != nil {
		return function.Definition{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_functions
		SET name = $2, description = $3, source = $4, secrets = $5, updated_at = $6
		WHERE id = $1
	`, def.ID, def.Name, def.Description, def.Source, secretsJSON, def.UpdatedAt)
	if err != nil {
		return function.Definition{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return function.Definition{}, sql.ErrNoRows
	}
	return def, nil
}

func (s *Store) GetFunction(ctx context.Context, id string) (function.Definition, error) {
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

func (s *Store) ListFunctions(ctx context.Context, accountID string) ([]function.Definition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, description, source, secrets, created_at, updated_at
		FROM app_functions
		WHERE $1 = '' OR account_id = $1
		ORDER BY created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []function.Definition
	for rows.Next() {
		var (
			def        function.Definition
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

func (s *Store) CreateExecution(ctx context.Context, exec function.Execution) (function.Execution, error) {
	if exec.ID == "" {
		exec.ID = uuid.NewString()
	}
	exec.StartedAt = exec.StartedAt.UTC()
	exec.CompletedAt = exec.CompletedAt.UTC()

	inputJSON, err := json.Marshal(exec.Input)
	if err != nil {
		return function.Execution{}, err
	}
	outputJSON, err := json.Marshal(exec.Output)
	if err != nil {
		return function.Execution{}, err
	}
	logsJSON, err := json.Marshal(exec.Logs)
	if err != nil {
		return function.Execution{}, err
	}
	actionsJSON, err := json.Marshal(exec.Actions)
	if err != nil {
		return function.Execution{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_function_executions
			(id, account_id, function_id, input, output, logs, actions, error, status, started_at, completed_at, duration_ns)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, exec.ID, exec.AccountID, exec.FunctionID, inputJSON, outputJSON, logsJSON, actionsJSON, toNullString(exec.Error), exec.Status, exec.StartedAt, toNullTime(exec.CompletedAt), exec.Duration.Nanoseconds())
	if err != nil {
		return function.Execution{}, err
	}
	return exec, nil
}

func (s *Store) GetExecution(ctx context.Context, id string) (function.Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, function_id, input, output, logs, actions, error, status, started_at, completed_at, duration_ns
		FROM app_function_executions
		WHERE id = $1
	`, id)

	return scanFunctionExecution(row)
}

func (s *Store) ListFunctionExecutions(ctx context.Context, functionID string, limit int) ([]function.Execution, error) {
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

	var result []function.Execution
	for rows.Next() {
		exec, err := scanFunctionExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, exec)
	}
	return result, rows.Err()
}

func (s *Store) DeleteFunction(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM app_functions WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// --- TriggerStore -----------------------------------------------------------

func (s *Store) CreateTrigger(ctx context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	if trg.ID == "" {
		trg.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	trg.CreatedAt = now
	trg.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_triggers (id, account_id, function_id, rule, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, trg.ID, trg.AccountID, trg.FunctionID, trg.Rule, trg.Enabled, trg.CreatedAt, trg.UpdatedAt)
	if err != nil {
		return trigger.Trigger{}, err
	}
	return trg, nil
}

func (s *Store) UpdateTrigger(ctx context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	existing, err := s.GetTrigger(ctx, trg.ID)
	if err != nil {
		return trigger.Trigger{}, err
	}
	trg.AccountID = existing.AccountID
	trg.FunctionID = existing.FunctionID
	trg.CreatedAt = existing.CreatedAt
	trg.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_triggers
		SET rule = $2, enabled = $3, updated_at = $4
		WHERE id = $1
	`, trg.ID, trg.Rule, trg.Enabled, trg.UpdatedAt)
	if err != nil {
		return trigger.Trigger{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return trigger.Trigger{}, sql.ErrNoRows
	}
	return trg, nil
}

func (s *Store) GetTrigger(ctx context.Context, id string) (trigger.Trigger, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, function_id, rule, enabled, created_at, updated_at
		FROM app_triggers
		WHERE id = $1
	`, id)

	var trg trigger.Trigger
	if err := row.Scan(&trg.ID, &trg.AccountID, &trg.FunctionID, &trg.Rule, &trg.Enabled, &trg.CreatedAt, &trg.UpdatedAt); err != nil {
		return trigger.Trigger{}, err
	}
	return trg, nil
}

func (s *Store) ListTriggers(ctx context.Context, accountID string) ([]trigger.Trigger, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, function_id, rule, enabled, created_at, updated_at
		FROM app_triggers
		WHERE $1 = '' OR account_id = $1
		ORDER BY created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []trigger.Trigger
	for rows.Next() {
		var trg trigger.Trigger
		if err := rows.Scan(&trg.ID, &trg.AccountID, &trg.FunctionID, &trg.Rule, &trg.Enabled, &trg.CreatedAt, &trg.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, trg)
	}
	return result, rows.Err()
}
