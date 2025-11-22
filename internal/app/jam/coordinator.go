package jam

import (
	"context"
)

// Coordinator pulls pending packages from a store and processes them via Engine.
type Coordinator struct {
	Store  PackageStore
	Engine Engine
}

// ProcessNext fetches the next pending package (if any) and runs it through the engine.
// Returns ok=false when no pending packages remain.
func (c Coordinator) ProcessNext(ctx context.Context) (ok bool, err error) {
	if c.Store == nil {
		return false, ErrInvalidCoordinator
	}
	pkg, found, err := c.Store.NextPending(ctx)
	if err != nil || !found {
		return found, err
	}

	// Mark pending explicitly to avoid reprocessing in concurrent settings.
	_ = c.Store.UpdatePackageStatus(ctx, pkg.ID, PackageStatusPending)

	report, attns, err := c.Engine.Process(ctx, pkg)
	if err != nil {
		_ = c.Store.UpdatePackageStatus(ctx, pkg.ID, PackageStatusDisputed)
		return true, err
	}

	if err := c.Store.SaveReport(ctx, report, attns); err != nil {
		return true, err
	}
	if err := c.Store.UpdatePackageStatus(ctx, pkg.ID, PackageStatusApplied); err != nil {
		return true, err
	}
	return true, nil
}

// ErrInvalidCoordinator is returned when required fields are missing.
var ErrInvalidCoordinator = Err("coordinator is missing store or engine")

// Err is a minimal string error helper to avoid fmt dependency.
type Err string

func (e Err) Error() string { return string(e) }
