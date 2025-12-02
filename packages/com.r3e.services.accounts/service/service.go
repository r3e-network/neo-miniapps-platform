package accounts

import (
	"context"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
	
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Compile-time check: Service exposes account lifecycle methods used by the core engine adapter.
type accountAPI interface {
	CreateAccount(context.Context, string, map[string]string) (string, error)
	ListAccounts(context.Context) ([]any, error)
}

var _ accountAPI = (*Service)(nil)

// Service manages account lifecycle operations.
// Uses SandboxedServiceEngine for capability-based access control.
type Service struct {
	*framework.SandboxedServiceEngine // Provides: Name, Domain, Manifest, Descriptor, ValidateAccount, Logger, sandbox capabilities
	store                             Store
	base                              *core.Base
}

// New creates an account service backed by the provided store.
func New(accounts framework.AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "accounts",
				Domain:       "accounts",
				Description:  "Account registry and metadata",
				DependsOn:    []string{"store"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceAccount},
				Capabilities: []string{"accounts"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelSystem, // Core service needs system level
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapServiceCall,
			},
			StorageQuota: 100 * 1024 * 1024, // 100MB for accounts
		}),
		store: store,
		base:  core.NewBaseFromStore[Account](store),
	}
}

// Create provisions a new account with optional metadata.
func (s *Service) Create(ctx context.Context, owner string, metadata map[string]string) (Account, error) {
	if owner == "" {
		return Account{}, core.RequiredError("owner")
	}

	acct := Account{Owner: owner, Metadata: metadata}
	attrs := map[string]string{"resource": "account", "owner": owner}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateAccount(ctx, acct)
	if err == nil {
		attrs["account_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return Account{}, err
	}

	s.Logger().WithField("account_id", created.ID).
		WithField("owner", owner).
		Info("account created")
	s.LogCreated("account", created.ID, created.ID)
	s.IncrementCounter("accounts_created_total", map[string]string{"account_id": created.ID})
	return created, nil
}

// UpdateMetadata replaces the metadata map for the specified account.
func (s *Service) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) (Account, error) {
	if err := s.base.EnsureAccount(ctx, id); err != nil {
		return Account{}, err
	}
	acct, err := s.store.GetAccount(ctx, id)
	if err != nil {
		return Account{}, err
	}
	acct.Metadata = metadata
	attrs := map[string]string{"resource": "account", "account_id": id, "op": "update_metadata"}
	ctx, finish := s.StartObservation(ctx, attrs)
	updated, err := s.store.UpdateAccount(ctx, acct)
	finish(err)
	if err != nil {
		return Account{}, err
	}
	s.Logger().WithField("account_id", id).Info("account metadata updated")
	s.LogUpdated("account", id, id)
	s.IncrementCounter("accounts_metadata_updated_total", map[string]string{"account_id": id})
	return updated, nil
}

// Get retrieves an account by identifier.
func (s *Service) Get(ctx context.Context, id string) (Account, error) {
	if err := s.base.EnsureAccount(ctx, id); err != nil {
		return Account{}, err
	}
	return s.store.GetAccount(ctx, id)
}

// List returns all accounts.
func (s *Service) List(ctx context.Context) ([]Account, error) {
	return s.store.ListAccounts(ctx)
}

// Delete removes an account by identifier.
func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if err := s.base.EnsureAccount(ctx, id); err != nil {
		return err
	}
	attrs := map[string]string{"resource": "account", "account_id": id}
	ctx, finish := s.StartObservation(ctx, attrs)
	if err := s.store.DeleteAccount(ctx, id); err != nil {
		finish(err)
		return err
	}
	finish(nil)
	s.Logger().WithField("account_id", id).Info("account deleted")
	s.LogDeleted("account", id, id)
	s.IncrementCounter("accounts_deleted_total", map[string]string{"account_id": id})
	return nil
}

// CreateAccount implements framework.AccountEngine for the core engine.
func (s *Service) CreateAccount(ctx context.Context, owner string, metadata map[string]string) (string, error) {
	acct, err := s.Create(ctx, owner, metadata)
	if err != nil {
		return "", err
	}
	return acct.ID, nil
}

// ListAccounts implements framework.AccountEngine for the core engine.
func (s *Service) ListAccounts(ctx context.Context) ([]any, error) {
	accts, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(accts))
	for _, a := range accts {
		out = append(out, a)
	}
	return out, nil
}

// AccountExists implements AccountChecker interface.
// Returns nil if the account exists, or an error if it does not.
func (s *Service) AccountExists(ctx context.Context, accountID string) error {
	return s.base.EnsureAccount(ctx, accountID)
}

// AccountTenant implements AccountChecker interface.
// Returns the tenant for an account (empty if none).
func (s *Service) AccountTenant(ctx context.Context, accountID string) string {
	acct, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return ""
	}
	if acct.Metadata != nil {
		if tenant, ok := acct.Metadata["tenant"]; ok {
			return tenant
		}
	}
	return ""
}

// CreateWorkspaceWallet creates a new workspace wallet.
func (s *Service) CreateWorkspaceWallet(ctx context.Context, wallet WorkspaceWallet) (WorkspaceWallet, error) {
	if err := s.base.EnsureAccount(ctx, wallet.WorkspaceID); err != nil {
		return WorkspaceWallet{}, err
	}
	wallet.WalletAddress = NormalizeWalletAddress(wallet.WalletAddress)
	attrs := map[string]string{"resource": "workspace_wallet", "workspace_id": wallet.WorkspaceID}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateWorkspaceWallet(ctx, wallet)
	if err == nil {
		attrs["wallet_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return WorkspaceWallet{}, err
	}
	s.LogCreated("workspace_wallet", created.ID, wallet.WorkspaceID)
	s.IncrementCounter("accounts_workspace_wallets_created_total", map[string]string{"workspace_id": wallet.WorkspaceID})
	return created, nil
}

// GetWorkspaceWallet retrieves a workspace wallet by ID.
func (s *Service) GetWorkspaceWallet(ctx context.Context, id string) (WorkspaceWallet, error) {
	return s.store.GetWorkspaceWallet(ctx, id)
}

// ListWorkspaceWallets lists all wallets for a workspace.
func (s *Service) ListWorkspaceWallets(ctx context.Context, workspaceID string) ([]WorkspaceWallet, error) {
	if err := s.base.EnsureAccount(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.store.ListWorkspaceWallets(ctx, workspaceID)
}

// FindWorkspaceWalletByAddress finds a wallet by address within a workspace.
func (s *Service) FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, walletAddr string) (WorkspaceWallet, error) {
	if err := s.base.EnsureAccount(ctx, workspaceID); err != nil {
		return WorkspaceWallet{}, err
	}
	return s.store.FindWorkspaceWalletByAddress(ctx, workspaceID, walletAddr)
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetWorkspaceWallets handles GET /workspace-wallets - list all wallets for an account.
func (s *Service) HTTPGetWorkspaceWallets(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListWorkspaceWallets(ctx, req.AccountID)
}

// HTTPPostWorkspaceWallets handles POST /workspace-wallets - create a new workspace wallet.
func (s *Service) HTTPPostWorkspaceWallets(ctx context.Context, req core.APIRequest) (any, error) {
	walletAddress, _ := req.Body["wallet_address"].(string)
	label, _ := req.Body["label"].(string)
	status, _ := req.Body["status"].(string)

	if err := ValidateWalletAddress(walletAddress); err != nil {
		return nil, err
	}

	wallet := WorkspaceWallet{
		WorkspaceID:   req.AccountID,
		WalletAddress: NormalizeWalletAddress(walletAddress),
		Label:         label,
		Status:        status,
	}

	return s.CreateWorkspaceWallet(ctx, wallet)
}

// HTTPGetWorkspaceWalletsById handles GET /workspace-wallets/{id} - get a specific wallet.
func (s *Service) HTTPGetWorkspaceWalletsById(ctx context.Context, req core.APIRequest) (any, error) {
	walletID := req.PathParams["id"]
	wallet, err := s.GetWorkspaceWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOwnership(wallet.WorkspaceID, req.AccountID, "wallet", walletID); err != nil {
		return nil, err
	}
	return wallet, nil
}
