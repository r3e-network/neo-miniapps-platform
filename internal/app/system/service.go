package system

import "context"

// Service represents a lifecycle-managed component. All application modules
// must implement this interface so the system manager can start and stop them
// deterministically.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
