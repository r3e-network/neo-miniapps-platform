package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
)

// GasBankStore implementation

func (s *Store) CreateGasAccount(ctx context.Context, acct gasbank.Account) (gasbank.Account, error) {
	if acct.ID == "" {
		acct.ID = uuid.NewString()
	}
	acct.AccountID = strings.TrimSpace(acct.AccountID)
	acct.WalletAddress = normalizeWallet(acct.WalletAddress)
	flagsJSON, err := json.Marshal(mapOrEmptyBool(acct.Flags))
	if err != nil {
		return gasbank.Account{}, err
	}
	metaJSON, err := json.Marshal(mapOrEmptyString(acct.Metadata))
	if err != nil {
		return gasbank.Account{}, err
	}
	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now
	tenant := s.accountTenant(ctx, acct.AccountID)

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
	`, acct.ID, acct.AccountID, toNullString(acct.WalletAddress), acct.Balance, acct.Available, acct.Pending, acct.Locked,
		acct.MinBalance, acct.DailyLimit, acct.DailyWithdrawal, acct.NotificationThreshold,
		acct.RequiredApprovals, flagsJSON, metaJSON, tenant, toNullTime(acct.LastWithdrawal), acct.CreatedAt, acct.UpdatedAt); err != nil {
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
	tenant := s.accountTenant(ctx, acct.AccountID)

	flagsJSON, err := json.Marshal(mapOrEmptyBool(acct.Flags))
	if err != nil {
		return gasbank.Account{}, err
	}
	metaJSON, err := json.Marshal(mapOrEmptyString(acct.Metadata))
	if err != nil {
		return gasbank.Account{}, err
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
	`, acct.ID, toNullString(acct.WalletAddress), acct.Balance, acct.Available, acct.Pending, acct.Locked, acct.MinBalance, acct.DailyLimit, acct.DailyWithdrawal, acct.NotificationThreshold, acct.RequiredApprovals, flagsJSON, metaJSON, tenant, toNullTime(acct.LastWithdrawal), acct.UpdatedAt)
	if err != nil {
		return gasbank.Account{}, err
	}
	return acct, nil
}

func (s *Store) GetGasAccount(ctx context.Context, id string) (gasbank.Account, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, wallet_address, balance, available, pending, locked,
		       min_balance, daily_limit, daily_withdrawal, notification_threshold,
		       required_approvals, flags, metadata, last_withdrawal, created_at, updated_at
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
		SELECT id, account_id, wallet_address, balance, available, pending, locked,
		       min_balance, daily_limit, daily_withdrawal, notification_threshold,
		       required_approvals, flags, metadata, last_withdrawal, created_at, updated_at
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
	tenant := s.accountTenant(ctx, accountID)
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
	approvalJSON, err := json.Marshal(tx.ApprovalPolicy)
	if err != nil {
		return gasbank.Transaction{}, err
	}
	metaJSON, err := json.Marshal(mapOrEmptyString(tx.Metadata))
	if err != nil {
		return gasbank.Transaction{}, err
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
	`, tx.ID, tx.AccountID, toNullString(tx.UserAccountID), tx.Type, tx.Amount, tx.NetAmount, tx.Status,
		toNullString(tx.BlockchainTxID), toNullString(tx.FromAddress), toNullString(tx.ToAddress), toNullString(tx.Notes), toNullString(tx.Error),
		toNullTime(tx.ScheduleAt), toNullString(tx.CronExpression), approvalJSON, tx.ResolverAttempt,
		toNullString(tx.ResolverError), toNullTime(tx.LastAttemptAt), toNullTime(tx.NextAttemptAt), toNullString(tx.DeadLetterReason),
		metaJSON, tenant, toNullTime(tx.DispatchedAt), toNullTime(tx.ResolvedAt), toNullTime(tx.CompletedAt), tx.CreatedAt, tx.UpdatedAt); err != nil {
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
	approvalJSON, err := json.Marshal(tx.ApprovalPolicy)
	if err != nil {
		return gasbank.Transaction{}, err
	}
	metaJSON, err := json.Marshal(mapOrEmptyString(tx.Metadata))
	if err != nil {
		return gasbank.Transaction{}, err
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
	`, tx.ID, tx.Type, tx.Amount, tx.NetAmount, tx.Status, toNullString(tx.BlockchainTxID), toNullString(tx.FromAddress), toNullString(tx.ToAddress), toNullString(tx.Notes), toNullString(tx.Error), toNullTime(tx.ScheduleAt), toNullString(tx.CronExpression), approvalJSON, tx.ResolverAttempt, toNullString(tx.ResolverError), toNullTime(tx.LastAttemptAt), toNullTime(tx.NextAttemptAt), toNullString(tx.DeadLetterReason), metaJSON, tenant, toNullTime(tx.DispatchedAt), toNullTime(tx.ResolvedAt), toNullTime(tx.CompletedAt), tx.UpdatedAt)
	if err != nil {
		return gasbank.Transaction{}, err
	}
	return tx, nil
}

func (s *Store) GetGasTransaction(ctx context.Context, id string) (gasbank.Transaction, error) {
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
		return gasbank.Transaction{}, err
	}
	if err := s.hydrateTransaction(ctx, &tx); err != nil {
		return gasbank.Transaction{}, err
	}
	return tx, nil
}

func (s *Store) ListGasTransactions(ctx context.Context, gasAccountID string, limit int) ([]gasbank.Transaction, error) {
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

	var result []gasbank.Transaction
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

func (s *Store) ListPendingWithdrawals(ctx context.Context) ([]gasbank.Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, user_account_id, type, amount, net_amount, status,
		       blockchain_tx_id, from_address, to_address, notes, error,
		       schedule_at, cron_expression, approval_policy, resolver_attempt,
		       resolver_error, last_attempt_at, next_attempt_at, dead_letter_reason,
		       metadata, dispatched_at, resolved_at, completed_at, created_at, updated_at
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
		if err := s.hydrateTransaction(ctx, &tx); err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}

func (s *Store) UpsertWithdrawalApproval(ctx context.Context, approval gasbank.WithdrawalApproval) (gasbank.WithdrawalApproval, error) {
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
	`, approval.TransactionID, approval.Approver, approval.Status, toNullString(approval.Signature), toNullString(approval.Note), toNullTime(approval.DecidedAt), tenant, approval.CreatedAt, approval.UpdatedAt).Scan(&createdAt)
	if err != nil {
		return gasbank.WithdrawalApproval{}, err
	}
	approval.CreatedAt = createdAt.UTC()
	return approval, nil
}

func (s *Store) ListWithdrawalApprovals(ctx context.Context, transactionID string) ([]gasbank.WithdrawalApproval, error) {
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

	var result []gasbank.WithdrawalApproval
	for rows.Next() {
		approval, err := scanWithdrawalApproval(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, approval)
	}
	return result, rows.Err()
}

func (s *Store) SaveWithdrawalSchedule(ctx context.Context, schedule gasbank.WithdrawalSchedule) (gasbank.WithdrawalSchedule, error) {
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
	`, schedule.TransactionID, toNullTime(schedule.ScheduleAt), toNullString(schedule.CronExpression), toNullTime(schedule.NextRunAt), toNullTime(schedule.LastRunAt), tenant, schedule.CreatedAt, schedule.UpdatedAt).Scan(&createdAt)
	if err != nil {
		return gasbank.WithdrawalSchedule{}, err
	}
	schedule.CreatedAt = createdAt.UTC()
	return schedule, nil
}

func (s *Store) GetWithdrawalSchedule(ctx context.Context, transactionID string) (gasbank.WithdrawalSchedule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT transaction_id, schedule_at, cron_expression, next_run_at, last_run_at, created_at, updated_at
		FROM app_gas_withdrawal_schedules
		WHERE transaction_id = $1
	`, transactionID)
	return scanWithdrawalSchedule(row)
}

func (s *Store) DeleteWithdrawalSchedule(ctx context.Context, transactionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_gas_withdrawal_schedules WHERE transaction_id = $1`, transactionID)
	return err
}

func (s *Store) ListDueWithdrawalSchedules(ctx context.Context, before time.Time, limit int) ([]gasbank.WithdrawalSchedule, error) {
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

	var result []gasbank.WithdrawalSchedule
	for rows.Next() {
		schedule, err := scanWithdrawalSchedule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, schedule)
	}
	return result, rows.Err()
}

func (s *Store) RecordSettlementAttempt(ctx context.Context, attempt gasbank.SettlementAttempt) (gasbank.SettlementAttempt, error) {
	if attempt.TransactionID == "" {
		return gasbank.SettlementAttempt{}, fmt.Errorf("transaction id required")
	}
	if attempt.Attempt <= 0 {
		if err := s.db.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(attempt), 0) + 1 FROM app_gas_settlement_attempts WHERE transaction_id = $1
		`, attempt.TransactionID).Scan(&attempt.Attempt); err != nil {
			return gasbank.SettlementAttempt{}, err
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
	`, attempt.TransactionID, attempt.Attempt, attempt.StartedAt, attempt.CompletedAt, durationToMillis(latency), toNullString(attempt.Status), toNullString(attempt.Error), tenant)
	if err != nil {
		return gasbank.SettlementAttempt{}, err
	}
	attempt.Latency = latency
	return attempt, nil
}

func (s *Store) ListSettlementAttempts(ctx context.Context, transactionID string, limit int) ([]gasbank.SettlementAttempt, error) {
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

	var result []gasbank.SettlementAttempt
	for rows.Next() {
		attempt, err := scanSettlementAttempt(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, attempt)
	}
	return result, rows.Err()
}

func (s *Store) UpsertDeadLetter(ctx context.Context, entry gasbank.DeadLetter) (gasbank.DeadLetter, error) {
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
	`, entry.TransactionID, entry.AccountID, entry.Reason, toNullString(entry.LastError), toNullTime(entry.LastAttemptAt), entry.Retries, tenant, entry.CreatedAt, entry.UpdatedAt).Scan(&createdAt)
	if err != nil {
		return gasbank.DeadLetter{}, err
	}
	entry.CreatedAt = createdAt.UTC()
	return entry, nil
}

func (s *Store) GetDeadLetter(ctx context.Context, transactionID string) (gasbank.DeadLetter, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT transaction_id, account_id, reason, last_error, last_attempt_at, retries, created_at, updated_at
		FROM app_gas_dead_letters
		WHERE transaction_id = $1
	`, transactionID)
	return scanDeadLetter(row)
}

func (s *Store) ListDeadLetters(ctx context.Context, accountID string, limit int) ([]gasbank.DeadLetter, error) {
	if limit <= 0 {
		limit = 50
	}
	tenant := s.accountTenant(ctx, accountID)
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

	var result []gasbank.DeadLetter
	for rows.Next() {
		entry, err := scanDeadLetter(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (s *Store) RemoveDeadLetter(ctx context.Context, transactionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_gas_dead_letters WHERE transaction_id = $1`, transactionID)
	return err
}

// gasAccountTenant fetches the tenant recorded on a gas account.
func (s *Store) gasAccountTenant(ctx context.Context, gasAccountID string) string {
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
func (s *Store) gasTransactionTenant(ctx context.Context, txID string) string {
	var accountID sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT account_id FROM app_gas_transactions WHERE id = $1
	`, txID).Scan(&accountID)
	if accountID.Valid {
		return s.accountTenant(ctx, accountID.String)
	}
	return ""
}

func mapOrEmptyBool(m map[string]bool) map[string]bool {
	if m == nil {
		return map[string]bool{}
	}
	return m
}

func mapOrEmptyString(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func durationToMillis(d time.Duration) int64 {
	return d.Milliseconds()
}

func scanWithdrawalApproval(scanner rowScanner) (gasbank.WithdrawalApproval, error) {
	var (
		approval  gasbank.WithdrawalApproval
		signature sql.NullString
		note      sql.NullString
		decidedAt sql.NullTime
		createdAt time.Time
		updatedAt time.Time
	)
	if err := scanner.Scan(&approval.TransactionID, &approval.Approver, &approval.Status, &signature, &note, &decidedAt, &createdAt, &updatedAt); err != nil {
		return gasbank.WithdrawalApproval{}, err
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

func scanWithdrawalSchedule(scanner rowScanner) (gasbank.WithdrawalSchedule, error) {
	var (
		schedule   gasbank.WithdrawalSchedule
		scheduleAt sql.NullTime
		cronExpr   sql.NullString
		nextRun    sql.NullTime
		lastRun    sql.NullTime
		createdAt  time.Time
		updatedAt  time.Time
	)
	if err := scanner.Scan(&schedule.TransactionID, &scheduleAt, &cronExpr, &nextRun, &lastRun, &createdAt, &updatedAt); err != nil {
		return gasbank.WithdrawalSchedule{}, err
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

func scanSettlementAttempt(scanner rowScanner) (gasbank.SettlementAttempt, error) {
	var (
		attempt     gasbank.SettlementAttempt
		completedAt sql.NullTime
		latencyMS   sql.NullInt64
		status      sql.NullString
		errMsg      sql.NullString
	)
	if err := scanner.Scan(&attempt.TransactionID, &attempt.Attempt, &attempt.StartedAt, &completedAt, &latencyMS, &status, &errMsg); err != nil {
		return gasbank.SettlementAttempt{}, err
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

func scanDeadLetter(scanner rowScanner) (gasbank.DeadLetter, error) {
	var (
		entry       gasbank.DeadLetter
		lastError   sql.NullString
		lastAttempt sql.NullTime
		createdAt   time.Time
		updatedAt   time.Time
	)
	if err := scanner.Scan(&entry.TransactionID, &entry.AccountID, &entry.Reason, &lastError, &lastAttempt, &entry.Retries, &createdAt, &updatedAt); err != nil {
		return gasbank.DeadLetter{}, err
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

func (s *Store) hydrateTransaction(ctx context.Context, tx *gasbank.Transaction) error {
	if tx.Type != gasbank.TransactionWithdrawal {
		return nil
	}
	approvals, err := s.ListWithdrawalApprovals(ctx, tx.ID)
	if err != nil {
		return err
	}
	tx.Approvals = approvals
	return nil
}
