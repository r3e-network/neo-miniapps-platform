package functions

import (
	"context"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages lifecycle operations for function definitions. Execution is
// delegated to runtime-specific components; this service focuses on metadata
// management and validation.
type Service struct {
	accounts storage.AccountStore
	store    storage.FunctionStore
	log      *logger.Logger
	nowFn    func() time.Time
}

func NewService(accounts storage.AccountStore, store storage.FunctionStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("functions")
	}
	return &Service{
		accounts: accounts,
		store:    store,
		log:      log,
		nowFn:    time.Now,
	}
}

// Create registers a new function.
func (s *Service) Create(ctx context.Context, def function.Definition) (function.Definition, error) {
	if def.AccountID == "" {
		return function.Definition{}, fmt.Errorf("account_id is required")
	}
	if def.Name == "" {
		return function.Definition{}, fmt.Errorf("name is required")
	}
	if def.Source == "" {
		return function.Definition{}, fmt.Errorf("source is required")
	}

	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, def.AccountID); err != nil {
			return function.Definition{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	created, err := s.store.CreateFunction(ctx, def)
	if err != nil {
		return function.Definition{}, err
	}

	s.log.Infof("function %s created for account %s", created.ID, created.AccountID)
	return created, nil
}

// Update modifies descriptive attributes of a function definition.
func (s *Service) Update(ctx context.Context, def function.Definition) (function.Definition, error) {
	existing, err := s.store.GetFunction(ctx, def.ID)
	if err != nil {
		return function.Definition{}, err
	}

	if def.Name == "" {
		def.Name = existing.Name
	}
	if def.Description == "" {
		def.Description = existing.Description
	}
	if def.Source == "" {
		def.Source = existing.Source
	}
	if len(def.Secrets) == 0 {
		def.Secrets = existing.Secrets
	}
	def.AccountID = existing.AccountID

	updated, err := s.store.UpdateFunction(ctx, def)
	if err != nil {
		return function.Definition{}, err
	}

	s.log.Infof("function %s updated", def.ID)
	return updated, nil
}

// Get returns a function definition by identifier.
func (s *Service) Get(ctx context.Context, id string) (function.Definition, error) {
	return s.store.GetFunction(ctx, id)
}

// List returns all functions for the specified account.
func (s *Service) List(ctx context.Context, accountID string) ([]function.Definition, error) {
	return s.store.ListFunctions(ctx, accountID)
}
