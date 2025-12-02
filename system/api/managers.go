// Package api provides implementations of manager interfaces for the user service.
//
// DEPRECATED: This file contains legacy service-specific manager implementations.
// New services should implement their own stores in packages/com.r3e.services.*/
// using the store_postgres.go pattern. This file will be removed once all
// consumers migrate to service-specific implementations.
package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// PostgresAccountManager implements AccountManager using PostgreSQL.
type PostgresAccountManager struct {
	db  *sql.DB
	log *logger.Logger
}

// NewPostgresAccountManager creates a new PostgreSQL account manager.
func NewPostgresAccountManager(db *sql.DB, log *logger.Logger) *PostgresAccountManager {
	if log == nil {
		log = logger.NewDefault("account-manager")
	}
	return &PostgresAccountManager{db: db, log: log}
}

// EnsureSchema creates the required tables.
func (m *PostgresAccountManager) EnsureSchema(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			owner TEXT NOT NULL,
			metadata JSONB,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS account_wallets (
			account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			address TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (account_id, address)
		);

		CREATE INDEX IF NOT EXISTS idx_accounts_owner ON accounts(owner);
		CREATE INDEX IF NOT EXISTS idx_account_wallets_address ON account_wallets(address);
	`)
	return err
}

func (m *PostgresAccountManager) CreateAccount(ctx context.Context, ownerAddress string, metadata map[string]string) (string, error) {
	id := generateAccountID(ownerAddress)

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `
		INSERT INTO accounts (id, owner, metadata, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', now(), now())
	`, id, ownerAddress, metadataJSON)
	if err != nil {
		return "", fmt.Errorf("create account: %w", err)
	}

	m.log.WithField("account_id", id).WithField("owner", ownerAddress).Info("account created")
	return id, nil
}

func (m *PostgresAccountManager) GetAccount(ctx context.Context, accountID string) (*Account, error) {
	var account Account
	var metadataJSON []byte

	err := m.db.QueryRowContext(ctx, `
		SELECT id, owner, metadata, status, created_at, updated_at
		FROM accounts WHERE id = $1
	`, accountID).Scan(&account.ID, &account.Owner, &metadataJSON, &account.Status, &account.CreatedAt, &account.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found")
	}
	if err != nil {
		return nil, err
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &account.Metadata)
	}

	return &account, nil
}

func (m *PostgresAccountManager) UpdateAccount(ctx context.Context, accountID string, metadata map[string]string) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	result, err := m.db.ExecContext(ctx, `
		UPDATE accounts SET metadata = $2, updated_at = now() WHERE id = $1
	`, accountID, metadataJSON)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (m *PostgresAccountManager) LinkWallet(ctx context.Context, accountID, walletAddress string) error {
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO account_wallets (account_id, address, status, linked_at)
		VALUES ($1, $2, 'active', now())
		ON CONFLICT (account_id, address) DO UPDATE SET status = 'active'
	`, accountID, walletAddress)
	if err != nil {
		return fmt.Errorf("link wallet: %w", err)
	}

	m.log.WithField("account_id", accountID).WithField("wallet", walletAddress).Info("wallet linked")
	return nil
}

func (m *PostgresAccountManager) UnlinkWallet(ctx context.Context, accountID, walletAddress string) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE account_wallets SET status = 'revoked' WHERE account_id = $1 AND address = $2
	`, accountID, walletAddress)
	return err
}

func (m *PostgresAccountManager) ListWallets(ctx context.Context, accountID string) ([]Wallet, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT address, account_id, status, linked_at
		FROM account_wallets WHERE account_id = $1 AND status = 'active'
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []Wallet
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(&w.Address, &w.AccountID, &w.Status, &w.LinkedAt); err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}

	return wallets, rows.Err()
}

// PostgresSecretsManager implements SecretsManager using PostgreSQL.
type PostgresSecretsManager struct {
	db         *sql.DB
	log        *logger.Logger
	encryptKey []byte // For encrypting secrets at rest
}

// NewPostgresSecretsManager creates a new PostgreSQL secrets manager.
func NewPostgresSecretsManager(db *sql.DB, encryptKey []byte, log *logger.Logger) *PostgresSecretsManager {
	if log == nil {
		log = logger.NewDefault("secrets-manager")
	}
	return &PostgresSecretsManager{db: db, log: log, encryptKey: encryptKey}
}

// EnsureSchema creates the required tables.
func (m *PostgresSecretsManager) EnsureSchema(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS account_secrets (
			account_id TEXT NOT NULL,
			name TEXT NOT NULL,
			value BYTEA NOT NULL,
			encrypted BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (account_id, name)
		);

		CREATE INDEX IF NOT EXISTS idx_account_secrets_account ON account_secrets(account_id);
	`)
	return err
}

func (m *PostgresSecretsManager) SetSecret(ctx context.Context, accountID, name string, value []byte, encrypted bool) error {
	// In production, encrypt the value if encrypted=true
	storedValue := value
	if encrypted && len(m.encryptKey) > 0 {
		// Simple XOR encryption for demo; use proper encryption in production
		storedValue = make([]byte, len(value))
		for i := range value {
			storedValue[i] = value[i] ^ m.encryptKey[i%len(m.encryptKey)]
		}
	}

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO account_secrets (account_id, name, value, encrypted, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
		ON CONFLICT (account_id, name) DO UPDATE SET value = $3, encrypted = $4, updated_at = now()
	`, accountID, name, storedValue, encrypted)
	if err != nil {
		return fmt.Errorf("set secret: %w", err)
	}

	m.log.WithField("account_id", accountID).WithField("name", name).Info("secret set")
	return nil
}

func (m *PostgresSecretsManager) GetSecret(ctx context.Context, accountID, name string) ([]byte, error) {
	var value []byte
	var encrypted bool

	err := m.db.QueryRowContext(ctx, `
		SELECT value, encrypted FROM account_secrets WHERE account_id = $1 AND name = $2
	`, accountID, name).Scan(&value, &encrypted)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("secret not found")
	}
	if err != nil {
		return nil, err
	}

	// Decrypt if needed
	if encrypted && len(m.encryptKey) > 0 {
		decrypted := make([]byte, len(value))
		for i := range value {
			decrypted[i] = value[i] ^ m.encryptKey[i%len(m.encryptKey)]
		}
		return decrypted, nil
	}

	return value, nil
}

func (m *PostgresSecretsManager) DeleteSecret(ctx context.Context, accountID, name string) error {
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM account_secrets WHERE account_id = $1 AND name = $2
	`, accountID, name)
	return err
}

func (m *PostgresSecretsManager) ListSecrets(ctx context.Context, accountID string) ([]SecretInfo, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT name, encrypted, created_at, updated_at
		FROM account_secrets WHERE account_id = $1
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []SecretInfo
	for rows.Next() {
		var s SecretInfo
		if err := rows.Scan(&s.Name, &s.Encrypted, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}

	return secrets, rows.Err()
}

func (m *PostgresSecretsManager) RotateSecret(ctx context.Context, accountID, name string, newValue []byte) error {
	// Get current encryption status
	var encrypted bool
	err := m.db.QueryRowContext(ctx, `
		SELECT encrypted FROM account_secrets WHERE account_id = $1 AND name = $2
	`, accountID, name).Scan(&encrypted)
	if err != nil {
		return fmt.Errorf("secret not found")
	}

	return m.SetSecret(ctx, accountID, name, newValue, encrypted)
}

// PostgresContractManager implements ContractManager using PostgreSQL.
type PostgresContractManager struct {
	db  *sql.DB
	log *logger.Logger
}

// NewPostgresContractManager creates a new PostgreSQL contract manager.
func NewPostgresContractManager(db *sql.DB, log *logger.Logger) *PostgresContractManager {
	if log == nil {
		log = logger.NewDefault("contract-manager")
	}
	return &PostgresContractManager{db: db, log: log}
}

// EnsureSchema creates the required tables.
func (m *PostgresContractManager) EnsureSchema(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_contracts (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			script_hash TEXT NOT NULL,
			capabilities TEXT[],
			status TEXT NOT NULL DEFAULT 'active',
			paused BOOLEAN NOT NULL DEFAULT false,
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_user_contracts_account ON user_contracts(account_id);
		CREATE INDEX IF NOT EXISTS idx_user_contracts_script_hash ON user_contracts(script_hash);
	`)
	return err
}

func (m *PostgresContractManager) RegisterContract(ctx context.Context, accountID string, spec *ContractSpec) (string, error) {
	id := generateContractID(accountID, spec.ScriptHash)

	metadataJSON, _ := json.Marshal(spec.Metadata)

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO user_contracts (id, account_id, name, description, script_hash, capabilities, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
	`, id, accountID, spec.Name, spec.Description, spec.ScriptHash, spec.Capabilities, metadataJSON)
	if err != nil {
		return "", fmt.Errorf("register contract: %w", err)
	}

	m.log.WithField("contract_id", id).WithField("account_id", accountID).Info("contract registered")
	return id, nil
}

func (m *PostgresContractManager) UpdateContract(ctx context.Context, contractID string, spec *ContractSpec) error {
	metadataJSON, _ := json.Marshal(spec.Metadata)

	_, err := m.db.ExecContext(ctx, `
		UPDATE user_contracts SET
			name = $2, description = $3, capabilities = $4, metadata = $5, updated_at = now()
		WHERE id = $1
	`, contractID, spec.Name, spec.Description, spec.Capabilities, metadataJSON)
	return err
}

func (m *PostgresContractManager) GetContract(ctx context.Context, contractID string) (*Contract, error) {
	var c Contract
	var metadataJSON []byte

	err := m.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, description, script_hash, capabilities, status, paused, metadata, created_at, updated_at
		FROM user_contracts WHERE id = $1
	`, contractID).Scan(&c.ID, &c.AccountID, &c.Name, &c.Description, &c.ScriptHash, &c.Capabilities, &c.Status, &c.Paused, &metadataJSON, &c.CreatedAt, &c.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("contract not found")
	}
	if err != nil {
		return nil, err
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &c.Metadata)
	}

	return &c, nil
}

func (m *PostgresContractManager) ListContracts(ctx context.Context, accountID string) ([]*Contract, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, account_id, name, description, script_hash, capabilities, status, paused, metadata, created_at, updated_at
		FROM user_contracts WHERE account_id = $1 AND status = 'active'
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contracts []*Contract
	for rows.Next() {
		var c Contract
		var metadataJSON []byte
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Name, &c.Description, &c.ScriptHash, &c.Capabilities, &c.Status, &c.Paused, &metadataJSON, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &c.Metadata)
		}
		contracts = append(contracts, &c)
	}

	return contracts, rows.Err()
}

func (m *PostgresContractManager) PauseContract(ctx context.Context, contractID string, paused bool) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE user_contracts SET paused = $2, updated_at = now() WHERE id = $1
	`, contractID, paused)
	return err
}

func (m *PostgresContractManager) DeleteContract(ctx context.Context, contractID string) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE user_contracts SET status = 'deleted', updated_at = now() WHERE id = $1
	`, contractID)
	return err
}

// PostgresGasBankManager implements GasBankManager using PostgreSQL.
type PostgresGasBankManager struct {
	db  *sql.DB
	log *logger.Logger
}

// NewPostgresGasBankManager creates a new PostgreSQL gas bank manager.
func NewPostgresGasBankManager(db *sql.DB, log *logger.Logger) *PostgresGasBankManager {
	if log == nil {
		log = logger.NewDefault("gasbank-manager")
	}
	return &PostgresGasBankManager{db: db, log: log}
}

// EnsureSchema creates the required tables.
func (m *PostgresGasBankManager) EnsureSchema(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS account_balances (
			account_id TEXT PRIMARY KEY,
			available BIGINT NOT NULL DEFAULT 0,
			reserved BIGINT NOT NULL DEFAULT 0,
			total_deposited BIGINT NOT NULL DEFAULT 0,
			total_withdrawn BIGINT NOT NULL DEFAULT 0,
			total_fees_paid BIGINT NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS balance_transactions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			type TEXT NOT NULL,
			amount BIGINT NOT NULL,
			reference TEXT,
			tx_hash TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_balance_transactions_account ON balance_transactions(account_id);
		CREATE INDEX IF NOT EXISTS idx_balance_transactions_created ON balance_transactions(created_at);
	`)
	return err
}

func (m *PostgresGasBankManager) GetBalance(ctx context.Context, accountID string) (*Balance, error) {
	var b Balance
	b.AccountID = accountID

	err := m.db.QueryRowContext(ctx, `
		SELECT available, reserved, total_deposited, total_withdrawn, total_fees_paid, updated_at
		FROM account_balances WHERE account_id = $1
	`, accountID).Scan(&b.Available, &b.Reserved, &b.TotalDeposited, &b.TotalWithdrawn, &b.TotalFeesPaid, &b.UpdatedAt)

	if err == sql.ErrNoRows {
		// Return zero balance for new accounts
		b.UpdatedAt = time.Now()
		return &b, nil
	}
	if err != nil {
		return nil, err
	}

	return &b, nil
}

func (m *PostgresGasBankManager) GetTransactionHistory(ctx context.Context, accountID string, limit int) ([]*Transaction, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, account_id, type, amount, reference, tx_hash, created_at
		FROM balance_transactions WHERE account_id = $1
		ORDER BY created_at DESC LIMIT $2
	`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*Transaction
	for rows.Next() {
		var t Transaction
		var reference, txHash sql.NullString
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Type, &t.Amount, &reference, &txHash, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Reference = reference.String
		t.TxHash = txHash.String
		transactions = append(transactions, &t)
	}

	return transactions, rows.Err()
}

func (m *PostgresGasBankManager) EstimateFee(ctx context.Context, serviceType string, params map[string]any) (int64, error) {
	// Fee estimation based on service type
	switch serviceType {
	case "oracle":
		return 100000, nil // 0.001 GAS
	case "vrf":
		return 200000, nil // 0.002 GAS
	case "datafeeds":
		return 50000, nil // 0.0005 GAS
	case "automation":
		return 150000, nil // 0.0015 GAS
	case "functions":
		// Based on memory/timeout
		memory, _ := params["memory_mb"].(float64)
		timeout, _ := params["timeout_seconds"].(float64)
		if memory == 0 {
			memory = 128
		}
		if timeout == 0 {
			timeout = 30
		}
		return int64(memory * timeout * 100), nil
	default:
		return 100000, nil
	}
}

// Helper functions

func generateAccountID(owner string) string {
	hash := sha256.Sum256([]byte(owner + time.Now().String()))
	return "acc_" + hex.EncodeToString(hash[:8])
}

func generateContractID(accountID, scriptHash string) string {
	hash := sha256.Sum256([]byte(accountID + scriptHash + time.Now().String()))
	return "ctr_" + hex.EncodeToString(hash[:8])
}

// PostgresAutomationManager implements AutomationManager using PostgreSQL.
type PostgresAutomationManager struct {
	db  *sql.DB
	log *logger.Logger
}

// NewPostgresAutomationManager creates a new PostgreSQL automation manager.
func NewPostgresAutomationManager(db *sql.DB, log *logger.Logger) *PostgresAutomationManager {
	if log == nil {
		log = logger.NewDefault("automation-manager")
	}
	return &PostgresAutomationManager{db: db, log: log}
}

// EnsureSchema creates the required tables.
func (m *PostgresAutomationManager) EnsureSchema(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_functions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			runtime TEXT NOT NULL,
			code TEXT NOT NULL,
			code_hash TEXT NOT NULL,
			entry_point TEXT,
			timeout_seconds INTEGER DEFAULT 30,
			memory_mb INTEGER DEFAULT 128,
			secrets TEXT[],
			env_vars JSONB,
			enabled BOOLEAN NOT NULL DEFAULT true,
			status TEXT NOT NULL DEFAULT 'active',
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_run_at TIMESTAMPTZ
		);

		CREATE TABLE IF NOT EXISTS function_triggers (
			id TEXT PRIMARY KEY,
			function_id TEXT NOT NULL REFERENCES user_functions(id) ON DELETE CASCADE,
			type TEXT NOT NULL,
			schedule TEXT,
			event_type TEXT,
			contract TEXT,
			webhook_url TEXT,
			enabled BOOLEAN NOT NULL DEFAULT true,
			status TEXT NOT NULL DEFAULT 'active',
			config JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_fired_at TIMESTAMPTZ
		);

		CREATE INDEX IF NOT EXISTS idx_user_functions_account ON user_functions(account_id);
		CREATE INDEX IF NOT EXISTS idx_user_functions_status ON user_functions(status);
		CREATE INDEX IF NOT EXISTS idx_function_triggers_function ON function_triggers(function_id);
		CREATE INDEX IF NOT EXISTS idx_function_triggers_type ON function_triggers(type);
	`)
	return err
}

func (m *PostgresAutomationManager) DeployFunction(ctx context.Context, accountID string, spec *FunctionSpec) (string, error) {
	id := generateFunctionID(accountID, spec.Name)

	envVarsJSON, _ := json.Marshal(spec.EnvVars)
	metadataJSON, _ := json.Marshal(spec.Metadata)

	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	memory := spec.Memory
	if memory <= 0 {
		memory = 128
	}
	entryPoint := spec.EntryPoint
	if entryPoint == "" {
		entryPoint = "main"
	}

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO user_functions (
			id, account_id, name, description, runtime, code, code_hash,
			entry_point, timeout_seconds, memory_mb, secrets, env_vars, metadata,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now(), now()
		)
	`, id, accountID, spec.Name, spec.Description, spec.Runtime, spec.Code, spec.CodeHash,
		entryPoint, timeout, memory, spec.Secrets, envVarsJSON, metadataJSON)
	if err != nil {
		return "", fmt.Errorf("deploy function: %w", err)
	}

	m.log.WithField("function_id", id).WithField("account_id", accountID).Info("function deployed")
	return id, nil
}

func (m *PostgresAutomationManager) UpdateFunction(ctx context.Context, functionID string, spec *FunctionSpec) error {
	envVarsJSON, _ := json.Marshal(spec.EnvVars)
	metadataJSON, _ := json.Marshal(spec.Metadata)

	_, err := m.db.ExecContext(ctx, `
		UPDATE user_functions SET
			name = $2, description = $3, runtime = $4, code = $5, code_hash = $6,
			entry_point = $7, timeout_seconds = $8, memory_mb = $9, secrets = $10,
			env_vars = $11, metadata = $12, updated_at = now()
		WHERE id = $1
	`, functionID, spec.Name, spec.Description, spec.Runtime, spec.Code, spec.CodeHash,
		spec.EntryPoint, spec.Timeout, spec.Memory, spec.Secrets, envVarsJSON, metadataJSON)
	return err
}

func (m *PostgresAutomationManager) GetFunction(ctx context.Context, functionID string) (*Function, error) {
	var f Function
	var metadataJSON []byte
	var lastRunAt sql.NullTime

	err := m.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, description, runtime, code_hash, entry_point,
			timeout_seconds, memory_mb, enabled, status, metadata, created_at, updated_at, last_run_at
		FROM user_functions WHERE id = $1
	`, functionID).Scan(&f.ID, &f.AccountID, &f.Name, &f.Description, &f.Runtime, &f.CodeHash,
		&f.EntryPoint, &f.Timeout, &f.Memory, &f.Enabled, &f.Status, &metadataJSON,
		&f.CreatedAt, &f.UpdatedAt, &lastRunAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("function not found")
	}
	if err != nil {
		return nil, err
	}

	if lastRunAt.Valid {
		f.LastRunAt = &lastRunAt.Time
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &f.Metadata)
	}
	// Note: envVarsJSON not exposed in Function struct for security

	return &f, nil
}

func (m *PostgresAutomationManager) ListFunctions(ctx context.Context, accountID string) ([]*Function, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, account_id, name, description, runtime, code_hash, entry_point,
			timeout_seconds, memory_mb, enabled, status, metadata, created_at, updated_at, last_run_at
		FROM user_functions WHERE account_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var functions []*Function
	for rows.Next() {
		var f Function
		var metadataJSON []byte
		var lastRunAt sql.NullTime

		if err := rows.Scan(&f.ID, &f.AccountID, &f.Name, &f.Description, &f.Runtime, &f.CodeHash,
			&f.EntryPoint, &f.Timeout, &f.Memory, &f.Enabled, &f.Status, &metadataJSON,
			&f.CreatedAt, &f.UpdatedAt, &lastRunAt); err != nil {
			return nil, err
		}

		if lastRunAt.Valid {
			f.LastRunAt = &lastRunAt.Time
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &f.Metadata)
		}
		functions = append(functions, &f)
	}

	return functions, rows.Err()
}

func (m *PostgresAutomationManager) EnableFunction(ctx context.Context, functionID string, enabled bool) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE user_functions SET enabled = $2, updated_at = now() WHERE id = $1
	`, functionID, enabled)
	return err
}

func (m *PostgresAutomationManager) DeleteFunction(ctx context.Context, functionID string) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE user_functions SET status = 'deleted', updated_at = now() WHERE id = $1
	`, functionID)
	return err
}

// Trigger management

func (m *PostgresAutomationManager) CreateTrigger(ctx context.Context, functionID string, spec *TriggerSpec) (string, error) {
	id := generateTriggerID(functionID, spec.Type)

	configJSON, _ := json.Marshal(spec.Config)

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO function_triggers (
			id, function_id, type, schedule, event_type, contract, webhook_url,
			enabled, config, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now()
		)
	`, id, functionID, spec.Type, core.ToNullString(spec.Schedule), core.ToNullString(spec.EventType),
		core.ToNullString(spec.Contract), core.ToNullString(spec.WebhookURL), spec.Enabled, configJSON)
	if err != nil {
		return "", fmt.Errorf("create trigger: %w", err)
	}

	m.log.WithField("trigger_id", id).WithField("function_id", functionID).Info("trigger created")
	return id, nil
}

func (m *PostgresAutomationManager) UpdateTrigger(ctx context.Context, triggerID string, spec *TriggerSpec) error {
	configJSON, _ := json.Marshal(spec.Config)

	_, err := m.db.ExecContext(ctx, `
		UPDATE function_triggers SET
			type = $2, schedule = $3, event_type = $4, contract = $5, webhook_url = $6,
			enabled = $7, config = $8, updated_at = now()
		WHERE id = $1
	`, triggerID, spec.Type, core.ToNullString(spec.Schedule), core.ToNullString(spec.EventType),
		core.ToNullString(spec.Contract), core.ToNullString(spec.WebhookURL), spec.Enabled, configJSON)
	return err
}

func (m *PostgresAutomationManager) DeleteTrigger(ctx context.Context, triggerID string) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE function_triggers SET status = 'deleted', updated_at = now() WHERE id = $1
	`, triggerID)
	return err
}

func (m *PostgresAutomationManager) ListTriggers(ctx context.Context, functionID string) ([]*Trigger, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, function_id, type, schedule, event_type, contract, enabled, status,
			config, created_at, updated_at, last_fired_at
		FROM function_triggers WHERE function_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`, functionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []*Trigger
	for rows.Next() {
		var t Trigger
		var schedule, eventType, contract sql.NullString
		var configJSON []byte
		var lastFiredAt sql.NullTime

		if err := rows.Scan(&t.ID, &t.FunctionID, &t.Type, &schedule, &eventType, &contract,
			&t.Enabled, &t.Status, &configJSON, &t.CreatedAt, &t.UpdatedAt, &lastFiredAt); err != nil {
			return nil, err
		}

		t.Schedule = schedule.String
		t.EventType = eventType.String
		t.Contract = contract.String
		if lastFiredAt.Valid {
			t.LastFiredAt = &lastFiredAt.Time
		}
		if len(configJSON) > 0 {
			json.Unmarshal(configJSON, &t.Config)
		}
		triggers = append(triggers, &t)
	}

	return triggers, rows.Err()
}

// Helper functions for ID generation

func generateFunctionID(accountID, name string) string {
	hash := sha256.Sum256([]byte(accountID + name + time.Now().String()))
	return "fn_" + hex.EncodeToString(hash[:8])
}

func generateTriggerID(functionID, triggerType string) string {
	hash := sha256.Sum256([]byte(functionID + triggerType + time.Now().String()))
	return "trg_" + hex.EncodeToString(hash[:8])
}

// Compile-time interface checks
var (
	_ AccountManager    = (*PostgresAccountManager)(nil)
	_ SecretsManager    = (*PostgresSecretsManager)(nil)
	_ ContractManager   = (*PostgresContractManager)(nil)
	_ GasBankManager    = (*PostgresGasBankManager)(nil)
	_ AutomationManager = (*PostgresAutomationManager)(nil)
)
