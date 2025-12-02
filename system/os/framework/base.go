package framework

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ServiceState represents the current state of a service.
type ServiceState int32

const (
	StateUninitialized ServiceState = iota
	StateInitializing
	StateReady
	StateNotReady
	StateStopping
	StateStopped
	StateFailed
)

// String returns a human-readable state name.
func (s ServiceState) String() string {
	switch s {
	case StateUninitialized:
		return "uninitialized"
	case StateInitializing:
		return "initializing"
	case StateReady:
		return "ready"
	case StateNotReady:
		return "not-ready"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ServiceBase provides a thread-safe ready/not-ready toggle that satisfies the
// engine Ready/ReadySetter interfaces. Embed this into services to avoid
// hand-rolled readiness tracking.
type ServiceBase struct {
	state     atomic.Int32
	name      atomic.Value // string
	domain    atomic.Value // string
	startedAt atomic.Value // time.Time
	stoppedAt atomic.Value // time.Time

	mu        sync.RWMutex
	lastError error
	metadata  map[string]string
}

// NewServiceBase creates a new ServiceBase with the given name and domain.
func NewServiceBase(name, domain string) *ServiceBase {
	b := &ServiceBase{
		metadata: make(map[string]string),
	}
	b.name.Store(name)
	b.domain.Store(domain)
	return b
}

// Name returns the service name.
func (b *ServiceBase) Name() string {
	if v := b.name.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// Domain returns the service domain.
func (b *ServiceBase) Domain() string {
	if v := b.domain.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// SetName lets callers set a display name used in error messages.
func (b *ServiceBase) SetName(name string) {
	b.name.Store(strings.TrimSpace(name))
}

// SetDomain sets the service domain.
func (b *ServiceBase) SetDomain(domain string) {
	b.domain.Store(strings.TrimSpace(domain))
}

// State returns the current service state.
func (b *ServiceBase) State() ServiceState {
	return ServiceState(b.state.Load())
}

// SetState atomically sets the service state.
func (b *ServiceBase) SetState(state ServiceState) {
	b.state.Store(int32(state))
}

// CompareAndSwapState atomically sets state if current matches expected.
func (b *ServiceBase) CompareAndSwapState(expected, new ServiceState) bool {
	return b.state.CompareAndSwap(int32(expected), int32(new))
}

// SetReady marks the service as ready/not-ready (legacy interface).
func (b *ServiceBase) SetReady(status string, errMsg string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if strings.EqualFold(strings.TrimSpace(status), "ready") {
		b.state.Store(int32(StateReady))
		b.lastError = nil
	} else {
		b.state.Store(int32(StateNotReady))
		if errMsg != "" {
			b.lastError = fmt.Errorf("%s", errMsg)
		}
	}
}

// MarkReady is a helper to set readiness without an error message.
func (b *ServiceBase) MarkReady(ready bool) {
	if ready {
		b.state.Store(int32(StateReady))
	} else {
		b.state.Store(int32(StateNotReady))
	}
}

// MarkStarted records that the service has started.
func (b *ServiceBase) MarkStarted() {
	b.startedAt.Store(time.Now())
	b.state.Store(int32(StateReady))
}

// MarkStopped records that the service has stopped.
func (b *ServiceBase) MarkStopped() {
	b.stoppedAt.Store(time.Now())
	b.state.Store(int32(StateStopped))
}

// MarkFailed records that the service has failed with an error.
func (b *ServiceBase) MarkFailed(err error) {
	b.mu.Lock()
	b.lastError = err
	b.mu.Unlock()
	b.state.Store(int32(StateFailed))
}

// LastError returns the last recorded error.
func (b *ServiceBase) LastError() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastError
}

// StartedAt returns when the service started, or zero time if not started.
func (b *ServiceBase) StartedAt() time.Time {
	if v := b.startedAt.Load(); v != nil {
		return v.(time.Time)
	}
	return time.Time{}
}

// StoppedAt returns when the service stopped, or zero time if not stopped.
func (b *ServiceBase) StoppedAt() time.Time {
	if v := b.stoppedAt.Load(); v != nil {
		return v.(time.Time)
	}
	return time.Time{}
}

// Uptime returns how long the service has been running, or 0 if not started.
func (b *ServiceBase) Uptime() time.Duration {
	started := b.StartedAt()
	if started.IsZero() {
		return 0
	}
	stopped := b.StoppedAt()
	if !stopped.IsZero() {
		return stopped.Sub(started)
	}
	return time.Since(started)
}

// IsReady returns true if the service is in ready state.
func (b *ServiceBase) IsReady() bool {
	return b.State() == StateReady
}

// IsStopped returns true if the service is stopped or failed.
func (b *ServiceBase) IsStopped() bool {
	state := b.State()
	return state == StateStopped || state == StateFailed
}

// Ready reports whether the service is ready. When not ready, it returns a
// consistent error that includes the service name when available.
func (b *ServiceBase) Ready(ctx context.Context) error {
	_ = ctx
	state := b.State()
	if state == StateReady {
		return nil
	}

	name := b.Name()
	if lastErr := b.LastError(); lastErr != nil {
		if name != "" {
			return fmt.Errorf("%s: %w", name, lastErr)
		}
		return lastErr
	}

	if name != "" {
		return fmt.Errorf("%s: %s", name, state)
	}
	return fmt.Errorf("service %s", state)
}

// SetMetadata stores a key-value pair in the service metadata.
func (b *ServiceBase) SetMetadata(key, value string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.metadata == nil {
		b.metadata = make(map[string]string)
	}
	b.metadata[key] = value
}

// GetMetadata retrieves a metadata value by key.
func (b *ServiceBase) GetMetadata(key string) (string, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	v, ok := b.metadata[key]
	return v, ok
}

// AllMetadata returns a copy of all metadata.
func (b *ServiceBase) AllMetadata() map[string]string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make(map[string]string, len(b.metadata))
	for k, v := range b.metadata {
		result[k] = v
	}
	return result
}

// Start provides a default implementation that marks the service as ready.
// Services with custom startup logic should override this method.
func (b *ServiceBase) Start(ctx context.Context) error {
	_ = ctx
	b.MarkReady(true)
	return nil
}

// Stop provides a default implementation that marks the service as not ready.
// Services with custom shutdown logic should override this method.
func (b *ServiceBase) Stop(ctx context.Context) error {
	_ = ctx
	b.MarkReady(false)
	return nil
}
