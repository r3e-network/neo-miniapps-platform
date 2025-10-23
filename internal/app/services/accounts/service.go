package accounts

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages account lifecycle operations.
type Service struct {
	store storage.AccountStore
	log   *logger.Logger
}

// New creates an account service backed by the provided store.
func New(store storage.AccountStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("accounts")
	}
	return &Service{store: store, log: log}
}

// Create provisions a new account with optional metadata.
func (s *Service) Create(ctx context.Context, owner string, metadata map[string]string) (account.Account, error) {
	if owner == "" {
		return account.Account{}, fmt.Errorf("owner is required")
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
	return s.store.GetAccount(ctx, id)
}

// List returns all accounts.
func (s *Service) List(ctx context.Context) ([]account.Account, error) {
	return s.store.ListAccounts(ctx)
}

// Delete removes an account by identifier.
func (s *Service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}

	if err := s.store.DeleteAccount(ctx, id); err != nil {
		return err
	}
	s.log.WithField("account_id", id).Info("account deleted")
	return nil
}
