package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service coordinates automation jobs.
type Service struct {
	accounts  storage.AccountStore
	functions storage.FunctionStore
	store     storage.AutomationStore
	log       *logger.Logger
}

// New creates a configured automation service.
func New(accounts storage.AccountStore, functions storage.FunctionStore, store storage.AutomationStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("automation")
	}
	return &Service{
		accounts:  accounts,
		functions: functions,
		store:     store,
		log:       log,
	}
}

// CreateJob provisions a new automation job tied to a function.
func (s *Service) CreateJob(ctx context.Context, accountID, functionID, name, schedule, description string) (automation.Job, error) {
	accountID = strings.TrimSpace(accountID)
	functionID = strings.TrimSpace(functionID)
	name = strings.TrimSpace(name)
	schedule = strings.TrimSpace(schedule)

	if accountID == "" {
		return automation.Job{}, fmt.Errorf("account_id is required")
	}
	if functionID == "" {
		return automation.Job{}, fmt.Errorf("function_id is required")
	}
	if name == "" {
		return automation.Job{}, fmt.Errorf("name is required")
	}
	if schedule == "" {
		return automation.Job{}, fmt.Errorf("schedule is required")
	}

	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, accountID); err != nil {
			return automation.Job{}, fmt.Errorf("account validation failed: %w", err)
		}
	}
	if s.functions != nil {
		if _, err := s.functions.GetFunction(ctx, functionID); err != nil {
			return automation.Job{}, fmt.Errorf("function validation failed: %w", err)
		}
	}

	existing, err := s.store.ListAutomationJobs(ctx, accountID)
	if err != nil {
		return automation.Job{}, err
	}
	for _, job := range existing {
		if strings.EqualFold(job.Name, name) {
			return automation.Job{}, fmt.Errorf("job with name %q already exists", name)
		}
	}

	job := automation.Job{
		AccountID:   accountID,
		FunctionID:  functionID,
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Enabled:     true,
	}
	job, err = s.store.CreateAutomationJob(ctx, job)
	if err != nil {
		return automation.Job{}, err
	}
	s.log.WithField("job_id", job.ID).
		WithField("account_id", accountID).
		WithField("function_id", job.FunctionID).
		Info("automation job created")
	return job, nil
}

// UpdateJob applies modifications to an automation job.
func (s *Service) UpdateJob(ctx context.Context, jobID string, name, schedule, description *string, nextRun *time.Time) (automation.Job, error) {
	job, err := s.store.GetAutomationJob(ctx, jobID)
	if err != nil {
		return automation.Job{}, err
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return automation.Job{}, fmt.Errorf("name cannot be empty")
		}
		existing, err := s.store.ListAutomationJobs(ctx, job.AccountID)
		if err != nil {
			return automation.Job{}, err
		}
		for _, other := range existing {
			if other.ID != job.ID && strings.EqualFold(other.Name, trimmed) {
				return automation.Job{}, fmt.Errorf("job with name %q already exists", trimmed)
			}
		}
		job.Name = trimmed
	}
	if schedule != nil {
		trimmed := strings.TrimSpace(*schedule)
		if trimmed == "" {
			return automation.Job{}, fmt.Errorf("schedule cannot be empty")
		}
		job.Schedule = trimmed
	}
	if description != nil {
		job.Description = strings.TrimSpace(*description)
	}
	if nextRun != nil {
		job.NextRun = nextRun.UTC()
	}

	job, err = s.store.UpdateAutomationJob(ctx, job)
	if err != nil {
		return automation.Job{}, err
	}
	s.log.WithField("job_id", job.ID).
		WithField("account_id", job.AccountID).
		Info("automation job updated")
	return job, nil
}

// SetEnabled toggles a job's enabled flag.
func (s *Service) SetEnabled(ctx context.Context, jobID string, enabled bool) (automation.Job, error) {
	job, err := s.store.GetAutomationJob(ctx, jobID)
	if err != nil {
		return automation.Job{}, err
	}
	if job.Enabled == enabled {
		return job, nil
	}
	job.Enabled = enabled
	job, err = s.store.UpdateAutomationJob(ctx, job)
	if err != nil {
		return automation.Job{}, err
	}
	s.log.WithField("job_id", job.ID).
		WithField("account_id", job.AccountID).
		WithField("enabled", enabled).
		Info("automation job state changed")
	return job, nil
}

// RecordExecution stores execution metadata for a job.
func (s *Service) RecordExecution(ctx context.Context, jobID string, runAt time.Time, next time.Time) (automation.Job, error) {
	job, err := s.store.GetAutomationJob(ctx, jobID)
	if err != nil {
		return automation.Job{}, err
	}

	job.LastRun = runAt.UTC()
	job.NextRun = next.UTC()

	return s.store.UpdateAutomationJob(ctx, job)
}

// GetJob fetches a job by identifier.
func (s *Service) GetJob(ctx context.Context, jobID string) (automation.Job, error) {
	return s.store.GetAutomationJob(ctx, jobID)
}

// ListJobs lists jobs for an account.
func (s *Service) ListJobs(ctx context.Context, accountID string) ([]automation.Job, error) {
	return s.store.ListAutomationJobs(ctx, accountID)
}
