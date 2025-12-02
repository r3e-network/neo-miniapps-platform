package gasbank

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// PostgresStore implements Store using PostgreSQL.
// This is the service-local store implementation that uses the generic
// database connection provided by the Service Engine.
type PostgresStore struct {
	db       *sql.DB
	accounts AccountChecker
}

// NewPostgresStore creates a new PostgreSQL-backed Gas Bank store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

// CreateGasAccount creates a new gas account.
func (s *PostgresStore) CreateGasAccount(ctx context.Context, acct GasBankAccount) (GasBankAccount, error) {
	if acct.ID == "" {
		acct.ID = uuid.NewString()
	}
	acct.AccountID = strings.TrimSpace(acct.AccountID)
	acct.WalletAddress = normalizeWallet(acct.WalletAddress)
	flagsJSON, err := json.Marshal(core.MapOrEmptyBool(acct.Flags))
	if err != nil {
		return GasBankAccount{}, err
	}
	metaJSON, err := json.Marshal(core.MapOrEmptyString(acct.Metadata))
	if err != nil {
		return GasBankAccount{}, err
	}
	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now
	tenant := s.accounts.AccountTenant(ctx, acct.AccountID)

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO app_gas_accounts (
			id, account_id, wallet_address, balance, available, pending, locked,
			min_balance, daily_limit, daily_withdrawal, notification_threshold,
			required_approvals, flags, metadata, tenant, last_withdrawal, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18
		)
	`, acct.ID, acct.AccountID, core.ToNullString(acct.WalletAddress), acct.Balance, acct.Available, acct.Pending, acct.Locked,
		acct.MinBalance, acct.DailyLimit, acct.DailyWithdrawal, acct.NotificationThreshold,
		acct.RequiredApprovals, flagsJSON, metaJSON, tenant, core.ToNullTime(acct.LastWithdrawal), acct.CreatedAt, acct.UpdatedAt); err != nil {
		return GasBankAccount{}, err
	}
	return acct, nil
}

func (s *PostgresStore) UpdateGasAccount(ctx context.Context, acct GasBankAccount) (GasBankAccount, error) {
	existing, err := s.GetGasAccount(ctx, acct.ID)
	if err != nil {
		return GasBankAccount{}, err
	}

	acct.AccountID = existing.AccountID
	if acct.WalletAddress == "" {
		acct.WalletAddress = existing.WalletAddress
	}
	acct.WalletAddress = normalizeWallet(acct.WalletAddress)
	acct.CreatedAt = existing.CreatedAt
	acct.UpdatedAt = time.Now().UTC()
	tenant := s.accounts.AccountTenant(ctx, acct.AccountID)

	flagsJSON, err := json.Marshal(core.MapOrEmptyBool(acct.Flags))
	if err != nil {
		return GasBankAccount{}, err
	}
	metaJSON, err := json.Marshal(core.MapOrEmptyString(acct.Metadata))
	if err != nil {
		return GasBankAccount{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE app_gas_accounts
		SET wallet_address = $2,
		    balance = $3,
		    available = $4,
		    pending = $5,
		    locked = $6,
		    min_balance = $7,
		    daily_limit = $8,
		    daily_withdrawal = $9,
		    notification_threshold = $10,
		    required_approvals = $11,
		    flags = $12,
		    metadata = $13,
		    tenant = $14,
		    last_withdrawal = $15,
		    updated_at = $16
		WHERE id = $1
	`, acct.ID, core.ToNullString(acct.WalletAddress), acct.Balance, acct.Available, acct.Pending, acct.Locked, acct.MinBalance, acct.DailyLimit, acct.DailyWithdrawal, acct.NotificationThreshold, acct.RequiredApprovals, flagsJSON, metaJSON, tenant, core.ToNullTime(acct.LastWithdrawal), acct.UpdatedAt)
	if err != nil {
		return GasBankAccount{}, err
	}
	return acct, nil
}

func (s *PostgresStore) GetGasAccount(ctx context.Context, id string) (GasBankAccount, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, wallet_address, balance, available, pending, locked,
		       min_balance, daily_limit, daily_withdrawal, notification_threshold,
		       required_approvals, flags, metadata, last_withdrawal, created_at, updated_at
		FROM app_gas_accounts
		WHERE id = $1
	`, id)

	acct, err := scanGasAccount(row)
	if err != nil {
		return GasBankAccount{}, err
	}
	return acct, nil
}

func (s *PostgresStore) GetGasAccountByWallet(ctx context.Context, wallet string) (GasBankAccount, error) {
	wallet = normalizeWallet(wallet)
	if wallet == "" {
		return GasBankAccount{}, sql.ErrNoRows
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, wallet_address, balance, available, pending, locked,
		       min_balance, daily_limit, daily_withdrawal, notification_threshold,
		       required_approvals, flags, metadata, last_withdrawal, created_at, updated_at
		FROM app_gas_accounts
		WHERE wallet_address = $1
	`, wallet)

	acct, err := scanGasAccount(row)
	if err != nil {
		return GasBankAccount{}, err
	}
	return acct, nil
}

func (s *PostgresStore) ListGasAccounts(ctx context.Context, accountID string) ([]GasBankAccount, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, wallet_address, balance, available, pending, locked,
		       min_balance, daily_limit, daily_withdrawal, notification_threshold,
		       required_approvals, flags, metadata, last_withdrawal, created_at, updated_at
		FROM app_gas_accounts
		WHERE ($1 = '' OR account_id = $1) AND ($2 = '' OR tenant = $2)
		ORDER BY created_at
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []GasBankAccount
	for rows.Next() {
		acct, err := scanGasAccount(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, acct)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateGasTransaction(ctx context.Context, tx Transaction) (Transaction, error) {
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
	approvalJSON, err := json.Marshal(tx.ApprovalPolicy)
	if err != nil {
		return Transaction{}, err
	}
	metaJSON, err := json.Marshal(core.MapOrEmptyString(tx.Metadata))
	if err != nil {
		return Transaction{}, err
	}
	now := time.Now().UTC()
	tx.CreatedAt = now
	tx.UpdatedAt = now
	tenant := s.accountTenant(ctx, tx.AccountID)

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO app_gas_transactions (
			id, account_id, user_account_id, type, amount, net_amount, status,
			blockchain_tx_id, from_address, to_address, notes, error,
			schedule_at, cron_expression, approval_policy, resolver_attempt,
			resolver_error, last_attempt_at, next_attempt_at, dead_letter_reason,
			metadata, tenant, dispatched_at, resolved_at, completed_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27
		)
	`, tx.ID, tx.AccountID, core.ToNullString(tx.UserAccountID), tx.Type, tx.Amount, tx.NetAmount, tx.Status,
		core.ToNullString(tx.BlockchainTxID), core.ToNullString(tx.FromAddress), core.ToNullString(tx.ToAddress), core.ToNullString(tx.Notes), core.ToNullString(tx.Error),
		core.ToNullTime(tx.ScheduleAt), core.ToNullString(tx.CronExpression), approvalJSON, tx.ResolverAttempt,
		core.ToNullString(tx.ResolverError), core.ToNullTime(tx.LastAttemptAt), core.ToNullTime(tx.NextAttemptAt), core.ToNullString(tx.DeadLetterReason),
		metaJSON, tenant, core.ToNullTime(tx.DispatchedAt), core.ToNullTime(tx.ResolvedAt), core.ToNullTime(tx.CompletedAt), tx.CreatedAt, tx.UpdatedAt); err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

func (s *PostgresStore) UpdateGasTransaction(ctx context.Context, tx Transaction) (Transaction, error) {
	existing, err := s.GetGasTransaction(ctx, tx.ID)
	if err != nil {
		return Transaction{}, err
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
	approvalJSON, err := json.Marshal(tx.ApprovalPolicy)
	if err != nil {
		return Transaction{}, err
	}
	metaJSON, err := json.Marshal(core.MapOrEmptyString(tx.Metadata))
	if err != nil {
		return Transaction{}, err
	}
	tx.CreatedAt = existing.CreatedAt
	tx.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, tx.AccountID)

	_, err = s.db.ExecContext(ctx, `
		UPDATE app_gas_transactions
		SET type = $2,
		    amount = $3,
		    net_amount = $4,
		    status = $5,
		    blockchain_tx_id = $6,
		    from_address = $7,
		    to_address = $8,
		    notes = $9,
		    error = $10,
		    schedule_at = $11,
		    cron_expression = $12,
		    approval_policy = $13,
		    resolver_attempt = $14,
		    resolver_error = $15,
		    last_attempt_at = $16,
		    next_attempt_at = $17,
		    dead_letter_reason = $18,
		    metadata = $19,
		    tenant = $20,
		    dispatched_at = $21,
		    resolved_at = $22,
		    completed_at = $23,
		    updated_at = $24
		WHERE id = $1
	`, tx.ID, tx.Type, tx.Amount, tx.NetAmount, tx.Status, core.ToNullString(tx.BlockchainTxID), core.ToNullString(tx.FromAddress), core.ToNullString(tx.ToAddress), core.ToNullString(tx.Notes), core.ToNullString(tx.Error), core.ToNullTime(tx.ScheduleAt), core.ToNullString(tx.CronExpression), approvalJSON, tx.ResolverAttempt, core.ToNullString(tx.ResolverError), core.ToNullTime(tx.LastAttemptAt), core.ToNullTime(tx.NextAttemptAt), core.ToNullString(tx.DeadLetterReason), metaJSON, tenant, core.ToNullTime(tx.DispatchedAt), core.ToNullTime(tx.ResolvedAt), core.ToNullTime(tx.CompletedAt), tx.UpdatedAt)
	if err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

func (s *PostgresStore) GetGasTransaction(ctx context.Context, id string) (Transaction, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, user_account_id, type, amount, net_amount, status,
		       blockchain_tx_id, from_address, to_address, notes, error,
		       schedule_at, cron_expression, approval_policy, resolver_attempt,
		       resolver_error, last_attempt_at, next_attempt_at, dead_letter_reason,
		       metadata, dispatched_at, resolved_at, completed_at, created_at, updated_at
		FROM app_gas_transactions
		WHERE id = $1
	`, id)

	tx, err := scanGasTransaction(row)
	if err != nil {
		return Transaction{}, err
	}
	if err := s.hydrateTransaction(ctx, &tx); err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

func (s *PostgresStore) ListGasTransactions(ctx context.Context, gasAccountID string, limit int) ([]Transaction, error) {
	tenant := s.gasAccountTenant(ctx, gasAccountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, user_account_id, type, amount, net_amount, status,
		       blockchain_tx_id, from_address, to_address, notes, error,
		       schedule_at, cron_expression, approval_policy, resolver_attempt,
		       resolver_error, last_attempt_at, next_attempt_at, dead_letter_reason,
		       metadata, dispatched_at, resolved_at, completed_at, created_at, updated_at
		FROM app_gas_transactions
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, gasAccountID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Transaction
	for rows.Next() {
		tx, err := scanGasTransaction(rows)
		if err != nil {
			return nil, err
		}
		if err := s.hydrateTransaction(ctx, &tx); err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}

func (s *PostgresStore) ListPendingWithdrawals(ctx context.Context) ([]Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, user_account_id, type, amount, net_amount, status,
		       blockchain_tx_id, from_address, to_address, notes, error,
		       schedule_at, cron_expression, approval_policy, resolver_attempt,
		       resolver_error, last_attempt_at, next_attempt_at, dead_letter_reason,
		       metadata, dispatched_at, resolved_at, completed_at, created_at, updated_at
		FROM app_gas_transactions
		WHERE type = $1 AND status = $2
		ORDER BY created_at
	`, TransactionWithdrawal, StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Transaction
	for rows.Next() {
		tx, err := scanGasTransaction(rows)
		if err != nil {
			return nil, err
		}
		if err := s.hydrateTransaction(ctx, &tx); err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}

func (s *PostgresStore) UpsertWithdrawalApproval(ctx context.Context, approval WithdrawalApproval) (WithdrawalApproval, error) {
	now := time.Now().UTC()
	if approval.CreatedAt.IsZero() {
		approval.CreatedAt = now
	}
	approval.UpdatedAt = now
	tenant := s.gasTransactionTenant(ctx, approval.TransactionID)

	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO app_gas_withdrawal_approvals (transaction_id, approver, status, signature, note, decided_at, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (transaction_id, approver)
		DO UPDATE SET status = EXCLUDED.status,
		               signature = EXCLUDED.signature,
		               note = EXCLUDED.note,
		               decided_at = EXCLUDED.decided_at,
		               tenant = EXCLUDED.tenant,
		               updated_at = EXCLUDED.updated_at
		RETURNING created_at
	`, approval.TransactionID, approval.Approver, approval.Status, core.ToNullString(approval.Signature), core.ToNullString(approval.Note), core.ToNullTime(approval.DecidedAt), tenant, approval.CreatedAt, approval.UpdatedAt).Scan(&createdAt)
	if err != nil {
		return WithdrawalApproval{}, err
	}
	approval.CreatedAt = createdAt.UTC()
	return approval, nil
}

func (s *PostgresStore) ListWithdrawalApprovals(ctx context.Context, transactionID string) ([]WithdrawalApproval, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT transaction_id, approver, status, signature, note, decided_at, created_at, updated_at
		FROM app_gas_withdrawal_approvals
		WHERE transaction_id = $1
		ORDER BY decided_at DESC NULLS LAST, updated_at DESC
	`, transactionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WithdrawalApproval
	for rows.Next() {
		approval, err := scanWithdrawalApproval(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, approval)
	}
	return result, rows.Err()
}

func (s *PostgresStore) SaveWithdrawalSchedule(ctx context.Context, schedule WithdrawalSchedule) (WithdrawalSchedule, error) {
	now := time.Now().UTC()
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = now
	}
	schedule.UpdatedAt = now
	tenant := s.gasTransactionTenant(ctx, schedule.TransactionID)

	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO app_gas_withdrawal_schedules (transaction_id, schedule_at, cron_expression, next_run_at, last_run_at, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (transaction_id)
		DO UPDATE SET schedule_at = EXCLUDED.schedule_at,
		               cron_expression = EXCLUDED.cron_expression,
		               next_run_at = EXCLUDED.next_run_at,
		               last_run_at = EXCLUDED.last_run_at,
		               tenant = EXCLUDED.tenant,
		               updated_at = EXCLUDED.updated_at
		RETURNING created_at
	`, schedule.TransactionID, core.ToNullTime(schedule.ScheduleAt), core.ToNullString(schedule.CronExpression), core.ToNullTime(schedule.NextRunAt), core.ToNullTime(schedule.LastRunAt), tenant, schedule.CreatedAt, schedule.UpdatedAt).Scan(&createdAt)
	if err != nil {
		return WithdrawalSchedule{}, err
	}
	schedule.CreatedAt = createdAt.UTC()
	return schedule, nil
}

func (s *PostgresStore) GetWithdrawalSchedule(ctx context.Context, transactionID string) (WithdrawalSchedule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT transaction_id, schedule_at, cron_expression, next_run_at, last_run_at, created_at, updated_at
		FROM app_gas_withdrawal_schedules
		WHERE transaction_id = $1
	`, transactionID)
	return scanWithdrawalSchedule(row)
}

func (s *PostgresStore) DeleteWithdrawalSchedule(ctx context.Context, transactionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_gas_withdrawal_schedules WHERE transaction_id = $1`, transactionID)
	return err
}

func (s *PostgresStore) ListDueWithdrawalSchedules(ctx context.Context, before time.Time, limit int) ([]WithdrawalSchedule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT transaction_id, schedule_at, cron_expression, next_run_at, last_run_at, created_at, updated_at
		FROM app_gas_withdrawal_schedules
		WHERE schedule_at IS NOT NULL AND schedule_at <= $1
		ORDER BY schedule_at
		LIMIT $2
	`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WithdrawalSchedule
	for rows.Next() {
		schedule, err := scanWithdrawalSchedule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, schedule)
	}
	return result, rows.Err()
}

func (s *PostgresStore) RecordSettlementAttempt(ctx context.Context, attempt SettlementAttempt) (SettlementAttempt, error) {
	if attempt.TransactionID == "" {
		return SettlementAttempt{}, fmt.Errorf("transaction id required")
	}
	if attempt.Attempt <= 0 {
		if err := s.db.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(attempt), 0) + 1 FROM app_gas_settlement_attempts WHERE transaction_id = $1
		`, attempt.TransactionID).Scan(&attempt.Attempt); err != nil {
			return SettlementAttempt{}, err
		}
	}
	now := time.Now().UTC()
	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = now
	}
	if attempt.CompletedAt.IsZero() {
		attempt.CompletedAt = now
	}
	latency := attempt.Latency
	if latency == 0 && !attempt.CompletedAt.IsZero() {
		latency = attempt.CompletedAt.Sub(attempt.StartedAt)
	}
	tenant := s.gasTransactionTenant(ctx, attempt.TransactionID)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_gas_settlement_attempts (transaction_id, attempt, started_at, completed_at, latency_ms, status, error, tenant)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (transaction_id, attempt)
		DO UPDATE SET started_at = EXCLUDED.started_at,
		               completed_at = EXCLUDED.completed_at,
		               latency_ms = EXCLUDED.latency_ms,
		               status = EXCLUDED.status,
		               error = EXCLUDED.error,
		               tenant = EXCLUDED.tenant
	`, attempt.TransactionID, attempt.Attempt, attempt.StartedAt, attempt.CompletedAt, durationToMillis(latency), core.ToNullString(attempt.Status), core.ToNullString(attempt.Error), tenant)
	if err != nil {
		return SettlementAttempt{}, err
	}
	attempt.Latency = latency
	return attempt, nil
}

func (s *PostgresStore) ListSettlementAttempts(ctx context.Context, transactionID string, limit int) ([]SettlementAttempt, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT transaction_id, attempt, started_at, completed_at, latency_ms, status, error
		FROM app_gas_settlement_attempts
		WHERE transaction_id = $1
		ORDER BY attempt DESC
		LIMIT $2
	`, transactionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SettlementAttempt
	for rows.Next() {
		attempt, err := scanSettlementAttempt(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, attempt)
	}
	return result, rows.Err()
}

func (s *PostgresStore) UpsertDeadLetter(ctx context.Context, entry DeadLetter) (DeadLetter, error) {
	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	tenant := s.accountTenant(ctx, entry.AccountID)

	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO app_gas_dead_letters (transaction_id, account_id, reason, last_error, last_attempt_at, retries, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (transaction_id)
		DO UPDATE SET account_id = EXCLUDED.account_id,
		               reason = EXCLUDED.reason,
		               last_error = EXCLUDED.last_error,
		               last_attempt_at = EXCLUDED.last_attempt_at,
		               retries = EXCLUDED.retries,
		               tenant = EXCLUDED.tenant,
		               updated_at = EXCLUDED.updated_at
		RETURNING created_at
	`, entry.TransactionID, entry.AccountID, entry.Reason, core.ToNullString(entry.LastError), core.ToNullTime(entry.LastAttemptAt), entry.Retries, tenant, entry.CreatedAt, entry.UpdatedAt).Scan(&createdAt)
	if err != nil {
		return DeadLetter{}, err
	}
	entry.CreatedAt = createdAt.UTC()
	return entry, nil
}

func (s *PostgresStore) GetDeadLetter(ctx context.Context, transactionID string) (DeadLetter, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT transaction_id, account_id, reason, last_error, last_attempt_at, retries, created_at, updated_at
		FROM app_gas_dead_letters
		WHERE transaction_id = $1
	`, transactionID)
	return scanDeadLetter(row)
}

func (s *PostgresStore) ListDeadLetters(ctx context.Context, accountID string, limit int) ([]DeadLetter, error) {
	if limit <= 0 {
		limit = 50
	}
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT transaction_id, account_id, reason, last_error, last_attempt_at, retries, created_at, updated_at
		FROM app_gas_dead_letters
		WHERE ($1 = '' OR account_id = $1) AND ($2 = '' OR tenant = $2)
		ORDER BY updated_at DESC
		LIMIT $3
	`, accountID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DeadLetter
	for rows.Next() {
		entry, err := scanDeadLetter(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (s *PostgresStore) RemoveDeadLetter(ctx context.Context, transactionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_gas_dead_letters WHERE transaction_id = $1`, transactionID)
	return err
}

// gasAccountTenant fetches the tenant recorded on a gas account.
func (s *PostgresStore) gasAccountTenant(ctx context.Context, gasAccountID string) string {
	var tenant sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT tenant FROM app_gas_accounts WHERE id = $1
	`, gasAccountID).Scan(&tenant)
	if tenant.Valid {
		return tenant.String
	}
	return ""
}

// gasTransactionTenant returns the tenant for the account that owns the transaction ID.
func (s *PostgresStore) gasTransactionTenant(ctx context.Context, txID string) string {
	var accountID sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT account_id FROM app_gas_transactions WHERE id = $1
	`, txID).Scan(&accountID)
	if accountID.Valid {
		return s.accounts.AccountTenant(ctx, accountID.String)
	}
	return ""
}



func durationToMillis(d time.Duration) int64 {
	return d.Milliseconds()
}

func scanWithdrawalApproval(scanner core.RowScanner) (WithdrawalApproval, error) {
	var (
		approval  WithdrawalApproval
		signature sql.NullString
		note      sql.NullString
		decidedAt sql.NullTime
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&approval.TransactionID, &approval.Approver, &approval.Status, &signature, &note, &decidedAt, &createdAt, &updatedAt); err != nil {
		return WithdrawalApproval{}, err
	}
	if signature.Valid {
		approval.Signature = signature.String
	}
	if note.Valid {
		approval.Note = note.String
	}
	if decidedAt.Valid {
		approval.DecidedAt = decidedAt.Time.UTC()
	}
	approval.CreatedAt = createdAt.UTC()
	approval.UpdatedAt = updatedAt.UTC()
	return approval, nil
}

func scanWithdrawalSchedule(scanner core.RowScanner) (WithdrawalSchedule, error) {
	var (
		schedule   WithdrawalSchedule
		scheduleAt sql.NullTime
		cronExpr   sql.NullString
		nextRun    sql.NullTime
		lastRun    sql.NullTime
		createdAt  time.Time
		updatedAt  time.Time
	)
	if err := scanner.Scan(&schedule.TransactionID, &scheduleAt, &cronExpr, &nextRun, &lastRun, &createdAt, &updatedAt); err != nil {
		return WithdrawalSchedule{}, err
	}
	if scheduleAt.Valid {
		schedule.ScheduleAt = scheduleAt.Time.UTC()
	}
	if cronExpr.Valid {
		schedule.CronExpression = cronExpr.String
	}
	if nextRun.Valid {
		schedule.NextRunAt = nextRun.Time.UTC()
	}
	if lastRun.Valid {
		schedule.LastRunAt = lastRun.Time.UTC()
	}
	schedule.CreatedAt = createdAt.UTC()
	schedule.UpdatedAt = updatedAt.UTC()
	return schedule, nil
}

func scanSettlementAttempt(scanner core.RowScanner) (SettlementAttempt, error) {
	var (
		attempt     SettlementAttempt
		completedAt sql.NullTime
		latencyMS   sql.NullInt64
		status      sql.NullString
		errMsg      sql.NullString
	)
	if err := scanner.Scan(&attempt.TransactionID, &attempt.Attempt, &attempt.StartedAt, &completedAt, &latencyMS, &status, &errMsg); err != nil {
		return SettlementAttempt{}, err
	}
	attempt.StartedAt = attempt.StartedAt.UTC()
	if completedAt.Valid {
		attempt.CompletedAt = completedAt.Time.UTC()
	}
	if latencyMS.Valid {
		attempt.Latency = time.Duration(latencyMS.Int64) * time.Millisecond
	}
	if status.Valid {
		attempt.Status = status.String
	}
	if errMsg.Valid {
		attempt.Error = errMsg.String
	}
	return attempt, nil
}

func scanDeadLetter(scanner core.RowScanner) (DeadLetter, error) {
	var (
		entry       DeadLetter
		lastError   sql.NullString
		lastAttempt sql.NullTime
		createdAt   time.Time
		updatedAt   time.Time
	)
	if err := scanner.Scan(&entry.TransactionID, &entry.AccountID, &entry.Reason, &lastError, &lastAttempt, &entry.Retries, &createdAt, &updatedAt); err != nil {
		return DeadLetter{}, err
	}
	if lastError.Valid {
		entry.LastError = lastError.String
	}
	if lastAttempt.Valid {
		entry.LastAttemptAt = lastAttempt.Time.UTC()
	}
	entry.CreatedAt = createdAt.UTC()
	entry.UpdatedAt = updatedAt.UTC()
	return entry, nil
}

func (s *PostgresStore) hydrateTransaction(ctx context.Context, tx *Transaction) error {
	if tx.Type != TransactionWithdrawal {
		return nil
	}
	approvals, err := s.ListWithdrawalApprovals(ctx, tx.ID)
	if err != nil {
		return err
	}
	tx.Approvals = approvals
	return nil
}

// Helper functions

func (s *PostgresStore) accountTenant(ctx context.Context, accountID string) string {
	return s.accounts.AccountTenant(ctx, accountID)
}

func normalizeWallet(addr string) string {
	return normalizeWalletAddress(addr)
}


func scanGasAccount(scanner core.RowScanner) (GasBankAccount, error) {
	var (
		acct         GasBankAccount
		wallet       sql.NullString
		lastWithdraw sql.NullTime
		flagsJSON    []byte
		metaJSON     []byte
		createdAt    time.Time
		updatedAt    time.Time
	)
	if err := scanner.Scan(
		&acct.ID,
		&acct.AccountID,
		&wallet,
		&acct.Balance,
		&acct.Available,
		&acct.Pending,
		&acct.Locked,
		&acct.MinBalance,
		&acct.DailyLimit,
		&acct.DailyWithdrawal,
		&acct.NotificationThreshold,
		&acct.RequiredApprovals,
		&flagsJSON,
		&metaJSON,
		&lastWithdraw,
		&createdAt,
		&updatedAt,
	); err != nil {
		return GasBankAccount{}, err
	}
	if wallet.Valid {
		acct.WalletAddress = normalizeWallet(wallet.String)
	}
	if lastWithdraw.Valid {
		acct.LastWithdrawal = lastWithdraw.Time.UTC()
	}
	if len(flagsJSON) > 0 {
		var flags map[string]bool
		if err := json.Unmarshal(flagsJSON, &flags); err == nil {
			acct.Flags = flags
		}
	}
	if len(metaJSON) > 0 {
		var metadata map[string]string
		if err := json.Unmarshal(metaJSON, &metadata); err == nil {
			acct.Metadata = metadata
		}
	}
	acct.CreatedAt = createdAt.UTC()
	acct.UpdatedAt = updatedAt.UTC()
	return acct, nil
}

func scanGasTransaction(scanner core.RowScanner) (Transaction, error) {
	var (
		tx           Transaction
		userAccount  sql.NullString
		blockchainID sql.NullString
		fromAddr     sql.NullString
		toAddr       sql.NullString
		notes        sql.NullString
		errMsg       sql.NullString
		scheduleAt   sql.NullTime
		cronExpr     sql.NullString
		approvalJSON []byte
		resolverErr  sql.NullString
		lastAttempt  sql.NullTime
		nextAttempt  sql.NullTime
		deadLetter   sql.NullString
		metadataJSON []byte
		dispatchedAt sql.NullTime
		resolvedAt   sql.NullTime
		completedAt  sql.NullTime
		createdAt    time.Time
		updatedAt    time.Time
	)
	if err := scanner.Scan(
		&tx.ID,
		&tx.AccountID,
		&userAccount,
		&tx.Type,
		&tx.Amount,
		&tx.NetAmount,
		&tx.Status,
		&blockchainID,
		&fromAddr,
		&toAddr,
		&notes,
		&errMsg,
		&scheduleAt,
		&cronExpr,
		&approvalJSON,
		&tx.ResolverAttempt,
		&resolverErr,
		&lastAttempt,
		&nextAttempt,
		&deadLetter,
		&metadataJSON,
		&dispatchedAt,
		&resolvedAt,
		&completedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Transaction{}, err
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
	if scheduleAt.Valid {
		tx.ScheduleAt = scheduleAt.Time.UTC()
	}
	if cronExpr.Valid {
		tx.CronExpression = cronExpr.String
	}
	if len(approvalJSON) > 0 {
		var policy ApprovalPolicy
		if err := json.Unmarshal(approvalJSON, &policy); err == nil {
			tx.ApprovalPolicy = policy
		}
	}
	if resolverErr.Valid {
		tx.ResolverError = resolverErr.String
	}
	if lastAttempt.Valid {
		tx.LastAttemptAt = lastAttempt.Time.UTC()
	}
	if nextAttempt.Valid {
		tx.NextAttemptAt = nextAttempt.Time.UTC()
	}
	if deadLetter.Valid {
		tx.DeadLetterReason = deadLetter.String
	}
	if len(metadataJSON) > 0 {
		var metadata map[string]string
		if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
			tx.Metadata = metadata
		}
	}
	if dispatchedAt.Valid {
		tx.DispatchedAt = dispatchedAt.Time.UTC()
	}
	if resolvedAt.Valid {
		tx.ResolvedAt = resolvedAt.Time.UTC()
	}
	if completedAt.Valid {
		tx.CompletedAt = completedAt.Time.UTC()
	}
	tx.CreatedAt = createdAt.UTC()
	tx.UpdatedAt = updatedAt.UTC()
	return tx, nil
}
