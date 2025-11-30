// Package functions provides the Functions service as a ServicePackage.
package functions

import (
	"context"

	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the persistence interface for function definitions and executions.
// This interface is defined within the service package, following the principle
// that "everything of the service must be in service package".
type Store interface {
	// Function definition operations
	CreateFunction(ctx context.Context, def Definition) (Definition, error)
	UpdateFunction(ctx context.Context, def Definition) (Definition, error)
	GetFunction(ctx context.Context, id string) (Definition, error)
	ListFunctions(ctx context.Context, accountID string) ([]Definition, error)

	// Function execution operations
	CreateExecution(ctx context.Context, exec Execution) (Execution, error)
	GetExecution(ctx context.Context, id string) (Execution, error)
	ListFunctionExecutions(ctx context.Context, functionID string, limit int) ([]Execution, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
// Use framework.AccountChecker directly in new code.
type AccountChecker = framework.AccountChecker
