package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/R3E-Network/service_layer/pkg/storage"
	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/google/uuid"
)

// Store implements the storage interfaces backed by PostgreSQL.
type Store struct {
	db *sql.DB
}

var _ storage.AccountStore = (*Store)(nil)
var _ storage.FunctionStore = (*Store)(nil)
var _ storage.GasBankStore = (*Store)(nil)
var _ storage.AutomationStore = (*Store)(nil)
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

// accountTenant fetches the tenant for the given account ID (empty if none). It is best-effort
// and intentionally ignores errors to avoid failing core operations when tenant is unset.
func (s *Store) accountTenant(ctx context.Context, accountID string) string {
	var tenant sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT tenant FROM app_accounts WHERE id = $1
	`, accountID).Scan(&tenant)
	if tenant.Valid {
		return tenant.String
	}
	return ""
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

// AccountExists checks if an account exists by ID.
// Returns nil if account exists, error otherwise.
func (s *Store) AccountExists(ctx context.Context, accountID string) error {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM app_accounts WHERE id = $1)
	`, accountID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("account not found")
	}
	return nil
}

// AccountTenant returns the tenant for an account (empty if none).
func (s *Store) AccountTenant(ctx context.Context, accountID string) string {
	return s.accountTenant(ctx, accountID)
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
	tenant := s.accountTenant(ctx, def.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_functions (id, account_id, name, description, source, secrets, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, def.ID, def.AccountID, def.Name, def.Description, def.Source, secretsJSON, tenant, def.CreatedAt, def.UpdatedAt)
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
	tenant := s.accountTenant(ctx, def.AccountID)

	secretsJSON, err := json.Marshal(def.Secrets)
	if err != nil {
		return function.Definition{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_functions
		SET name = $2, description = $3, source = $4, secrets = $5, tenant = $6, updated_at = $7
		WHERE id = $1
	`, def.ID, def.Name, def.Description, def.Source, secretsJSON, tenant, def.UpdatedAt)
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
