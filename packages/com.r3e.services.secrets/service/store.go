// Package secrets provides the SECRETS Service as a ServicePackage.
package secrets

import (
	"context"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

// Store defines the persistence interface for secrets.
type Store interface {
	CreateSecret(ctx context.Context, sec Secret) (Secret, error)
	UpdateSecret(ctx context.Context, sec Secret) (Secret, error)
	GetSecret(ctx context.Context, accountID, name string) (Secret, error)
	ListSecrets(ctx context.Context, accountID string) ([]Secret, error)
	DeleteSecret(ctx context.Context, accountID, name string) error
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker
