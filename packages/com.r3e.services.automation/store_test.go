package automation

import (
	"context"

	"github.com/R3E-Network/service_layer/pkg/storage"
)

// memoryStoreAdapter adapts pkg/storage/memory.Store to local Store interface for testing.
type memoryStoreAdapter struct {
	store storage.AutomationStore
}

// NewMemoryStoreAdapter creates an adapter for testing with memory stores.
func NewMemoryStoreAdapter(store storage.AutomationStore) Store {
	return &memoryStoreAdapter{store: store}
}

func (a *memoryStoreAdapter) CreateAutomationJob(ctx context.Context, job Job) (Job, error) {
	result, err := a.store.CreateAutomationJob(ctx, toExternalJob(job))
	if err != nil {
		return Job{}, err
	}
	return fromExternalJob(result), nil
}

func (a *memoryStoreAdapter) UpdateAutomationJob(ctx context.Context, job Job) (Job, error) {
	result, err := a.store.UpdateAutomationJob(ctx, toExternalJob(job))
	if err != nil {
		return Job{}, err
	}
	return fromExternalJob(result), nil
}

func (a *memoryStoreAdapter) GetAutomationJob(ctx context.Context, id string) (Job, error) {
	result, err := a.store.GetAutomationJob(ctx, id)
	if err != nil {
		return Job{}, err
	}
	return fromExternalJob(result), nil
}

func (a *memoryStoreAdapter) ListAutomationJobs(ctx context.Context, accountID string) ([]Job, error) {
	results, err := a.store.ListAutomationJobs(ctx, accountID)
	if err != nil {
		return nil, err
	}
	jobs := make([]Job, len(results))
	for i, r := range results {
		jobs[i] = fromExternalJob(r)
	}
	return jobs, nil
}

// Type conversion helpers are now in store_adapter.go
