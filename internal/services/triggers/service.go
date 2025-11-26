package triggers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/internal/domain/trigger"
	engine "github.com/R3E-Network/service_layer/internal/engine"
	"github.com/R3E-Network/service_layer/internal/framework"
	core "github.com/R3E-Network/service_layer/internal/services/core"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages trigger records and validation.
type Service struct {
	framework.ServiceBase
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
	svc := &Service{base: core.NewBase(accounts), functions: functions, store: store, log: log}
	svc.SetName(svc.Name())
	return svc
}

// Name returns the stable service identifier.
func (s *Service) Name() string { return "triggers" }

// Domain reports the service domain.
func (s *Service) Domain() string { return "triggers" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Triggers that route events/webhooks to functions",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts", "svc-functions"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceEvent},
		Capabilities: []string{"triggers"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Layer:        core.LayerService,
		Capabilities: []string{"triggers"},
		DependsOn:    []string{"store", "svc-accounts", "svc-functions"},
		RequiresAPIs: []string{string(engine.APISurfaceStore), string(engine.APISurfaceEvent)},
	}
}

// Start marks readiness.
func (s *Service) Start(ctx context.Context) error { _ = ctx; s.MarkReady(true); return nil }

// Stop clears readiness.
func (s *Service) Stop(ctx context.Context) error { _ = ctx; s.MarkReady(false); return nil }

// Ready reports readiness for engine probes.
func (s *Service) Ready(ctx context.Context) error {
	return s.ServiceBase.Ready(ctx)
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
