package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/applications/storage"
)

// Base bundles shared service helpers (account validation, workspace wallets, etc).
type Base struct {
	accounts storage.AccountStore
	wallets  storage.WorkspaceWalletStore
	tracer   Tracer
}

// NewBase constructs a helper bound to the provided stores.
func NewBase(accounts storage.AccountStore) *Base {
	return &Base{accounts: accounts, tracer: NoopTracer}
}

// SetWallets wires a workspace wallet store for signer validation.
func (b *Base) SetWallets(store storage.WorkspaceWalletStore) {
	b.wallets = store
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
	_, err := b.accounts.GetAccount(ctx, accountID)
	return err
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
		return "", fmt.Errorf("account_id is required")
	}
	if b.accounts == nil {
		return trimmed, nil
	}
	if _, err := b.accounts.GetAccount(ctx, trimmed); err != nil {
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
		if _, err := b.wallets.FindWorkspaceWalletByAddress(ctx, accountID, signer); err != nil {
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
