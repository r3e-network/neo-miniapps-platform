package mixer

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL store.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// CreateMixRequest inserts a new mix request.
func (s *PostgresStore) CreateMixRequest(ctx context.Context, req MixRequest) (MixRequest, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now

	targetsJSON, _ := json.Marshal(req.Targets)
	metadataJSON, _ := json.Marshal(req.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mixer_requests
		(id, account_id, status, source_wallet, amount, token_address, mix_duration, split_count,
		 targets, deposit_tx_hashes, deposit_pool_ids, deposited_amount, zk_proof_hash, tee_signature,
		 on_chain_proof_tx, mix_start_at, mix_end_at, withdrawable_at, completed_at, completion_proof_tx,
		 delivered_amount, error, refund_tx_hash, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
	`, req.ID, req.AccountID, req.Status, req.SourceWallet, req.Amount, req.TokenAddress, req.MixDuration, req.SplitCount,
		targetsJSON, pq.Array(req.DepositTxHashes), pq.Array(req.DepositPoolIDs), req.DepositedAmount,
		req.ZKProofHash, req.TEESignature, req.OnChainProofTx, req.MixStartAt, req.MixEndAt, req.WithdrawableAt,
		req.CompletedAt, req.CompletionProofTx, req.DeliveredAmount, req.Error, req.RefundTxHash,
		metadataJSON, req.CreatedAt, req.UpdatedAt)
	if err != nil {
		return MixRequest{}, err
	}
	return req, nil
}

// UpdateMixRequest updates an existing mix request.
func (s *PostgresStore) UpdateMixRequest(ctx context.Context, req MixRequest) (MixRequest, error) {
	req.UpdatedAt = time.Now().UTC()
	targetsJSON, _ := json.Marshal(req.Targets)
	metadataJSON, _ := json.Marshal(req.Metadata)

	_, err := s.db.ExecContext(ctx, `
		UPDATE mixer_requests SET
			status = $2, deposit_tx_hashes = $3, deposit_pool_ids = $4, deposited_amount = $5,
			zk_proof_hash = $6, tee_signature = $7, on_chain_proof_tx = $8, completed_at = $9,
			completion_proof_tx = $10, delivered_amount = $11, error = $12, refund_tx_hash = $13,
			targets = $14, metadata = $15, updated_at = $16
		WHERE id = $1
	`, req.ID, req.Status, pq.Array(req.DepositTxHashes), pq.Array(req.DepositPoolIDs), req.DepositedAmount,
		req.ZKProofHash, req.TEESignature, req.OnChainProofTx, req.CompletedAt, req.CompletionProofTx,
		req.DeliveredAmount, req.Error, req.RefundTxHash, targetsJSON, metadataJSON, req.UpdatedAt)
	if err != nil {
		return MixRequest{}, err
	}
	return req, nil
}

// GetMixRequest retrieves a mix request by ID.
func (s *PostgresStore) GetMixRequest(ctx context.Context, id string) (MixRequest, error) {
	var req MixRequest
	var targetsJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, status, source_wallet, amount, token_address, mix_duration, split_count,
		       targets, deposit_tx_hashes, deposit_pool_ids, deposited_amount, zk_proof_hash, tee_signature,
		       on_chain_proof_tx, mix_start_at, mix_end_at, withdrawable_at, completed_at, completion_proof_tx,
		       delivered_amount, error, refund_tx_hash, metadata, created_at, updated_at
		FROM mixer_requests WHERE id = $1
	`, id).Scan(&req.ID, &req.AccountID, &req.Status, &req.SourceWallet, &req.Amount, &req.TokenAddress,
		&req.MixDuration, &req.SplitCount, &targetsJSON, pq.Array(&req.DepositTxHashes),
		pq.Array(&req.DepositPoolIDs), &req.DepositedAmount, &req.ZKProofHash, &req.TEESignature,
		&req.OnChainProofTx, &req.MixStartAt, &req.MixEndAt, &req.WithdrawableAt, &req.CompletedAt,
		&req.CompletionProofTx, &req.DeliveredAmount, &req.Error, &req.RefundTxHash,
		&metadataJSON, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return MixRequest{}, err
	}
	_ = json.Unmarshal(targetsJSON, &req.Targets)
	_ = json.Unmarshal(metadataJSON, &req.Metadata)
	return req, nil
}

// GetMixRequestByProofHash retrieves a mix request by ZK proof hash.
func (s *PostgresStore) GetMixRequestByProofHash(ctx context.Context, proofHash string) (MixRequest, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM mixer_requests WHERE zk_proof_hash = $1`, proofHash).Scan(&id)
	if err != nil {
		return MixRequest{}, err
	}
	return s.GetMixRequest(ctx, id)
}

// ListMixRequests lists mix requests for an account.
func (s *PostgresStore) ListMixRequests(ctx context.Context, accountID string, limit int) ([]MixRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_requests WHERE account_id = $1 ORDER BY created_at DESC LIMIT $2
	`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanMixRequestIDs(ctx, rows)
}

// ListMixRequestsByStatus lists mix requests by status.
func (s *PostgresStore) ListMixRequestsByStatus(ctx context.Context, status RequestStatus, limit int) ([]MixRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_requests WHERE status = $1 ORDER BY created_at DESC LIMIT $2
	`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanMixRequestIDs(ctx, rows)
}

// ListPendingMixRequests lists pending mix requests.
func (s *PostgresStore) ListPendingMixRequests(ctx context.Context) ([]MixRequest, error) {
	return s.ListMixRequestsByStatus(ctx, RequestStatusPending, 100)
}

// ListExpiredMixRequests lists requests past their withdrawable time.
func (s *PostgresStore) ListExpiredMixRequests(ctx context.Context, before time.Time) ([]MixRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_requests WHERE withdrawable_at < $1 AND status NOT IN ('completed', 'refunded')
		ORDER BY withdrawable_at ASC LIMIT 100
	`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanMixRequestIDs(ctx, rows)
}

func (s *PostgresStore) scanMixRequestIDs(ctx context.Context, rows *sql.Rows) ([]MixRequest, error) {
	var result []MixRequest
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		req, err := s.GetMixRequest(ctx, id)
		if err != nil {
			continue
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

// CreatePoolAccount inserts a new pool account.
func (s *PostgresStore) CreatePoolAccount(ctx context.Context, pool PoolAccount) (PoolAccount, error) {
	if pool.ID == "" {
		pool.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	pool.CreatedAt = now
	pool.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mixer_pool_accounts
		(id, wallet_address, status, balance, pending_in, pending_out, tee_key_id, encrypted_priv_key,
		 total_received, total_sent, transaction_count, retire_after, last_activity_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, pool.ID, pool.WalletAddress, pool.Status, pool.Balance, pool.PendingIn, pool.PendingOut,
		pool.TEEKeyID, pool.EncryptedPrivKey, pool.TotalReceived, pool.TotalSent, pool.TransactionCount,
		pool.RetireAfter, pool.LastActivityAt, pool.CreatedAt, pool.UpdatedAt)
	if err != nil {
		return PoolAccount{}, err
	}
	return pool, nil
}

// UpdatePoolAccount updates an existing pool account.
func (s *PostgresStore) UpdatePoolAccount(ctx context.Context, pool PoolAccount) (PoolAccount, error) {
	pool.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE mixer_pool_accounts SET
			status = $2, balance = $3, pending_in = $4, pending_out = $5, total_received = $6,
			total_sent = $7, transaction_count = $8, last_activity_at = $9, updated_at = $10
		WHERE id = $1
	`, pool.ID, pool.Status, pool.Balance, pool.PendingIn, pool.PendingOut, pool.TotalReceived,
		pool.TotalSent, pool.TransactionCount, pool.LastActivityAt, pool.UpdatedAt)
	if err != nil {
		return PoolAccount{}, err
	}
	return pool, nil
}

// GetPoolAccount retrieves a pool account by ID.
func (s *PostgresStore) GetPoolAccount(ctx context.Context, id string) (PoolAccount, error) {
	var pool PoolAccount
	err := s.db.QueryRowContext(ctx, `
		SELECT id, wallet_address, status, balance, pending_in, pending_out, tee_key_id, encrypted_priv_key,
		       total_received, total_sent, transaction_count, retire_after, last_activity_at, created_at, updated_at
		FROM mixer_pool_accounts WHERE id = $1
	`, id).Scan(&pool.ID, &pool.WalletAddress, &pool.Status, &pool.Balance, &pool.PendingIn, &pool.PendingOut,
		&pool.TEEKeyID, &pool.EncryptedPrivKey, &pool.TotalReceived, &pool.TotalSent, &pool.TransactionCount,
		&pool.RetireAfter, &pool.LastActivityAt, &pool.CreatedAt, &pool.UpdatedAt)
	return pool, err
}

// GetPoolAccountByWallet retrieves a pool account by wallet address.
func (s *PostgresStore) GetPoolAccountByWallet(ctx context.Context, wallet string) (PoolAccount, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM mixer_pool_accounts WHERE wallet_address = $1`, wallet).Scan(&id)
	if err != nil {
		return PoolAccount{}, err
	}
	return s.GetPoolAccount(ctx, id)
}

// ListPoolAccounts lists pool accounts by status.
func (s *PostgresStore) ListPoolAccounts(ctx context.Context, status PoolAccountStatus) ([]PoolAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_pool_accounts WHERE status = $1 ORDER BY created_at DESC
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanPoolAccountIDs(ctx, rows)
}

// ListActivePoolAccounts lists active pool accounts.
func (s *PostgresStore) ListActivePoolAccounts(ctx context.Context) ([]PoolAccount, error) {
	return s.ListPoolAccounts(ctx, PoolAccountStatusActive)
}

// ListRetirablePoolAccounts lists pool accounts ready for retirement.
func (s *PostgresStore) ListRetirablePoolAccounts(ctx context.Context, before time.Time) ([]PoolAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_pool_accounts WHERE retire_after < $1 AND status = 'active'
	`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanPoolAccountIDs(ctx, rows)
}

func (s *PostgresStore) scanPoolAccountIDs(ctx context.Context, rows *sql.Rows) ([]PoolAccount, error) {
	var result []PoolAccount
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		pool, err := s.GetPoolAccount(ctx, id)
		if err != nil {
			continue
		}
		result = append(result, pool)
	}
	return result, rows.Err()
}

// CreateMixTransaction inserts a new mix transaction.
func (s *PostgresStore) CreateMixTransaction(ctx context.Context, tx MixTransaction) (MixTransaction, error) {
	if tx.ID == "" {
		tx.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	tx.CreatedAt = now
	tx.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mixer_transactions
		(id, type, status, from_pool_id, to_pool_id, amount, request_id, target_address, tx_hash,
		 block_number, gas_used, error, scheduled_at, executed_at, confirmed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, tx.ID, tx.Type, tx.Status, tx.FromPoolID, tx.ToPoolID, tx.Amount, tx.RequestID, tx.TargetAddress,
		tx.TxHash, tx.BlockNumber, tx.GasUsed, tx.Error, tx.ScheduledAt, tx.ExecutedAt, tx.ConfirmedAt,
		tx.CreatedAt, tx.UpdatedAt)
	if err != nil {
		return MixTransaction{}, err
	}
	return tx, nil
}

// UpdateMixTransaction updates an existing mix transaction.
func (s *PostgresStore) UpdateMixTransaction(ctx context.Context, tx MixTransaction) (MixTransaction, error) {
	tx.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE mixer_transactions SET
			status = $2, tx_hash = $3, block_number = $4, gas_used = $5, error = $6,
			executed_at = $7, confirmed_at = $8, updated_at = $9
		WHERE id = $1
	`, tx.ID, tx.Status, tx.TxHash, tx.BlockNumber, tx.GasUsed, tx.Error, tx.ExecutedAt, tx.ConfirmedAt, tx.UpdatedAt)
	if err != nil {
		return MixTransaction{}, err
	}
	return tx, nil
}

// GetMixTransaction retrieves a mix transaction by ID.
func (s *PostgresStore) GetMixTransaction(ctx context.Context, id string) (MixTransaction, error) {
	var tx MixTransaction
	err := s.db.QueryRowContext(ctx, `
		SELECT id, type, status, from_pool_id, to_pool_id, amount, request_id, target_address, tx_hash,
		       block_number, gas_used, error, scheduled_at, executed_at, confirmed_at, created_at, updated_at
		FROM mixer_transactions WHERE id = $1
	`, id).Scan(&tx.ID, &tx.Type, &tx.Status, &tx.FromPoolID, &tx.ToPoolID, &tx.Amount, &tx.RequestID,
		&tx.TargetAddress, &tx.TxHash, &tx.BlockNumber, &tx.GasUsed, &tx.Error, &tx.ScheduledAt,
		&tx.ExecutedAt, &tx.ConfirmedAt, &tx.CreatedAt, &tx.UpdatedAt)
	return tx, err
}

// GetMixTransactionByHash retrieves a mix transaction by tx hash.
func (s *PostgresStore) GetMixTransactionByHash(ctx context.Context, txHash string) (MixTransaction, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM mixer_transactions WHERE tx_hash = $1`, txHash).Scan(&id)
	if err != nil {
		return MixTransaction{}, err
	}
	return s.GetMixTransaction(ctx, id)
}

// ListMixTransactions lists mix transactions for a request.
func (s *PostgresStore) ListMixTransactions(ctx context.Context, requestID string, limit int) ([]MixTransaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_transactions WHERE request_id = $1 ORDER BY scheduled_at ASC LIMIT $2
	`, requestID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanMixTransactionIDs(ctx, rows)
}

// ListMixTransactionsByPool lists mix transactions for a pool account.
func (s *PostgresStore) ListMixTransactionsByPool(ctx context.Context, poolID string, limit int) ([]MixTransaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_transactions WHERE from_pool_id = $1 OR to_pool_id = $1 ORDER BY created_at DESC LIMIT $2
	`, poolID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanMixTransactionIDs(ctx, rows)
}

// ListScheduledMixTransactions lists scheduled transactions ready for execution.
func (s *PostgresStore) ListScheduledMixTransactions(ctx context.Context, before time.Time, limit int) ([]MixTransaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_transactions WHERE status = 'scheduled' AND scheduled_at < $1 ORDER BY scheduled_at ASC LIMIT $2
	`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanMixTransactionIDs(ctx, rows)
}

// ListPendingMixTransactions lists pending transactions.
func (s *PostgresStore) ListPendingMixTransactions(ctx context.Context) ([]MixTransaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_transactions WHERE status IN ('pending', 'submitted') ORDER BY created_at ASC LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanMixTransactionIDs(ctx, rows)
}

func (s *PostgresStore) scanMixTransactionIDs(ctx context.Context, rows *sql.Rows) ([]MixTransaction, error) {
	var result []MixTransaction
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		tx, err := s.GetMixTransaction(ctx, id)
		if err != nil {
			continue
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}

// CreateWithdrawalClaim inserts a new withdrawal claim.
func (s *PostgresStore) CreateWithdrawalClaim(ctx context.Context, claim WithdrawalClaim) (WithdrawalClaim, error) {
	if claim.ID == "" {
		claim.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	claim.CreatedAt = now
	claim.UpdatedAt = now

	metadataJSON, _ := json.Marshal(claim.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mixer_withdrawal_claims
		(id, request_id, account_id, claim_amount, claim_address, status, claim_tx_hash, claim_block_number,
		 claimable_at, resolution_tx_hash, resolved_at, error, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, claim.ID, claim.RequestID, claim.AccountID, claim.ClaimAmount, claim.ClaimAddress, claim.Status,
		claim.ClaimTxHash, claim.ClaimBlockNumber, claim.ClaimableAt, claim.ResolutionTxHash, claim.ResolvedAt,
		claim.Error, metadataJSON, claim.CreatedAt, claim.UpdatedAt)
	if err != nil {
		return WithdrawalClaim{}, err
	}
	return claim, nil
}

// UpdateWithdrawalClaim updates an existing withdrawal claim.
func (s *PostgresStore) UpdateWithdrawalClaim(ctx context.Context, claim WithdrawalClaim) (WithdrawalClaim, error) {
	claim.UpdatedAt = time.Now().UTC()
	metadataJSON, _ := json.Marshal(claim.Metadata)

	_, err := s.db.ExecContext(ctx, `
		UPDATE mixer_withdrawal_claims SET
			status = $2, claim_tx_hash = $3, claim_block_number = $4, resolution_tx_hash = $5,
			resolved_at = $6, error = $7, metadata = $8, updated_at = $9
		WHERE id = $1
	`, claim.ID, claim.Status, claim.ClaimTxHash, claim.ClaimBlockNumber, claim.ResolutionTxHash,
		claim.ResolvedAt, claim.Error, metadataJSON, claim.UpdatedAt)
	if err != nil {
		return WithdrawalClaim{}, err
	}
	return claim, nil
}

// GetWithdrawalClaim retrieves a withdrawal claim by ID.
func (s *PostgresStore) GetWithdrawalClaim(ctx context.Context, id string) (WithdrawalClaim, error) {
	var claim WithdrawalClaim
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, request_id, account_id, claim_amount, claim_address, status, claim_tx_hash, claim_block_number,
		       claimable_at, resolution_tx_hash, resolved_at, error, metadata, created_at, updated_at
		FROM mixer_withdrawal_claims WHERE id = $1
	`, id).Scan(&claim.ID, &claim.RequestID, &claim.AccountID, &claim.ClaimAmount, &claim.ClaimAddress,
		&claim.Status, &claim.ClaimTxHash, &claim.ClaimBlockNumber, &claim.ClaimableAt, &claim.ResolutionTxHash,
		&claim.ResolvedAt, &claim.Error, &metadataJSON, &claim.CreatedAt, &claim.UpdatedAt)
	if err != nil {
		return WithdrawalClaim{}, err
	}
	_ = json.Unmarshal(metadataJSON, &claim.Metadata)
	return claim, nil
}

// GetWithdrawalClaimByRequest retrieves a withdrawal claim by request ID.
func (s *PostgresStore) GetWithdrawalClaimByRequest(ctx context.Context, requestID string) (WithdrawalClaim, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM mixer_withdrawal_claims WHERE request_id = $1`, requestID).Scan(&id)
	if err != nil {
		return WithdrawalClaim{}, err
	}
	return s.GetWithdrawalClaim(ctx, id)
}

// ListWithdrawalClaims lists withdrawal claims for an account.
func (s *PostgresStore) ListWithdrawalClaims(ctx context.Context, accountID string, limit int) ([]WithdrawalClaim, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_withdrawal_claims WHERE account_id = $1 ORDER BY created_at DESC LIMIT $2
	`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WithdrawalClaim
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		claim, err := s.GetWithdrawalClaim(ctx, id)
		if err != nil {
			continue
		}
		result = append(result, claim)
	}
	return result, rows.Err()
}

// ListClaimableWithdrawals lists claims that can be executed.
func (s *PostgresStore) ListClaimableWithdrawals(ctx context.Context, before time.Time) ([]WithdrawalClaim, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM mixer_withdrawal_claims WHERE status = 'pending' AND claimable_at < $1
	`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WithdrawalClaim
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		claim, err := s.GetWithdrawalClaim(ctx, id)
		if err != nil {
			continue
		}
		result = append(result, claim)
	}
	return result, rows.Err()
}

// GetServiceDeposit retrieves the service deposit record.
func (s *PostgresStore) GetServiceDeposit(ctx context.Context) (ServiceDeposit, error) {
	var deposit ServiceDeposit
	err := s.db.QueryRowContext(ctx, `
		SELECT id, amount, locked_amount, available_amount, wallet_address, last_top_up_at, updated_at
		FROM mixer_service_deposit LIMIT 1
	`).Scan(&deposit.ID, &deposit.Amount, &deposit.LockedAmount, &deposit.AvailableAmount,
		&deposit.WalletAddress, &deposit.LastTopUpAt, &deposit.UpdatedAt)
	if err == sql.ErrNoRows {
		return ServiceDeposit{Amount: "0", LockedAmount: "0", AvailableAmount: "0"}, nil
	}
	return deposit, err
}

// UpdateServiceDeposit updates the service deposit record.
func (s *PostgresStore) UpdateServiceDeposit(ctx context.Context, deposit ServiceDeposit) (ServiceDeposit, error) {
	deposit.UpdatedAt = time.Now().UTC()
	if deposit.ID == "" {
		deposit.ID = uuid.NewString()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO mixer_service_deposit (id, amount, locked_amount, available_amount, wallet_address, last_top_up_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, deposit.ID, deposit.Amount, deposit.LockedAmount, deposit.AvailableAmount, deposit.WalletAddress, deposit.LastTopUpAt, deposit.UpdatedAt)
		if err != nil {
			return ServiceDeposit{}, err
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
			UPDATE mixer_service_deposit SET amount = $2, locked_amount = $3, available_amount = $4, last_top_up_at = $5, updated_at = $6 WHERE id = $1
		`, deposit.ID, deposit.Amount, deposit.LockedAmount, deposit.AvailableAmount, deposit.LastTopUpAt, deposit.UpdatedAt)
		if err != nil {
			return ServiceDeposit{}, err
		}
	}
	return deposit, nil
}

// GetMixStats returns service statistics.
func (s *PostgresStore) GetMixStats(ctx context.Context) (MixStats, error) {
	var stats MixStats
	stats.GeneratedAt = time.Now().UTC()

	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mixer_requests`).Scan(&stats.TotalRequests)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mixer_requests WHERE status IN ('pending', 'deposited', 'mixing')`).Scan(&stats.ActiveRequests)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mixer_requests WHERE status = 'completed'`).Scan(&stats.CompletedRequests)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mixer_pool_accounts WHERE status = 'active'`).Scan(&stats.ActivePoolAccounts)

	deposit, _ := s.GetServiceDeposit(ctx)
	stats.ServiceDeposit = deposit.Amount
	stats.AvailableCapacity = deposit.AvailableAmount

	return stats, nil
}
