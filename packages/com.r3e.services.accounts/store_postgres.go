// Package accounts provides the Accounts service as a ServicePackage.
package accounts

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PostgresStore implements Store using PostgreSQL.
// This is the service-local store implementation that uses the generic
// database connection provided by the Service Engine.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL-backed account store.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// CreateAccount creates a new account.
func (s *PostgresStore) CreateAccount(ctx context.Context, acct Account) (Account, error) {
	if acct.ID == "" {
		acct.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now
	tenant := tenantFromMetadata(acct.Metadata)

	metadataJSON, err := json.Marshal(acct.Metadata)
	if err != nil {
		return Account{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_accounts (id, owner, metadata, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, acct.ID, acct.Owner, metadataJSON, tenant, acct.CreatedAt, acct.UpdatedAt)
	if err != nil {
		return Account{}, err
	}
	return acct, nil
}

// UpdateAccount updates an existing account.
func (s *PostgresStore) UpdateAccount(ctx context.Context, acct Account) (Account, error) {
	existing, err := s.GetAccount(ctx, acct.ID)
	if err != nil {
		return Account{}, err
	}

	acct.CreatedAt = existing.CreatedAt
	acct.UpdatedAt = time.Now().UTC()
	tenant := tenantFromMetadata(acct.Metadata)

	metadataJSON, err := json.Marshal(acct.Metadata)
	if err != nil {
		return Account{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_accounts
		SET owner = $2, metadata = $3, tenant = $4, updated_at = $5
		WHERE id = $1
	`, acct.ID, acct.Owner, metadataJSON, tenant, acct.UpdatedAt)
	if err != nil {
		return Account{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Account{}, sql.ErrNoRows
	}
	return acct, nil
}

// GetAccount retrieves an account by ID.
func (s *PostgresStore) GetAccount(ctx context.Context, id string) (Account, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, owner, metadata, tenant, created_at, updated_at
		FROM app_accounts
		WHERE id = $1
	`, id)

	var (
		acct        Account
		metadataRaw []byte
		tenant      sql.NullString
	)

	if err := row.Scan(&acct.ID, &acct.Owner, &metadataRaw, &tenant, &acct.CreatedAt, &acct.UpdatedAt); err != nil {
		return Account{}, err
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

// ListAccounts lists all accounts.
func (s *PostgresStore) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner, metadata, tenant, created_at, updated_at
		FROM app_accounts
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Account
	for rows.Next() {
		var (
			acct        Account
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

// DeleteAccount deletes an account by ID.
func (s *PostgresStore) DeleteAccount(ctx context.Context, id string) error {
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

func tenantFromMetadata(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	return meta["tenant"]
}

// CreateWorkspaceWallet creates a new workspace wallet.
func (s *PostgresStore) CreateWorkspaceWallet(ctx context.Context, wallet WorkspaceWallet) (WorkspaceWallet, error) {
	if wallet.ID == "" {
		wallet.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	wallet.CreatedAt = now
	wallet.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO workspace_wallets (id, workspace_id, wallet_address, label, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, wallet.ID, wallet.WorkspaceID, wallet.WalletAddress, wallet.Label, wallet.Status, wallet.CreatedAt, wallet.UpdatedAt)
	if err != nil {
		return WorkspaceWallet{}, err
	}
	return wallet, nil
}

// GetWorkspaceWallet retrieves a workspace wallet by ID.
func (s *PostgresStore) GetWorkspaceWallet(ctx context.Context, id string) (WorkspaceWallet, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, wallet_address, label, status, created_at, updated_at
		FROM workspace_wallets
		WHERE id = $1
	`, id)

	var wallet WorkspaceWallet
	if err := row.Scan(&wallet.ID, &wallet.WorkspaceID, &wallet.WalletAddress, &wallet.Label, &wallet.Status, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
		return WorkspaceWallet{}, err
	}
	return wallet, nil
}

// ListWorkspaceWallets lists all wallets for a workspace.
func (s *PostgresStore) ListWorkspaceWallets(ctx context.Context, workspaceID string) ([]WorkspaceWallet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, wallet_address, label, status, created_at, updated_at
		FROM workspace_wallets
		WHERE workspace_id = $1
		ORDER BY created_at
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WorkspaceWallet
	for rows.Next() {
		var wallet WorkspaceWallet
		if err := rows.Scan(&wallet.ID, &wallet.WorkspaceID, &wallet.WalletAddress, &wallet.Label, &wallet.Status, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, wallet)
	}
	return result, rows.Err()
}

// FindWorkspaceWalletByAddress finds a wallet by address within a workspace.
func (s *PostgresStore) FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, walletAddr string) (WorkspaceWallet, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, wallet_address, label, status, created_at, updated_at
		FROM workspace_wallets
		WHERE workspace_id = $1 AND LOWER(wallet_address) = LOWER($2)
	`, workspaceID, walletAddr)

	var wallet WorkspaceWallet
	if err := row.Scan(&wallet.ID, &wallet.WorkspaceID, &wallet.WalletAddress, &wallet.Label, &wallet.Status, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
		return WorkspaceWallet{}, err
	}
	return wallet, nil
}
