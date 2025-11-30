package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Service coordinates automation jobs.
type Service struct {
	framework.ServiceBase
	accounts AccountChecker
	store    Store
	log      *logger.Logger
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "automation" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "automation" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Automation jobs and schedulers",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts", "svc-functions"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceEvent, engine.APISurfaceCompute},
		Capabilities: []string{"automation"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// New creates a configured automation service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("automation")
	}
	svc := &Service{
		accounts: accounts,
		store:    store,
		log:      log,
	}
	svc.SetName(svc.Name())
	return svc
}

// Start/Stop/Ready are inherited from framework.ServiceBase.

// CreateJob provisions a new automation job tied to a function.
func (s *Service) CreateJob(ctx context.Context, accountID, functionID, name, schedule, description string) (Job, error) {
	accountID = strings.TrimSpace(accountID)
	functionID = strings.TrimSpace(functionID)
	name = strings.TrimSpace(name)
	schedule = strings.TrimSpace(schedule)

	if accountID == "" {
		return Job{}, core.RequiredError("account_id")
	}
	if functionID == "" {
		return Job{}, core.RequiredError("function_id")
	}
	if name == "" {
		return Job{}, core.RequiredError("name")
	}
	if schedule == "" {
		return Job{}, core.RequiredError("schedule")
	}

	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return Job{}, fmt.Errorf("account validation failed: %w", err)
	}
	// Note: Function validation is handled by the caller (application layer)
	// since we don't have direct access to the functions store here.

	existing, err := s.store.ListAutomationJobs(ctx, accountID)
	if err != nil {
		return Job{}, err
	}
	for _, job := range existing {
		if strings.EqualFold(job.Name, name) {
			return Job{}, fmt.Errorf("job with name %q already exists", name)
		}
	}

	job := Job{
		AccountID:   accountID,
		FunctionID:  functionID,
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Enabled:     true,
	}
	if err := s.applyNextRun(&job, time.Now().UTC()); err != nil {
		return Job{}, err
	}
	job, err = s.store.CreateAutomationJob(ctx, job)
	if err != nil {
		return Job{}, err
	}
	s.log.WithField("job_id", job.ID).
		WithField("account_id", accountID).
		WithField("function_id", job.FunctionID).
		Info("automation job created")
	return job, nil
}

// UpdateJob applies modifications to an automation job.
func (s *Service) UpdateJob(ctx context.Context, jobID string, name, schedule, description *string, nextRun *time.Time) (Job, error) {
	job, err := s.store.GetAutomationJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return Job{}, fmt.Errorf("name cannot be empty")
		}
		existing, err := s.store.ListAutomationJobs(ctx, job.AccountID)
		if err != nil {
			return Job{}, err
		}
		for _, other := range existing {
			if other.ID != job.ID && strings.EqualFold(other.Name, trimmed) {
				return Job{}, fmt.Errorf("job with name %q already exists", trimmed)
			}
		}
		job.Name = trimmed
	}
	if schedule != nil {
		trimmed := strings.TrimSpace(*schedule)
		if trimmed == "" {
			return Job{}, fmt.Errorf("schedule cannot be empty")
		}
		job.Schedule = trimmed
	}
	if description != nil {
		job.Description = strings.TrimSpace(*description)
	}
	if nextRun != nil {
		job.NextRun = nextRun.UTC()
	} else if schedule != nil {
		if err := s.applyNextRun(&job, time.Now().UTC()); err != nil {
			return Job{}, err
		}
	}

	job, err = s.store.UpdateAutomationJob(ctx, job)
	if err != nil {
		return Job{}, err
	}
	s.log.WithField("job_id", job.ID).
		WithField("account_id", job.AccountID).
		Info("automation job updated")
	return job, nil
}

// SetEnabled toggles a job's enabled flag.
func (s *Service) SetEnabled(ctx context.Context, jobID string, enabled bool) (Job, error) {
	job, err := s.store.GetAutomationJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Enabled == enabled {
		return job, nil
	}
	job.Enabled = enabled
	job, err = s.store.UpdateAutomationJob(ctx, job)
	if err != nil {
		return Job{}, err
	}
	s.log.WithField("job_id", job.ID).
		WithField("account_id", job.AccountID).
		WithField("enabled", enabled).
		Info("automation job state changed")
	return job, nil
}

// RecordExecution stores execution metadata for a job.
func (s *Service) RecordExecution(ctx context.Context, jobID string, runAt time.Time) (Job, error) {
	job, err := s.store.GetAutomationJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}

	job.LastRun = runAt.UTC()
	if err := s.applyNextRun(&job, job.LastRun); err != nil {
		s.log.WithError(err).
			WithField("job_id", job.ID).
			Warn("failed to compute next run; clearing value")
		job.NextRun = time.Time{}
	}

	return s.store.UpdateAutomationJob(ctx, job)
}

// GetJob fetches a job by identifier.
func (s *Service) GetJob(ctx context.Context, jobID string) (Job, error) {
	return s.store.GetAutomationJob(ctx, jobID)
}

// ListJobs lists jobs for an account.
func (s *Service) ListJobs(ctx context.Context, accountID string) ([]Job, error) {
	trimmed := strings.TrimSpace(accountID)
	if trimmed == "" {
		if accountID == "" {
			return s.store.ListAutomationJobs(ctx, "")
		}
		return nil, core.RequiredError("account_id")
	}
	if err := s.accounts.AccountExists(ctx, trimmed); err != nil {
		return nil, err
	}
	return s.store.ListAutomationJobs(ctx, trimmed)
}

func (s *Service) applyNextRun(job *Job, from time.Time) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}
	next, err := nextRunFromSpec(job.Schedule, from)
	if err != nil {
		return err
	}
	job.NextRun = next
	return nil
}
