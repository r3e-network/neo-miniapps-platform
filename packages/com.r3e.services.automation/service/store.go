package automation

import (
	"context"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

// Store defines the persistence interface for automation jobs using local types.
type Store interface {
	CreateAutomationJob(ctx context.Context, job Job) (Job, error)
	UpdateAutomationJob(ctx context.Context, job Job) (Job, error)
	GetAutomationJob(ctx context.Context, id string) (Job, error)
	ListAutomationJobs(ctx context.Context, accountID string) ([]Job, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker
