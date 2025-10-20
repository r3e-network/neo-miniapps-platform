package triggers

import (
	"context"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service coordinates triggers tied to function definitions.
type Service struct {
	accounts  storage.AccountStore
	functions storage.FunctionStore
	store     storage.TriggerStore
	log       *logger.Logger
}

func NewService(accounts storage.AccountStore, functions storage.FunctionStore, store storage.TriggerStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("triggers")
	}
	return &Service{
		accounts:  accounts,
		functions: functions,
		store:     store,
		log:       log,
	}
}

// Register creates a new trigger ensuring the account and function exist.
func (s *Service) Register(ctx context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	if trg.AccountID == "" || trg.FunctionID == "" {
		return trigger.Trigger{}, fmt.Errorf("account_id and function_id are required")
	}

	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, trg.AccountID); err != nil {
			return trigger.Trigger{}, fmt.Errorf("account validation failed: %w", err)
		}
	}
	if s.functions != nil {
		if _, err := s.functions.GetFunction(ctx, trg.FunctionID); err != nil {
			return trigger.Trigger{}, fmt.Errorf("function validation failed: %w", err)
		}
	}

	trg.Enabled = true
	created, err := s.store.CreateTrigger(ctx, trg)
	if err != nil {
		return trigger.Trigger{}, err
	}

	s.log.Infof("trigger %s registered for function %s", created.ID, created.FunctionID)
	return created, nil
}

// SetEnabled toggles the enabled flag.
func (s *Service) SetEnabled(ctx context.Context, id string, enabled bool) (trigger.Trigger, error) {
	trg, err := s.store.GetTrigger(ctx, id)
	if err != nil {
		return trigger.Trigger{}, err
	}
	trg.Enabled = enabled
	trg.UpdatedAt = time.Now().UTC()

	updated, err := s.store.UpdateTrigger(ctx, trg)
	if err != nil {
		return trigger.Trigger{}, err
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}
	s.log.Infof("trigger %s %s", id, state)
	return updated, nil
}

// List returns triggers scoped to an account.
func (s *Service) List(ctx context.Context, accountID string) ([]trigger.Trigger, error) {
	return s.store.ListTriggers(ctx, accountID)
}
