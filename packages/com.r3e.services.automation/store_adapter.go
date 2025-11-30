package automation

import (
	"context"

	domainAutomation "github.com/R3E-Network/service_layer/domain/automation"
	"github.com/R3E-Network/service_layer/pkg/storage"
)

// legacyStoreAdapter adapts storage.AutomationStore to local Store interface.
type legacyStoreAdapter struct {
	store storage.AutomationStore
}

// NewLegacyStoreAdapter creates an adapter from storage.AutomationStore to local Store.
func NewLegacyStoreAdapter(store storage.AutomationStore) Store {
	return &legacyStoreAdapter{store: store}
}

func (a *legacyStoreAdapter) CreateAutomationJob(ctx context.Context, job Job) (Job, error) {
	result, err := a.store.CreateAutomationJob(ctx, toExternalJob(job))
	if err != nil {
		return Job{}, err
	}
	return fromExternalJob(result), nil
}

func (a *legacyStoreAdapter) UpdateAutomationJob(ctx context.Context, job Job) (Job, error) {
	result, err := a.store.UpdateAutomationJob(ctx, toExternalJob(job))
	if err != nil {
		return Job{}, err
	}
	return fromExternalJob(result), nil
}

func (a *legacyStoreAdapter) GetAutomationJob(ctx context.Context, id string) (Job, error) {
	result, err := a.store.GetAutomationJob(ctx, id)
	if err != nil {
		return Job{}, err
	}
	return fromExternalJob(result), nil
}

func (a *legacyStoreAdapter) ListAutomationJobs(ctx context.Context, accountID string) ([]Job, error) {
	results, err := a.store.ListAutomationJobs(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]Job, len(results))
	for i, r := range results {
		out[i] = fromExternalJob(r)
	}
	return out, nil
}

// Type conversion helpers

func toExternalJob(job Job) domainAutomation.Job {
	return domainAutomation.Job{
		ID:          job.ID,
		AccountID:   job.AccountID,
		FunctionID:  job.FunctionID,
		Name:        job.Name,
		Description: job.Description,
		Schedule:    job.Schedule,
		Status:      domainAutomation.JobStatus(job.Status),
		Enabled:     job.Enabled,
		RunCount:    job.RunCount,
		MaxRuns:     job.MaxRuns,
		LastRun:     job.LastRun,
		NextRun:     job.NextRun,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
}

func fromExternalJob(job domainAutomation.Job) Job {
	return Job{
		ID:          job.ID,
		AccountID:   job.AccountID,
		FunctionID:  job.FunctionID,
		Name:        job.Name,
		Description: job.Description,
		Schedule:    job.Schedule,
		Status:      JobStatus(job.Status),
		Enabled:     job.Enabled,
		RunCount:    job.RunCount,
		MaxRuns:     job.MaxRuns,
		LastRun:     job.LastRun,
		NextRun:     job.NextRun,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
}
