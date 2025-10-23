package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/domain/secret"
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
var _ storage.OracleStore = (*Store)(nil)
var _ storage.SecretStore = (*Store)(nil)

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

	metadataJSON, err := json.Marshal(acct.Metadata)
	if err != nil {
		return account.Account{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_accounts (id, owner, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, acct.ID, acct.Owner, metadataJSON, acct.CreatedAt, acct.UpdatedAt)
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

	metadataJSON, err := json.Marshal(acct.Metadata)
	if err != nil {
		return account.Account{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_accounts
		SET owner = $2, metadata = $3, updated_at = $4
		WHERE id = $1
	`, acct.ID, acct.Owner, metadataJSON, acct.UpdatedAt)
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
		SELECT id, owner, metadata, created_at, updated_at
		FROM app_accounts
		WHERE id = $1
	`, id)

	var (
		acct        account.Account
		metadataRaw []byte
	)

	if err := row.Scan(&acct.ID, &acct.Owner, &metadataRaw, &acct.CreatedAt, &acct.UpdatedAt); err != nil {
		return account.Account{}, err
	}

	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &acct.Metadata)
	}

	return acct, nil
}

func (s *Store) ListAccounts(ctx context.Context) ([]account.Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner, metadata, created_at, updated_at
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
		)

		if err := rows.Scan(&acct.ID, &acct.Owner, &metadataRaw, &acct.CreatedAt, &acct.UpdatedAt); err != nil {
			return nil, err
		}
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &acct.Metadata)
		}
		result = append(result, acct)
	}
	return result, rows.Err()
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

// --- GasBankStore -----------------------------------------------------------

func (s *Store) CreateGasAccount(ctx context.Context, acct gasbank.Account) (gasbank.Account, error) {
	if acct.ID == "" {
		acct.ID = uuid.NewString()
	}
	acct.AccountID = strings.TrimSpace(acct.AccountID)
	acct.WalletAddress = normalizeWallet(acct.WalletAddress)
	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_gas_accounts (id, account_id, wallet_address, balance, available, pending, daily_withdrawal, last_withdrawal, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, acct.ID, acct.AccountID, toNullString(acct.WalletAddress), acct.Balance, acct.Available, acct.Pending, acct.DailyWithdrawal, toNullTime(acct.LastWithdrawal), acct.CreatedAt, acct.UpdatedAt)
	if err != nil {
		return gasbank.Account{}, err
	}
	return acct, nil
}

func (s *Store) UpdateGasAccount(ctx context.Context, acct gasbank.Account) (gasbank.Account, error) {
	existing, err := s.GetGasAccount(ctx, acct.ID)
	if err != nil {
		return gasbank.Account{}, err
	}

	acct.AccountID = existing.AccountID
	if acct.WalletAddress == "" {
		acct.WalletAddress = existing.WalletAddress
	}
	acct.WalletAddress = normalizeWallet(acct.WalletAddress)
	acct.CreatedAt = existing.CreatedAt
	acct.UpdatedAt = time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		UPDATE app_gas_accounts
		SET wallet_address = $2, balance = $3, available = $4, pending = $5, daily_withdrawal = $6, last_withdrawal = $7, updated_at = $8
		WHERE id = $1
	`, acct.ID, toNullString(acct.WalletAddress), acct.Balance, acct.Available, acct.Pending, acct.DailyWithdrawal, toNullTime(acct.LastWithdrawal), acct.UpdatedAt)
	if err != nil {
		return gasbank.Account{}, err
	}
	return acct, nil
}

func (s *Store) GetGasAccount(ctx context.Context, id string) (gasbank.Account, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, wallet_address, balance, available, pending, daily_withdrawal, last_withdrawal, created_at, updated_at
		FROM app_gas_accounts
		WHERE id = $1
	`, id)

	acct, err := scanGasAccount(row)
	if err != nil {
		return gasbank.Account{}, err
	}
	return acct, nil
}

func (s *Store) GetGasAccountByWallet(ctx context.Context, wallet string) (gasbank.Account, error) {
	wallet = normalizeWallet(wallet)
	if wallet == "" {
		return gasbank.Account{}, sql.ErrNoRows
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, wallet_address, balance, available, pending, daily_withdrawal, last_withdrawal, created_at, updated_at
		FROM app_gas_accounts
		WHERE wallet_address = $1
	`, wallet)

	acct, err := scanGasAccount(row)
	if err != nil {
		return gasbank.Account{}, err
	}
	return acct, nil
}

func (s *Store) ListGasAccounts(ctx context.Context, accountID string) ([]gasbank.Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, wallet_address, balance, available, pending, daily_withdrawal, last_withdrawal, created_at, updated_at
		FROM app_gas_accounts
		WHERE $1 = '' OR account_id = $1
		ORDER BY created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []gasbank.Account
	for rows.Next() {
		acct, err := scanGasAccount(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, acct)
	}
	return result, rows.Err()
}

func (s *Store) CreateGasTransaction(ctx context.Context, tx gasbank.Transaction) (gasbank.Transaction, error) {
	if tx.ID == "" {
		tx.ID = uuid.NewString()
	}
	tx.AccountID = strings.TrimSpace(tx.AccountID)
	tx.UserAccountID = strings.TrimSpace(tx.UserAccountID)
	tx.Type = strings.TrimSpace(tx.Type)
	tx.BlockchainTxID = strings.TrimSpace(tx.BlockchainTxID)
	tx.FromAddress = strings.TrimSpace(tx.FromAddress)
	tx.ToAddress = strings.TrimSpace(tx.ToAddress)
	tx.Notes = strings.TrimSpace(tx.Notes)
	tx.Error = strings.TrimSpace(tx.Error)
	now := time.Now().UTC()
	tx.CreatedAt = now
	tx.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_gas_transactions (id, account_id, user_account_id, type, amount, net_amount, status, blockchain_tx_id, from_address, to_address, notes, error, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, tx.ID, tx.AccountID, toNullString(tx.UserAccountID), tx.Type, tx.Amount, tx.NetAmount, tx.Status, toNullString(tx.BlockchainTxID), toNullString(tx.FromAddress), toNullString(tx.ToAddress), toNullString(tx.Notes), toNullString(tx.Error), toNullTime(tx.CompletedAt), tx.CreatedAt, tx.UpdatedAt)
	if err != nil {
		return gasbank.Transaction{}, err
	}
	return tx, nil
}

func (s *Store) UpdateGasTransaction(ctx context.Context, tx gasbank.Transaction) (gasbank.Transaction, error) {
	existing, err := s.GetGasTransaction(ctx, tx.ID)
	if err != nil {
		return gasbank.Transaction{}, err
	}

	tx.AccountID = existing.AccountID
	if tx.UserAccountID == "" {
		tx.UserAccountID = existing.UserAccountID
	}
	tx.UserAccountID = strings.TrimSpace(tx.UserAccountID)
	tx.Type = strings.TrimSpace(tx.Type)
	tx.BlockchainTxID = strings.TrimSpace(tx.BlockchainTxID)
	tx.FromAddress = strings.TrimSpace(tx.FromAddress)
	tx.ToAddress = strings.TrimSpace(tx.ToAddress)
	tx.Notes = strings.TrimSpace(tx.Notes)
	tx.Error = strings.TrimSpace(tx.Error)
	tx.CreatedAt = existing.CreatedAt
	tx.UpdatedAt = time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		UPDATE app_gas_transactions
		SET type = $2, amount = $3, net_amount = $4, status = $5, blockchain_tx_id = $6, from_address = $7, to_address = $8, notes = $9, error = $10, completed_at = $11, updated_at = $12
		WHERE id = $1
	`, tx.ID, tx.Type, tx.Amount, tx.NetAmount, tx.Status, toNullString(tx.BlockchainTxID), toNullString(tx.FromAddress), toNullString(tx.ToAddress), toNullString(tx.Notes), toNullString(tx.Error), toNullTime(tx.CompletedAt), tx.UpdatedAt)
	if err != nil {
		return gasbank.Transaction{}, err
	}
	return tx, nil
}

func (s *Store) GetGasTransaction(ctx context.Context, id string) (gasbank.Transaction, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, user_account_id, type, amount, net_amount, status, blockchain_tx_id, from_address, to_address, notes, error, completed_at, created_at, updated_at
		FROM app_gas_transactions
		WHERE id = $1
	`, id)

	tx, err := scanGasTransaction(row)
	if err != nil {
		return gasbank.Transaction{}, err
	}
	return tx, nil
}

func (s *Store) ListGasTransactions(ctx context.Context, gasAccountID string) ([]gasbank.Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, user_account_id, type, amount, net_amount, status, blockchain_tx_id, from_address, to_address, notes, error, completed_at, created_at, updated_at
		FROM app_gas_transactions
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, gasAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []gasbank.Transaction
	for rows.Next() {
		tx, err := scanGasTransaction(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}

func (s *Store) ListPendingWithdrawals(ctx context.Context) ([]gasbank.Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, user_account_id, type, amount, net_amount, status, blockchain_tx_id, from_address, to_address, notes, error, completed_at, created_at, updated_at
		FROM app_gas_transactions
		WHERE type = $1 AND status = $2
		ORDER BY created_at
	`, gasbank.TransactionWithdrawal, gasbank.StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []gasbank.Transaction
	for rows.Next() {
		tx, err := scanGasTransaction(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}

// --- AutomationStore --------------------------------------------------------

func (s *Store) CreateAutomationJob(ctx context.Context, job automation.Job) (automation.Job, error) {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_automation_jobs (id, account_id, function_id, name, description, schedule, enabled, last_run, next_run, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, job.ID, job.AccountID, job.FunctionID, job.Name, job.Description, job.Schedule, job.Enabled, toNullTime(job.LastRun), toNullTime(job.NextRun), job.CreatedAt, job.UpdatedAt)
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

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_automation_jobs
		SET name = $2, description = $3, schedule = $4, enabled = $5, last_run = $6, next_run = $7, updated_at = $8
		WHERE id = $1
	`, job.ID, job.Name, job.Description, job.Schedule, job.Enabled, toNullTime(job.LastRun), toNullTime(job.NextRun), job.UpdatedAt)
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

// --- PriceFeedStore ---------------------------------------------------------

func (s *Store) CreatePriceFeed(ctx context.Context, feed pricefeed.Feed) (pricefeed.Feed, error) {
	if feed.ID == "" {
		feed.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	feed.CreatedAt = now
	feed.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_price_feeds (id, account_id, base_asset, quote_asset, pair, update_interval, deviation_percent, heartbeat_interval, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, feed.ID, feed.AccountID, feed.BaseAsset, feed.QuoteAsset, feed.Pair, feed.UpdateInterval, feed.DeviationPercent, feed.Heartbeat, feed.Active, feed.CreatedAt, feed.UpdatedAt)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	return feed, nil
}

func (s *Store) UpdatePriceFeed(ctx context.Context, feed pricefeed.Feed) (pricefeed.Feed, error) {
	existing, err := s.GetPriceFeed(ctx, feed.ID)
	if err != nil {
		return pricefeed.Feed{}, err
	}

	feed.AccountID = existing.AccountID
	feed.BaseAsset = existing.BaseAsset
	feed.QuoteAsset = existing.QuoteAsset
	feed.Pair = existing.Pair
	feed.CreatedAt = existing.CreatedAt
	feed.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_price_feeds
		SET update_interval = $2, deviation_percent = $3, heartbeat_interval = $4, active = $5, updated_at = $6
		WHERE id = $1
	`, feed.ID, feed.UpdateInterval, feed.DeviationPercent, feed.Heartbeat, feed.Active, feed.UpdatedAt)
	if err != nil {
		return pricefeed.Feed{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return pricefeed.Feed{}, sql.ErrNoRows
	}
	return feed, nil
}

func (s *Store) GetPriceFeed(ctx context.Context, id string) (pricefeed.Feed, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, base_asset, quote_asset, pair, update_interval, deviation_percent, heartbeat_interval, active, created_at, updated_at
		FROM app_price_feeds
		WHERE id = $1
	`, id)

	var feed pricefeed.Feed
	if err := row.Scan(&feed.ID, &feed.AccountID, &feed.BaseAsset, &feed.QuoteAsset, &feed.Pair, &feed.UpdateInterval, &feed.DeviationPercent, &feed.Heartbeat, &feed.Active, &feed.CreatedAt, &feed.UpdatedAt); err != nil {
		return pricefeed.Feed{}, err
	}
	return feed, nil
}

func (s *Store) ListPriceFeeds(ctx context.Context, accountID string) ([]pricefeed.Feed, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, base_asset, quote_asset, pair, update_interval, deviation_percent, heartbeat_interval, active, created_at, updated_at
		FROM app_price_feeds
		WHERE $1 = '' OR account_id = $1
		ORDER BY created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []pricefeed.Feed
	for rows.Next() {
		var feed pricefeed.Feed
		if err := rows.Scan(&feed.ID, &feed.AccountID, &feed.BaseAsset, &feed.QuoteAsset, &feed.Pair, &feed.UpdateInterval, &feed.DeviationPercent, &feed.Heartbeat, &feed.Active, &feed.CreatedAt, &feed.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, feed)
	}
	return result, rows.Err()
}

func (s *Store) CreatePriceSnapshot(ctx context.Context, snap pricefeed.Snapshot) (pricefeed.Snapshot, error) {
	if snap.ID == "" {
		snap.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	snap.CreatedAt = now
	if snap.CollectedAt.IsZero() {
		snap.CollectedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_price_feed_snapshots (id, feed_id, price, source, collected_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, snap.ID, snap.FeedID, snap.Price, snap.Source, snap.CollectedAt, snap.CreatedAt)
	if err != nil {
		return pricefeed.Snapshot{}, err
	}
	return snap, nil
}

func (s *Store) ListPriceSnapshots(ctx context.Context, feedID string) ([]pricefeed.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, feed_id, price, source, collected_at, created_at
		FROM app_price_feed_snapshots
		WHERE feed_id = $1
		ORDER BY collected_at DESC
	`, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []pricefeed.Snapshot
	for rows.Next() {
		var snap pricefeed.Snapshot
		if err := rows.Scan(&snap.ID, &snap.FeedID, &snap.Price, &snap.Source, &snap.CollectedAt, &snap.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, snap)
	}
	return result, rows.Err()
}

func (s *Store) ListPendingRequests(ctx context.Context) ([]oracle.Request, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, data_source_id, status, payload, result, error, created_at, updated_at, completed_at
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
		if err := rows.Scan(&req.ID, &req.AccountID, &req.DataSourceID, &req.Status, &req.Payload, &req.Result, &req.Error, &req.CreatedAt, &req.UpdatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			req.CompletedAt = completedAt.Time
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

// --- SecretStore -----------------------------------------------------------

func (s *Store) CreateSecret(ctx context.Context, sec secret.Secret) (secret.Secret, error) {
	if sec.ID == "" {
		sec.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	sec.CreatedAt = now
	sec.UpdatedAt = now
	sec.Version = 1

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_secrets (id, account_id, name, value, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sec.ID, sec.AccountID, sec.Name, sec.Value, sec.Version, sec.CreatedAt, sec.UpdatedAt)
	if err != nil {
		return secret.Secret{}, err
	}
	return sec, nil
}

func (s *Store) UpdateSecret(ctx context.Context, sec secret.Secret) (secret.Secret, error) {
	existing, err := s.GetSecret(ctx, sec.AccountID, sec.Name)
	if err != nil {
		return secret.Secret{}, err
	}

	sec.ID = existing.ID
	sec.AccountID = existing.AccountID
	sec.Name = existing.Name
	sec.CreatedAt = existing.CreatedAt
	sec.Version = existing.Version + 1
	sec.UpdatedAt = time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		UPDATE app_secrets
		SET value = $1, version = $2, updated_at = $3
		WHERE id = $4
	`, sec.Value, sec.Version, sec.UpdatedAt, sec.ID)
	if err != nil {
		return secret.Secret{}, err
	}
	return sec, nil
}

func (s *Store) GetSecret(ctx context.Context, accountID, name string) (secret.Secret, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, value, version, created_at, updated_at
		FROM app_secrets
		WHERE account_id = $1 AND lower(name) = lower($2)
	`, accountID, name)

	sec, err := scanSecret(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return secret.Secret{}, fmt.Errorf("secret %s not found for account %s", name, accountID)
		}
		return secret.Secret{}, err
	}
	return sec, nil
}

func (s *Store) ListSecrets(ctx context.Context, accountID string) ([]secret.Secret, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, value, version, created_at, updated_at
		FROM app_secrets
		WHERE account_id = $1
		ORDER BY created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []secret.Secret
	for rows.Next() {
		sec, err := scanSecret(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, sec)
	}
	return result, rows.Err()
}

func (s *Store) DeleteSecret(ctx context.Context, accountID, name string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM app_secrets
		WHERE account_id = $1 AND lower(name) = lower($2)
	`, accountID, name)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("secret %s not found for account %s", name, accountID)
	}
	return nil
}

// --- OracleStore ------------------------------------------------------------

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

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_oracle_sources (id, account_id, name, description, url, method, headers, body, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, src.ID, src.AccountID, src.Name, src.Description, src.URL, src.Method, headersJSON, src.Body, src.Enabled, src.CreatedAt, src.UpdatedAt)
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

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_oracle_sources
		SET name = $2, description = $3, url = $4, method = $5, headers = $6, body = $7, enabled = $8, updated_at = $9
		WHERE id = $1
	`, src.ID, src.Name, src.Description, src.URL, src.Method, headersJSON, src.Body, src.Enabled, src.UpdatedAt)
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

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_oracle_requests (id, account_id, data_source_id, status, payload, result, error, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, req.ID, req.AccountID, req.DataSourceID, req.Status, req.Payload, req.Result, req.Error, req.CreatedAt, req.UpdatedAt, toNullTime(req.CompletedAt))
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

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_oracle_requests
		SET status = $2, payload = $3, result = $4, error = $5, updated_at = $6, completed_at = $7
		WHERE id = $1
	`, req.ID, req.Status, req.Payload, req.Result, req.Error, req.UpdatedAt, toNullTime(req.CompletedAt))
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
		SELECT id, account_id, data_source_id, status, payload, result, error, created_at, updated_at, completed_at
		FROM app_oracle_requests
		WHERE id = $1
	`, id)

	var (
		req         oracle.Request
		completedAt sql.NullTime
	)
	if err := row.Scan(&req.ID, &req.AccountID, &req.DataSourceID, &req.Status, &req.Payload, &req.Result, &req.Error, &req.CreatedAt, &req.UpdatedAt, &completedAt); err != nil {
		return oracle.Request{}, err
	}
	if completedAt.Valid {
		req.CompletedAt = completedAt.Time.UTC()
	}
	return req, nil
}

func (s *Store) ListRequests(ctx context.Context, accountID string) ([]oracle.Request, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, data_source_id, status, payload, result, error, created_at, updated_at, completed_at
		FROM app_oracle_requests
		WHERE $1 = '' OR account_id = $1
		ORDER BY created_at DESC
	`, accountID)
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
		if err := rows.Scan(&req.ID, &req.AccountID, &req.DataSourceID, &req.Status, &req.Payload, &req.Result, &req.Error, &req.CreatedAt, &req.UpdatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			req.CompletedAt = completedAt.Time.UTC()
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFunctionExecution(scanner rowScanner) (function.Execution, error) {
	var (
		exec        function.Execution
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
		return function.Execution{}, err
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

func scanGasAccount(scanner rowScanner) (gasbank.Account, error) {
	var (
		acct         gasbank.Account
		wallet       sql.NullString
		lastWithdraw sql.NullTime
		createdAt    time.Time
		updatedAt    time.Time
	)
	if err := scanner.Scan(&acct.ID, &acct.AccountID, &wallet, &acct.Balance, &acct.Available, &acct.Pending, &acct.DailyWithdrawal, &lastWithdraw, &createdAt, &updatedAt); err != nil {
		return gasbank.Account{}, err
	}
	if wallet.Valid {
		acct.WalletAddress = normalizeWallet(wallet.String)
	}
	if lastWithdraw.Valid {
		acct.LastWithdrawal = lastWithdraw.Time.UTC()
	}
	acct.CreatedAt = createdAt.UTC()
	acct.UpdatedAt = updatedAt.UTC()
	return acct, nil
}

func scanGasTransaction(scanner rowScanner) (gasbank.Transaction, error) {
	var (
		tx           gasbank.Transaction
		userAccount  sql.NullString
		blockchainID sql.NullString
		fromAddr     sql.NullString
		toAddr       sql.NullString
		notes        sql.NullString
		errMsg       sql.NullString
		completedAt  sql.NullTime
		createdAt    time.Time
		updatedAt    time.Time
	)
	if err := scanner.Scan(&tx.ID, &tx.AccountID, &userAccount, &tx.Type, &tx.Amount, &tx.NetAmount, &tx.Status, &blockchainID, &fromAddr, &toAddr, &notes, &errMsg, &completedAt, &createdAt, &updatedAt); err != nil {
		return gasbank.Transaction{}, err
	}
	if userAccount.Valid {
		tx.UserAccountID = userAccount.String
	}
	if blockchainID.Valid {
		tx.BlockchainTxID = blockchainID.String
	}
	if fromAddr.Valid {
		tx.FromAddress = fromAddr.String
	}
	if toAddr.Valid {
		tx.ToAddress = toAddr.String
	}
	if notes.Valid {
		tx.Notes = notes.String
	}
	if errMsg.Valid {
		tx.Error = errMsg.String
	}
	if completedAt.Valid {
		tx.CompletedAt = completedAt.Time.UTC()
	}
	tx.CreatedAt = createdAt.UTC()
	tx.UpdatedAt = updatedAt.UTC()
	return tx, nil
}

func scanSecret(scanner rowScanner) (secret.Secret, error) {
	var (
		sec       secret.Secret
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&sec.ID, &sec.AccountID, &sec.Name, &sec.Value, &sec.Version, &createdAt, &updatedAt); err != nil {
		return secret.Secret{}, err
	}
	sec.CreatedAt = createdAt.UTC()
	sec.UpdatedAt = updatedAt.UTC()
	return sec, nil
}

func toNullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func normalizeWallet(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

func toNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}
