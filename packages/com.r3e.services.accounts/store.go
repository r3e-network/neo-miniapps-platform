// Package accounts provides the Accounts service as a ServicePackage.
package accounts

import (
	"context"
)

// Store defines the persistence interface for accounts.
// This interface is defined within the service package, following the principle
// that "everything of the service must be in service package".
type Store interface {
	CreateAccount(ctx context.Context, acct Account) (Account, error)
	UpdateAccount(ctx context.Context, acct Account) (Account, error)
	GetAccount(ctx context.Context, id string) (Account, error)
	ListAccounts(ctx context.Context) ([]Account, error)
	DeleteAccount(ctx context.Context, id string) error

	// WorkspaceWallet methods
	CreateWorkspaceWallet(ctx context.Context, wallet WorkspaceWallet) (WorkspaceWallet, error)
	GetWorkspaceWallet(ctx context.Context, id string) (WorkspaceWallet, error)
	ListWorkspaceWallets(ctx context.Context, workspaceID string) ([]WorkspaceWallet, error)
	FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, wallet string) (WorkspaceWallet, error)
}
