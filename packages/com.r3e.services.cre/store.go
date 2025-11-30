// Package cre provides the CRE Service as a ServicePackage.
package cre

import (
	"context"

	
	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the persistence interface for cre.
// This interface is defined within the service package, following the principle
// that "everything of the service must be in service package".
type Store interface {
	CreatePlaybook(ctx context.Context, pb Playbook) (Playbook, error)
	UpdatePlaybook(ctx context.Context, pb Playbook) (Playbook, error)
	GetPlaybook(ctx context.Context, id string) (Playbook, error)
	ListPlaybooks(ctx context.Context, accountID string) ([]Playbook, error)

	CreateRun(ctx context.Context, run Run) (Run, error)
	UpdateRun(ctx context.Context, run Run) (Run, error)
	GetRun(ctx context.Context, id string) (Run, error)
	ListRuns(ctx context.Context, accountID string, limit int) ([]Run, error)

	CreateExecutor(ctx context.Context, exec Executor) (Executor, error)
	UpdateExecutor(ctx context.Context, exec Executor) (Executor, error)
	GetExecutor(ctx context.Context, id string) (Executor, error)
	ListExecutors(ctx context.Context, accountID string) ([]Executor, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker
