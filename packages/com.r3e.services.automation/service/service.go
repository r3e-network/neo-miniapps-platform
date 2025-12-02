package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
	
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Service coordinates automation jobs.
// Uses SandboxedServiceEngine for capability-based access control.
type Service struct {
	*framework.SandboxedServiceEngine // Provides: Name, Domain, Manifest, Descriptor, ValidateAccount, Logger, sandbox capabilities
	store                             Store
}

// New creates a configured automation service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "automation",
				Domain:       "automation",
				Description:  "Automation jobs and schedulers",
				DependsOn:    []string{"store", "svc-accounts", "svc-functions"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore, framework.APISurfaceEvent, framework.APISurfaceCompute},
				Capabilities: []string{"automation"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
			},
			StorageQuota: 10 * 1024 * 1024, // 10MB
		}),
		store: store,
	}
}

// CreateJob provisions a new automation job tied to a function.
func (s *Service) CreateJob(ctx context.Context, accountID, functionID, name, schedule, description string) (job Job, err error) {
	accountID = strings.TrimSpace(accountID)
	functionID = strings.TrimSpace(functionID)
	name = strings.TrimSpace(name)
	schedule = strings.TrimSpace(schedule)
	attrs := map[string]string{"account_id": accountID, "resource": "automation_job"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()

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

	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
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

	job = Job{
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
	attrs["job_id"] = job.ID
	s.Logger().WithField("job_id", job.ID).
		WithField("account_id", accountID).
		WithField("function_id", job.FunctionID).
		Info("automation job created")
	s.LogCreated("automation_job", job.ID, job.AccountID)
	s.IncrementCounter("automation_jobs_created_total", map[string]string{"account_id": accountID})
	return job, nil
}

// UpdateJob applies modifications to an automation job.
func (s *Service) UpdateJob(ctx context.Context, jobID string, name, schedule, description *string, nextRun *time.Time) (job Job, err error) {
	attrs := map[string]string{"job_id": jobID, "resource": "automation_job"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()
	job, err = s.store.GetAutomationJob(ctx, jobID)
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
	attrs["account_id"] = job.AccountID
	s.Logger().WithField("job_id", job.ID).
		WithField("account_id", job.AccountID).
		Info("automation job updated")
	s.LogUpdated("automation_job", job.ID, job.AccountID)
	s.IncrementCounter("automation_jobs_updated_total", map[string]string{"account_id": job.AccountID})
	return job, nil
}

// SetEnabled toggles a job's enabled flag.
func (s *Service) SetEnabled(ctx context.Context, jobID string, enabled bool) (job Job, err error) {
	attrs := map[string]string{"job_id": jobID, "resource": "automation_job_enable"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()
	job, err = s.store.GetAutomationJob(ctx, jobID)
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
	attrs["account_id"] = job.AccountID
	s.Logger().WithField("job_id", job.ID).
		WithField("account_id", job.AccountID).
		WithField("enabled", enabled).
		Info("automation job state changed")
	action := "disabled"
	if enabled {
		action = "enabled"
	}
	s.LogAction("job_"+action, "automation_job", job.ID, job.AccountID)
	value := 0.0
	if enabled {
		value = 1
	}
	s.SetGauge("automation_job_enabled", map[string]string{"job_id": job.ID, "account_id": job.AccountID}, value)
	return job, nil
}

// RecordExecution stores execution metadata for a job.
func (s *Service) RecordExecution(ctx context.Context, jobID string, runAt time.Time) (job Job, err error) {
	attrs := map[string]string{"job_id": jobID, "resource": "automation_job_run"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()
	job, err = s.store.GetAutomationJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}

	job.LastRun = runAt.UTC()
	if err := s.applyNextRun(&job, job.LastRun); err != nil {
		s.Logger().WithError(err).
			WithField("job_id", job.ID).
			Warn("failed to compute next run; clearing value")
		job.NextRun = time.Time{}
	}
	attrs["account_id"] = job.AccountID
	s.IncrementCounter("automation_job_runs_total", map[string]string{"job_id": job.ID, "account_id": job.AccountID})
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
	if err := s.ValidateAccountExists(ctx, trimmed); err != nil {
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

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.
// Method signature: func (s *Service) HTTP{Method}{Path}(ctx context.Context, req core.APIRequest) (any, error)

// HTTPGetJobs handles GET /jobs - list all automation jobs for an account.
func (s *Service) HTTPGetJobs(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListJobs(ctx, req.AccountID)
}

// HTTPPostJobs handles POST /jobs - create a new automation job.
func (s *Service) HTTPPostJobs(ctx context.Context, req core.APIRequest) (any, error) {
	functionID, _ := req.Body["function_id"].(string)
	name, _ := req.Body["name"].(string)
	schedule, _ := req.Body["schedule"].(string)
	description, _ := req.Body["description"].(string)
	return s.CreateJob(ctx, req.AccountID, functionID, name, schedule, description)
}

// HTTPGetJobsById handles GET /jobs/{id} - get a specific job.
func (s *Service) HTTPGetJobsById(ctx context.Context, req core.APIRequest) (any, error) {
	jobID := req.PathParams["id"]
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	// Verify ownership
	if job.AccountID != req.AccountID {
		return nil, fmt.Errorf("forbidden: job belongs to different account")
	}
	return job, nil
}

// HTTPPatchJobsById handles PATCH /jobs/{id} - update a job.
func (s *Service) HTTPPatchJobsById(ctx context.Context, req core.APIRequest) (any, error) {
	jobID := req.PathParams["id"]

	// Verify ownership first
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.AccountID != req.AccountID {
		return nil, fmt.Errorf("forbidden: job belongs to different account")
	}

	// Extract optional update fields
	var name, schedule, description *string
	if v, ok := req.Body["name"].(string); ok {
		name = &v
	}
	if v, ok := req.Body["schedule"].(string); ok {
		schedule = &v
	}
	if v, ok := req.Body["description"].(string); ok {
		description = &v
	}

	// Handle enabled toggle separately
	if enabled, ok := req.Body["enabled"].(bool); ok {
		return s.SetEnabled(ctx, jobID, enabled)
	}

	return s.UpdateJob(ctx, jobID, name, schedule, description, nil)
}

// HTTPDeleteJobsById handles DELETE /jobs/{id} - delete a job (if supported).
func (s *Service) HTTPDeleteJobsById(ctx context.Context, req core.APIRequest) (any, error) {
	jobID := req.PathParams["id"]

	// Verify ownership first
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.AccountID != req.AccountID {
		return nil, fmt.Errorf("forbidden: job belongs to different account")
	}

	// Disable the job (soft delete)
	return s.SetEnabled(ctx, jobID, false)
}
