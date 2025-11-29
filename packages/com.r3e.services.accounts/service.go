package accounts

import (
	"context"
	"strings"

	"github.com/R3E-Network/service_layer/applications/storage"
	"github.com/R3E-Network/service_layer/domain/account"
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
	store storage.AccountStore
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

// Start marks the service as ready.
func (s *Service) Start(ctx context.Context) error { _ = ctx; s.MarkReady(true); return nil }

// Stop marks the service as not ready.
func (s *Service) Stop(ctx context.Context) error { _ = ctx; s.MarkReady(false); return nil }

// Ready reports readiness for engine status.
func (s *Service) Ready(ctx context.Context) error {
	return s.ServiceBase.Ready(ctx)
}

// New creates an account service backed by the provided store.
func New(store storage.AccountStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("accounts")
	}
	svc := &Service{store: store, log: log, base: core.NewBase(store)}
	svc.SetName(svc.Name())
	return svc
}

// Create provisions a new account with optional metadata.
func (s *Service) Create(ctx context.Context, owner string, metadata map[string]string) (account.Account, error) {
	if owner == "" {
		return account.Account{}, core.RequiredError("owner")
	}

	acct := account.Account{Owner: owner, Metadata: metadata}
	created, err := s.store.CreateAccount(ctx, acct)
	if err != nil {
		return account.Account{}, err
	}

	s.log.WithField("account_id", created.ID).
		WithField("owner", owner).
		Info("account created")
	return created, nil
}

// UpdateMetadata replaces the metadata map for the specified account.
func (s *Service) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) (account.Account, error) {
	if err := s.base.EnsureAccount(ctx, id); err != nil {
		return account.Account{}, err
	}
	acct, err := s.store.GetAccount(ctx, id)
	if err != nil {
		return account.Account{}, err
	}
	acct.Metadata = metadata
	updated, err := s.store.UpdateAccount(ctx, acct)
	if err != nil {
		return account.Account{}, err
	}
	s.log.WithField("account_id", id).Info("account metadata updated")
	return updated, nil
}

// Get retrieves an account by identifier.
func (s *Service) Get(ctx context.Context, id string) (account.Account, error) {
	if err := s.base.EnsureAccount(ctx, id); err != nil {
		return account.Account{}, err
	}
	return s.store.GetAccount(ctx, id)
}

// List returns all accounts.
func (s *Service) List(ctx context.Context) ([]account.Account, error) {
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
