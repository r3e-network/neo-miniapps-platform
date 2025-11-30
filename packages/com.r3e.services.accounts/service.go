package accounts

import (
	"context"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Compile-time check: Service exposes account lifecycle methods used by the core engine adapter.
type accountAPI interface {
	CreateAccount(context.Context, string, map[string]string) (string, error)
	ListAccounts(context.Context) ([]any, error)
}

var _ accountAPI = (*Service)(nil)

// Service manages account lifecycle operations.
type Service struct {
	framework.ServiceBase
	store Store
	log   *logger.Logger
	base  *core.Base
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "accounts" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "accounts" }

// Manifest describes how the accounts service plugs into the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Account registry and metadata",
		Layer:        "service",
		DependsOn:    []string{"store"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceAccount},
		Capabilities: []string{"accounts"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// Start/Stop/Ready are inherited from framework.ServiceBase.

// New creates an account service backed by the provided store.
func New(store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("accounts")
	}
	svc := &Service{store: store, log: log, base: core.NewBaseFromStore[Account](store)}
	svc.SetName(svc.Name())
	return svc
}

// Create provisions a new account with optional metadata.
func (s *Service) Create(ctx context.Context, owner string, metadata map[string]string) (Account, error) {
	if owner == "" {
		return Account{}, core.RequiredError("owner")
	}

	acct := Account{Owner: owner, Metadata: metadata}
	created, err := s.store.CreateAccount(ctx, acct)
	if err != nil {
		return Account{}, err
	}

	s.log.WithField("account_id", created.ID).
		WithField("owner", owner).
		Info("account created")
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
	updated, err := s.store.UpdateAccount(ctx, acct)
	if err != nil {
		return Account{}, err
	}
	s.log.WithField("account_id", id).Info("account metadata updated")
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
	if err := s.store.DeleteAccount(ctx, id); err != nil {
		return err
	}
	s.log.WithField("account_id", id).Info("account deleted")
	return nil
}

// CreateAccount implements engine.AccountEngine for the core engine.
func (s *Service) CreateAccount(ctx context.Context, owner string, metadata map[string]string) (string, error) {
	acct, err := s.Create(ctx, owner, metadata)
	if err != nil {
		return "", err
	}
	return acct.ID, nil
}

// ListAccounts implements engine.AccountEngine for the core engine.
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
	return s.store.CreateWorkspaceWallet(ctx, wallet)
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

