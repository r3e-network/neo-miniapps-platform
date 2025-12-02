// Package api provides the user-facing API for direct service layer interaction.
// This allows users to manage accounts, secrets, contracts, and automation
// without going through on-chain transactions.
//
// DEPRECATED: This package contains legacy service-specific code that violates
// the "Service Operating System" architecture principle. The framework layer
// (system/) should be service-agnostic. New code should use the declarative
// HTTP API pattern in service packages (HTTP{Method}{Path} methods).
// This package will be refactored to remove service-specific dependencies.
package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/events"
)

// UserService provides direct user interaction with the service layer.
// This complements on-chain interactions for operations that don't require
// blockchain consensus (e.g., secret management, function deployment).
type UserService struct {
	accounts    AccountManager
	secrets     SecretsManager
	contracts   ContractManager
	automation  AutomationManager
	gasbank     GasBankManager
	router      *events.RequestRouter
	log         *logger.Logger
}

// AccountManager handles account operations.
type AccountManager interface {
	CreateAccount(ctx context.Context, ownerAddress string, metadata map[string]string) (string, error)
	GetAccount(ctx context.Context, accountID string) (*Account, error)
	UpdateAccount(ctx context.Context, accountID string, metadata map[string]string) error
	LinkWallet(ctx context.Context, accountID, walletAddress string) error
	UnlinkWallet(ctx context.Context, accountID, walletAddress string) error
	ListWallets(ctx context.Context, accountID string) ([]Wallet, error)
}

// SecretsManager handles secret key operations.
type SecretsManager interface {
	SetSecret(ctx context.Context, accountID, name string, value []byte, encrypted bool) error
	GetSecret(ctx context.Context, accountID, name string) ([]byte, error)
	DeleteSecret(ctx context.Context, accountID, name string) error
	ListSecrets(ctx context.Context, accountID string) ([]SecretInfo, error)
	RotateSecret(ctx context.Context, accountID, name string, newValue []byte) error
}

// ContractManager handles user contract registration.
type ContractManager interface {
	RegisterContract(ctx context.Context, accountID string, spec *ContractSpec) (string, error)
	UpdateContract(ctx context.Context, contractID string, spec *ContractSpec) error
	GetContract(ctx context.Context, contractID string) (*Contract, error)
	ListContracts(ctx context.Context, accountID string) ([]*Contract, error)
	PauseContract(ctx context.Context, contractID string, paused bool) error
	DeleteContract(ctx context.Context, contractID string) error
}

// AutomationManager handles automation/function deployment.
type AutomationManager interface {
	DeployFunction(ctx context.Context, accountID string, spec *FunctionSpec) (string, error)
	UpdateFunction(ctx context.Context, functionID string, spec *FunctionSpec) error
	GetFunction(ctx context.Context, functionID string) (*Function, error)
	ListFunctions(ctx context.Context, accountID string) ([]*Function, error)
	EnableFunction(ctx context.Context, functionID string, enabled bool) error
	DeleteFunction(ctx context.Context, functionID string) error

	// Trigger management
	CreateTrigger(ctx context.Context, functionID string, trigger *TriggerSpec) (string, error)
	UpdateTrigger(ctx context.Context, triggerID string, trigger *TriggerSpec) error
	DeleteTrigger(ctx context.Context, triggerID string) error
	ListTriggers(ctx context.Context, functionID string) ([]*Trigger, error)
}

// GasBankManager handles balance operations.
type GasBankManager interface {
	GetBalance(ctx context.Context, accountID string) (*Balance, error)
	GetTransactionHistory(ctx context.Context, accountID string, limit int) ([]*Transaction, error)
	EstimateFee(ctx context.Context, serviceType string, params map[string]any) (int64, error)
}

// Data types

// Account represents a user account.
type Account struct {
	ID        string            `json:"id"`
	Owner     string            `json:"owner"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Wallet represents a linked wallet.
type Wallet struct {
	Address   string    `json:"address"`
	AccountID string    `json:"account_id"`
	Status    string    `json:"status"`
	LinkedAt  time.Time `json:"linked_at"`
}

// SecretInfo contains metadata about a secret (not the value).
type SecretInfo struct {
	Name      string    `json:"name"`
	Encrypted bool      `json:"encrypted"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ContractSpec defines a user contract registration.
type ContractSpec struct {
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	ScriptHash   string            `json:"script_hash"`
	Capabilities []string          `json:"capabilities"` // oracle, vrf, datafeeds, etc.
	CallbackABI  map[string]any    `json:"callback_abi,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Contract represents a registered user contract.
type Contract struct {
	ID           string            `json:"id"`
	AccountID    string            `json:"account_id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	ScriptHash   string            `json:"script_hash"`
	Capabilities []string          `json:"capabilities"`
	Status       string            `json:"status"`
	Paused       bool              `json:"paused"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// FunctionSpec defines an automation function.
type FunctionSpec struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Runtime     string            `json:"runtime"` // javascript, python, wasm
	Code        string            `json:"code"`    // Base64 encoded or inline
	CodeHash    string            `json:"code_hash,omitempty"`
	EntryPoint  string            `json:"entry_point,omitempty"`
	Timeout     int               `json:"timeout_seconds,omitempty"`
	Memory      int               `json:"memory_mb,omitempty"`
	Secrets     []string          `json:"secrets,omitempty"` // Secret names to inject
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Function represents a deployed automation function.
type Function struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Runtime     string            `json:"runtime"`
	CodeHash    string            `json:"code_hash"`
	EntryPoint  string            `json:"entry_point"`
	Timeout     int               `json:"timeout_seconds"`
	Memory      int               `json:"memory_mb"`
	Enabled     bool              `json:"enabled"`
	Status      string            `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastRunAt   *time.Time        `json:"last_run_at,omitempty"`
}

// TriggerSpec defines a function trigger.
type TriggerSpec struct {
	Type       string         `json:"type"` // cron, event, webhook, manual
	Schedule   string         `json:"schedule,omitempty"` // Cron expression
	EventType  string         `json:"event_type,omitempty"`
	Contract   string         `json:"contract,omitempty"`
	WebhookURL string         `json:"webhook_url,omitempty"`
	Enabled    bool           `json:"enabled"`
	Config     map[string]any `json:"config,omitempty"`
}

// Trigger represents a function trigger.
type Trigger struct {
	ID         string         `json:"id"`
	FunctionID string         `json:"function_id"`
	Type       string         `json:"type"`
	Schedule   string         `json:"schedule,omitempty"`
	EventType  string         `json:"event_type,omitempty"`
	Contract   string         `json:"contract,omitempty"`
	Enabled    bool           `json:"enabled"`
	Status     string         `json:"status"`
	Config     map[string]any `json:"config,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	LastFiredAt *time.Time    `json:"last_fired_at,omitempty"`
}

// Balance represents account balance.
type Balance struct {
	AccountID      string    `json:"account_id"`
	Available      int64     `json:"available"`
	Reserved       int64     `json:"reserved"`
	TotalDeposited int64     `json:"total_deposited"`
	TotalWithdrawn int64     `json:"total_withdrawn"`
	TotalFeesPaid  int64     `json:"total_fees_paid"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Transaction represents a balance transaction.
type Transaction struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Type      string    `json:"type"` // deposit, withdrawal, fee, refund
	Amount    int64     `json:"amount"`
	Reference string    `json:"reference,omitempty"`
	TxHash    string    `json:"tx_hash,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// UserServiceConfig configures the user service.
type UserServiceConfig struct {
	Accounts   AccountManager
	Secrets    SecretsManager
	Contracts  ContractManager
	Automation AutomationManager
	GasBank    GasBankManager
	Router     *events.RequestRouter
	Logger     *logger.Logger
}

// NewUserService creates a new user service.
func NewUserService(cfg UserServiceConfig) *UserService {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault("user-api")
	}
	return &UserService{
		accounts:   cfg.Accounts,
		secrets:    cfg.Secrets,
		contracts:  cfg.Contracts,
		automation: cfg.Automation,
		gasbank:    cfg.GasBank,
		router:     cfg.Router,
		log:        cfg.Logger,
	}
}

// Account operations

// CreateAccount creates a new user account.
func (s *UserService) CreateAccount(ctx context.Context, ownerAddress string, metadata map[string]string) (string, error) {
	if s.accounts == nil {
		return "", fmt.Errorf("account manager not configured")
	}
	ownerAddress = strings.TrimSpace(ownerAddress)
	if ownerAddress == "" {
		return "", fmt.Errorf("owner address required")
	}
	return s.accounts.CreateAccount(ctx, ownerAddress, metadata)
}

// GetAccount retrieves account details.
func (s *UserService) GetAccount(ctx context.Context, accountID string) (*Account, error) {
	if s.accounts == nil {
		return nil, fmt.Errorf("account manager not configured")
	}
	return s.accounts.GetAccount(ctx, accountID)
}

// UpdateAccount updates account metadata.
func (s *UserService) UpdateAccount(ctx context.Context, accountID string, metadata map[string]string) error {
	if s.accounts == nil {
		return fmt.Errorf("account manager not configured")
	}
	return s.accounts.UpdateAccount(ctx, accountID, metadata)
}

// LinkWallet links a wallet to an account.
func (s *UserService) LinkWallet(ctx context.Context, accountID, walletAddress string) error {
	if s.accounts == nil {
		return fmt.Errorf("account manager not configured")
	}
	return s.accounts.LinkWallet(ctx, accountID, walletAddress)
}

// Secret operations

// SetSecret stores a secret for an account.
func (s *UserService) SetSecret(ctx context.Context, accountID, name string, value []byte, encrypted bool) error {
	if s.secrets == nil {
		return fmt.Errorf("secrets manager not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("secret name required")
	}
	if len(value) == 0 {
		return fmt.Errorf("secret value required")
	}
	return s.secrets.SetSecret(ctx, accountID, name, value, encrypted)
}

// GetSecret retrieves a secret value.
func (s *UserService) GetSecret(ctx context.Context, accountID, name string) ([]byte, error) {
	if s.secrets == nil {
		return nil, fmt.Errorf("secrets manager not configured")
	}
	return s.secrets.GetSecret(ctx, accountID, name)
}

// DeleteSecret removes a secret.
func (s *UserService) DeleteSecret(ctx context.Context, accountID, name string) error {
	if s.secrets == nil {
		return fmt.Errorf("secrets manager not configured")
	}
	return s.secrets.DeleteSecret(ctx, accountID, name)
}

// ListSecrets lists all secrets for an account (metadata only).
func (s *UserService) ListSecrets(ctx context.Context, accountID string) ([]SecretInfo, error) {
	if s.secrets == nil {
		return nil, fmt.Errorf("secrets manager not configured")
	}
	return s.secrets.ListSecrets(ctx, accountID)
}

// Contract operations

// RegisterContract registers a user contract.
func (s *UserService) RegisterContract(ctx context.Context, accountID string, spec *ContractSpec) (string, error) {
	if s.contracts == nil {
		return "", fmt.Errorf("contract manager not configured")
	}
	if spec == nil {
		return "", fmt.Errorf("contract spec required")
	}
	spec.Name = strings.TrimSpace(spec.Name)
	spec.ScriptHash = strings.TrimSpace(spec.ScriptHash)
	if spec.Name == "" {
		return "", fmt.Errorf("contract name required")
	}
	if spec.ScriptHash == "" {
		return "", fmt.Errorf("script hash required")
	}
	return s.contracts.RegisterContract(ctx, accountID, spec)
}

// GetContract retrieves contract details.
func (s *UserService) GetContract(ctx context.Context, contractID string) (*Contract, error) {
	if s.contracts == nil {
		return nil, fmt.Errorf("contract manager not configured")
	}
	return s.contracts.GetContract(ctx, contractID)
}

// ListContracts lists all contracts for an account.
func (s *UserService) ListContracts(ctx context.Context, accountID string) ([]*Contract, error) {
	if s.contracts == nil {
		return nil, fmt.Errorf("contract manager not configured")
	}
	return s.contracts.ListContracts(ctx, accountID)
}

// PauseContract pauses or unpauses a contract.
func (s *UserService) PauseContract(ctx context.Context, contractID string, paused bool) error {
	if s.contracts == nil {
		return fmt.Errorf("contract manager not configured")
	}
	return s.contracts.PauseContract(ctx, contractID, paused)
}

// Function operations

// DeployFunction deploys an automation function.
func (s *UserService) DeployFunction(ctx context.Context, accountID string, spec *FunctionSpec) (string, error) {
	if s.automation == nil {
		return "", fmt.Errorf("automation manager not configured")
	}
	if spec == nil {
		return "", fmt.Errorf("function spec required")
	}
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Runtime = strings.TrimSpace(spec.Runtime)
	if spec.Name == "" {
		return "", fmt.Errorf("function name required")
	}
	if spec.Runtime == "" {
		return "", fmt.Errorf("runtime required")
	}
	if spec.Code == "" {
		return "", fmt.Errorf("code required")
	}
	return s.automation.DeployFunction(ctx, accountID, spec)
}

// GetFunction retrieves function details.
func (s *UserService) GetFunction(ctx context.Context, functionID string) (*Function, error) {
	if s.automation == nil {
		return nil, fmt.Errorf("automation manager not configured")
	}
	return s.automation.GetFunction(ctx, functionID)
}

// ListFunctions lists all functions for an account.
func (s *UserService) ListFunctions(ctx context.Context, accountID string) ([]*Function, error) {
	if s.automation == nil {
		return nil, fmt.Errorf("automation manager not configured")
	}
	return s.automation.ListFunctions(ctx, accountID)
}

// EnableFunction enables or disables a function.
func (s *UserService) EnableFunction(ctx context.Context, functionID string, enabled bool) error {
	if s.automation == nil {
		return fmt.Errorf("automation manager not configured")
	}
	return s.automation.EnableFunction(ctx, functionID, enabled)
}

// CreateTrigger creates a trigger for a function.
func (s *UserService) CreateTrigger(ctx context.Context, functionID string, trigger *TriggerSpec) (string, error) {
	if s.automation == nil {
		return "", fmt.Errorf("automation manager not configured")
	}
	if trigger == nil {
		return "", fmt.Errorf("trigger spec required")
	}
	return s.automation.CreateTrigger(ctx, functionID, trigger)
}

// Balance operations

// GetBalance retrieves account balance.
func (s *UserService) GetBalance(ctx context.Context, accountID string) (*Balance, error) {
	if s.gasbank == nil {
		return nil, fmt.Errorf("gasbank manager not configured")
	}
	return s.gasbank.GetBalance(ctx, accountID)
}

// GetTransactionHistory retrieves balance transaction history.
func (s *UserService) GetTransactionHistory(ctx context.Context, accountID string, limit int) ([]*Transaction, error) {
	if s.gasbank == nil {
		return nil, fmt.Errorf("gasbank manager not configured")
	}
	if limit <= 0 {
		limit = 50
	}
	return s.gasbank.GetTransactionHistory(ctx, accountID, limit)
}

// EstimateFee estimates the fee for a service operation.
func (s *UserService) EstimateFee(ctx context.Context, serviceType string, params map[string]any) (int64, error) {
	if s.gasbank == nil {
		return 0, fmt.Errorf("gasbank manager not configured")
	}
	return s.gasbank.EstimateFee(ctx, serviceType, params)
}

// Request operations (via router)

// SubmitRequest submits a service request directly.
func (s *UserService) SubmitRequest(ctx context.Context, accountID string, serviceType events.ServiceType, payload map[string]any) (*events.Request, error) {
	if s.router == nil {
		return nil, fmt.Errorf("request router not configured")
	}

	// Check balance first
	if s.gasbank != nil {
		fee, err := s.gasbank.EstimateFee(ctx, string(serviceType), payload)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate fee: %w", err)
		}
		balance, err := s.gasbank.GetBalance(ctx, accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get balance: %w", err)
		}
		if balance.Available < fee {
			return nil, fmt.Errorf("insufficient balance: need %d, have %d", fee, balance.Available)
		}
	}

	req, err := s.router.CreateRequest(ctx, accountID, serviceType, payload)
	if err != nil {
		return nil, err
	}

	if err := s.router.SubmitRequest(req); err != nil {
		return nil, err
	}

	return req, nil
}

// GetRequest retrieves a request by ID.
func (s *UserService) GetRequest(ctx context.Context, requestID string) (*events.Request, error) {
	if s.router == nil {
		return nil, fmt.Errorf("request router not configured")
	}
	return s.router.GetRequest(ctx, requestID)
}

// ListRequests lists requests for an account.
func (s *UserService) ListRequests(ctx context.Context, accountID string, serviceType events.ServiceType, status events.RequestStatus, limit int) ([]*events.Request, error) {
	if s.router == nil {
		return nil, fmt.Errorf("request router not configured")
	}
	return s.router.ListRequests(ctx, accountID, serviceType, status, limit)
}
