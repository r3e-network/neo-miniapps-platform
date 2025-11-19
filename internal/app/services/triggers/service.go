package triggers

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages trigger records and validation.
type Service struct {
	base      *core.Base
	functions storage.FunctionStore
	store     storage.TriggerStore
	log       *logger.Logger
}

// New constructs a trigger service.
func New(accounts storage.AccountStore, functions storage.FunctionStore, store storage.TriggerStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("triggers")
	}
	return &Service{base: core.NewBase(accounts), functions: functions, store: store, log: log}
}

// Register creates a trigger after validating dependencies.
func (s *Service) Register(ctx context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	if err := s.base.EnsureAccount(ctx, trg.AccountID); err != nil {
		return trigger.Trigger{}, err
	}
	if strings.TrimSpace(trg.FunctionID) == "" {
		return trigger.Trigger{}, fmt.Errorf("function_id is required")
	}
	if s.functions != nil {
		fn, err := s.functions.GetFunction(ctx, trg.FunctionID)
		if err != nil {
			return trigger.Trigger{}, fmt.Errorf("function validation failed: %w", err)
		}
		if fn.AccountID != trg.AccountID {
			return trigger.Trigger{}, fmt.Errorf("function %s does not belong to account %s", trg.FunctionID, trg.AccountID)
		}
	}

	if err := s.validateAndNormalize(&trg); err != nil {
		return trigger.Trigger{}, err
	}

	created, err := s.store.CreateTrigger(ctx, trg)
	if err != nil {
		return trigger.Trigger{}, err
	}
	s.log.WithField("trigger_id", created.ID).
		WithField("account_id", created.AccountID).
		WithField("function_id", created.FunctionID).
		Info("trigger registered")
	return created, nil
}

// SetEnabled toggles a trigger.
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
	s.log.WithField("trigger_id", id).
		WithField("account_id", trg.AccountID).
		WithField("enabled", enabled).
		Info("trigger state changed")
	return updated, nil
}

// List lists triggers for an account.
func (s *Service) List(ctx context.Context, accountID string) ([]trigger.Trigger, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListTriggers(ctx, accountID)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "triggers",
		Domain:       "triggers",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"triggers", "schedules"},
	}
}

func (s *Service) validateAndNormalize(trg *trigger.Trigger) error {
	trg.Rule = strings.TrimSpace(trg.Rule)
	trg.Config = normalizeConfig(trg.Config)
	if trg.Type == "" {
		trg.Type = trigger.TypeCron
	}
	trg.Type = trigger.Type(strings.ToLower(string(trg.Type)))

	switch trg.Type {
	case trigger.TypeCron:
		if trg.Rule == "" {
			return fmt.Errorf("rule is required for cron trigger")
		}
		trg.Config = nil
	case trigger.TypeEvent:
		if trg.Rule == "" {
			return fmt.Errorf("event name is required for event trigger")
		}
	case trigger.TypeWebhook:
		if trg.Config == nil {
			trg.Config = make(map[string]string)
		}
		url := strings.TrimSpace(trg.Config["url"])
		if url == "" {
			return fmt.Errorf("config.url is required for webhook trigger")
		}
		trg.Config["url"] = url
		method := strings.TrimSpace(trg.Config["method"])
		if method == "" {
			method = "POST"
		}
		trg.Config["method"] = strings.ToUpper(method)
	default:
		return fmt.Errorf("unsupported trigger type %q", trg.Type)
	}

	return nil
}

func normalizeConfig(cfg map[string]string) map[string]string {
	if len(cfg) == 0 {
		return nil
	}
	out := make(map[string]string, len(cfg))
	for k, v := range cfg {
		out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return out
}
