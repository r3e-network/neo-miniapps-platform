package accounts

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service provides high-level operations for managing accounts. It acts as the
// entry point for other modules that need to reason about tenants or logical
// owners.
type Service struct {
	store storage.AccountStore
	log   *logger.Logger
}

// NewService constructs an account service backed by the provided store.
func NewService(store storage.AccountStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("accounts")
	}
	return &Service{
		store: store,
		log:   log,
	}
}

// Create provisions a new account.
func (s *Service) Create(ctx context.Context, owner string, metadata map[string]string) (account.Account, error) {
	if owner == "" {
		return account.Account{}, fmt.Errorf("owner is required")
	}

	acct := account.Account{
		Owner:    owner,
		Metadata: metadata,
	}

	created, err := s.store.CreateAccount(ctx, acct)
	if err != nil {
		return account.Account{}, err
	}

	s.log.Infof("account %s created for %s", created.ID, owner)
	return created, nil
}

// UpdateMetadata replaces the metadata map for the specified account.
func (s *Service) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) (account.Account, error) {
	acct, err := s.store.GetAccount(ctx, id)
	if err != nil {
		return account.Account{}, err
	}

	acct.Metadata = metadata
	updated, err := s.store.UpdateAccount(ctx, acct)
	if err != nil {
		return account.Account{}, err
	}

	s.log.Infof("account %s metadata updated", id)
	return updated, nil
}

// Get returns the account with the given identifier.
func (s *Service) Get(ctx context.Context, id string) (account.Account, error) {
	return s.store.GetAccount(ctx, id)
}

// List returns all accounts.
func (s *Service) List(ctx context.Context) ([]account.Account, error) {
	return s.store.ListAccounts(ctx)
}

// Delete permanently removes an account.
func (s *Service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	if err := s.store.DeleteAccount(ctx, id); err != nil {
		return err
	}
	s.log.Infof("account %s deleted", id)
	return nil
}
