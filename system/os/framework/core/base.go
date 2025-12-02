package service

import (
	"context"
	"fmt"
	"strings"
)

// AccountChecker is the standard interface for account validation.
// Services should depend on this interface rather than concrete store types.
// This is the canonical interface - use it consistently across all services.
//
// All service packages should import this interface from framework rather than
// defining their own. This eliminates ~65 lines of duplicate interface definitions.
type AccountChecker interface {
	// AccountExists checks if an account exists by ID.
	// Returns nil if the account exists, or an error if not found.
	AccountExists(ctx context.Context, accountID string) error

	// AccountTenant returns the tenant identifier for an account.
	// Returns empty string if the account has no tenant or doesn't exist.
	// This supports multi-tenancy filtering in service stores.
	AccountTenant(ctx context.Context, accountID string) string
}

// BasicAccountChecker is a minimal interface for services that only need existence checks.
// Use AccountChecker for full functionality including tenant support.
type BasicAccountChecker interface {
	AccountExists(ctx context.Context, accountID string) error
}

// WalletChecker is the standard interface for wallet ownership validation.
type WalletChecker interface {
	WalletOwnedBy(ctx context.Context, accountID, wallet string) error
}


// AccountStoreAdapter wraps any account store to implement AccountChecker.
// Optionally supports tenant lookup via TenantFunc.
type AccountStoreAdapter[T any] struct {
	Store interface {
		GetAccount(ctx context.Context, id string) (T, error)
	}
	// TenantFunc is optional. When set, enables AccountTenant() support.
	TenantFunc func(ctx context.Context, id string) string
}

// AccountExists checks if an account exists.
func (a AccountStoreAdapter[T]) AccountExists(ctx context.Context, id string) error {
	_, err := a.Store.GetAccount(ctx, id)
	return err
}

// AccountTenant returns the tenant for an account.
// Returns empty string if TenantFunc is not configured.
func (a AccountStoreAdapter[T]) AccountTenant(ctx context.Context, id string) string {
	if a.TenantFunc == nil {
		return ""
	}
	return a.TenantFunc(ctx, id)
}


// WalletStoreAdapter wraps any wallet store to implement WalletChecker.
type WalletStoreAdapter[T any] struct {
	Store interface {
		FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, wallet string) (T, error)
	}
}

// WalletOwnedBy checks if a wallet belongs to the account.
func (a WalletStoreAdapter[T]) WalletOwnedBy(ctx context.Context, accountID, wallet string) error {
	_, err := a.Store.FindWorkspaceWalletByAddress(ctx, accountID, wallet)
	return err
}


// Base bundles shared service helpers (account validation, workspace wallets, etc).
type Base struct {
	accounts AccountChecker
	wallets  WalletChecker
	tracer   Tracer
}

// NewBase constructs a helper bound to the provided account checker.
func NewBase(accounts AccountChecker) *Base {
	return &Base{accounts: accounts, tracer: NoopTracer}
}

// NewBaseFromStore constructs a helper from a typed account store.
func NewBaseFromStore[T any](store interface {
	GetAccount(ctx context.Context, id string) (T, error)
}) *Base {
	return &Base{accounts: AccountStoreAdapter[T]{Store: store}, tracer: NoopTracer}
}

// SetWallets wires a workspace wallet store for signer validation.
func (b *Base) SetWallets(store WalletChecker) {
	b.wallets = store
}

// WrapWalletStore wraps a typed wallet store to implement WalletChecker.
func WrapWalletStore[T any](store interface {
	FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, wallet string) (T, error)
}) WalletChecker {
	return WalletStoreAdapter[T]{Store: store}
}

// SetTracer configures the tracer used for cross-cutting spans.
func (b *Base) SetTracer(tracer Tracer) {
	if tracer == nil {
		b.tracer = NoopTracer
		return
	}
	b.tracer = tracer
}

// EnsureAccount validates presence and optional existence of an account ID.
func (b *Base) EnsureAccount(ctx context.Context, accountID string) error {
	if strings.TrimSpace(accountID) == "" {
		return RequiredError("account_id")
	}
	if b.accounts == nil {
		return nil
	}
	return b.accounts.AccountExists(ctx, accountID)
}

// ValidateAccount validates an account ID and wraps errors with context.
// This is a convenience method that provides consistent error wrapping.
func (b *Base) ValidateAccount(ctx context.Context, accountID string) error {
	if err := b.EnsureAccount(ctx, accountID); err != nil {
		return fmt.Errorf("account validation failed: %w", err)
	}
	return nil
}

// EnsureResourceOwnership validates that a resource belongs to the specified account.
// This is a convenience method combining account validation and ownership check.
func (b *Base) EnsureResourceOwnership(ctx context.Context, resourceAccountID, requestAccountID, resourceType, resourceID string) error {
	if err := b.EnsureAccount(ctx, requestAccountID); err != nil {
		return err
	}
	return EnsureOwnership(resourceAccountID, requestAccountID, resourceType, resourceID)
}

// NormalizeAccount trims and validates an account identifier. It returns the
// trimmed ID after ensuring existence (when an account store is configured).
func (b *Base) NormalizeAccount(ctx context.Context, accountID string) (string, error) {
	trimmed := strings.TrimSpace(accountID)
	if trimmed == "" {
		return "", RequiredError("account_id")
	}
	if b.accounts == nil {
		return trimmed, nil
	}
	if err := b.accounts.AccountExists(ctx, trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

// EnsureSignersOwned verifies that each signer belongs to the workspace.
func (b *Base) EnsureSignersOwned(ctx context.Context, accountID string, signers []string) error {
	if len(signers) == 0 || b.wallets == nil {
		return nil
	}
	for _, signer := range signers {
		if err := b.wallets.WalletOwnedBy(ctx, accountID, signer); err != nil {
			return fmt.Errorf("signer %s not registered for account %s", signer, accountID)
		}
	}
	return nil
}

// Tracer exposes the currently configured tracer (defaults to no-op).
func (b *Base) Tracer() Tracer {
	if b == nil || b.tracer == nil {
		return NoopTracer
	}
	return b.tracer
}
