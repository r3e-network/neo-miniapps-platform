// Package bus provides bus operation utilities including concurrency limiting.
// The Limiter enforces MaxConcurrency settings for bus operations to prevent
// resource exhaustion and ensure fair scheduling across modules.
package bus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Common errors
var (
	ErrLimitExceeded = errors.New("concurrency limit exceeded")
	ErrAcquireTimeout = errors.New("acquire timeout")
	ErrLimiterClosed = errors.New("limiter is closed")
)

// Kind represents the type of bus operation.
type Kind string

const (
	KindEvent   Kind = "event"
	KindData    Kind = "data"
	KindCompute Kind = "compute"
)

// LimiterConfig holds configuration for a bus limiter.
type LimiterConfig struct {
	// MaxConcurrent is the maximum number of concurrent operations.
	// 0 means unlimited.
	MaxConcurrent int

	// AcquireTimeout is the maximum time to wait for a permit.
	// 0 means no timeout (wait indefinitely).
	AcquireTimeout time.Duration

	// QueueSize is the maximum number of waiting operations.
	// 0 means unlimited queue.
	QueueSize int
}

// DefaultLimiterConfig returns a sensible default configuration.
func DefaultLimiterConfig() LimiterConfig {
	return LimiterConfig{
		MaxConcurrent:  100,
		AcquireTimeout: 30 * time.Second,
		QueueSize:      1000,
	}
}

// Limiter enforces concurrency limits for bus operations.
type Limiter struct {
	mu        sync.Mutex
	config    LimiterConfig
	permits   chan struct{}
	waiting   int32
	active    int32
	closed    bool

	// Stats
	totalAcquired int64
	totalReleased int64
	totalRejected int64
	totalTimeouts int64
}

// NewLimiter creates a new bus limiter.
func NewLimiter(config LimiterConfig) *Limiter {
	l := &Limiter{
		config: config,
	}

	if config.MaxConcurrent > 0 {
		l.permits = make(chan struct{}, config.MaxConcurrent)
		// Pre-fill permits
		for i := 0; i < config.MaxConcurrent; i++ {
			l.permits <- struct{}{}
		}
	}

	return l
}

// Acquire attempts to acquire a permit for an operation.
// It blocks until a permit is available or the context is cancelled.
func (l *Limiter) Acquire(ctx context.Context) error {
	if l.config.MaxConcurrent <= 0 {
		// No limit
		atomic.AddInt32(&l.active, 1)
		atomic.AddInt64(&l.totalAcquired, 1)
		return nil
	}

	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return ErrLimiterClosed
	}

	// Check queue limit
	if l.config.QueueSize > 0 && int(atomic.LoadInt32(&l.waiting)) >= l.config.QueueSize {
		l.mu.Unlock()
		atomic.AddInt64(&l.totalRejected, 1)
		return ErrLimitExceeded
	}

	atomic.AddInt32(&l.waiting, 1)
	l.mu.Unlock()

	defer atomic.AddInt32(&l.waiting, -1)

	// Set up timeout if configured
	var timeoutCh <-chan time.Time
	if l.config.AcquireTimeout > 0 {
		timer := time.NewTimer(l.config.AcquireTimeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case <-l.permits:
		atomic.AddInt32(&l.active, 1)
		atomic.AddInt64(&l.totalAcquired, 1)
		return nil
	case <-ctx.Done():
		atomic.AddInt64(&l.totalTimeouts, 1)
		return ctx.Err()
	case <-timeoutCh:
		atomic.AddInt64(&l.totalTimeouts, 1)
		return ErrAcquireTimeout
	}
}

// TryAcquire attempts to acquire a permit without blocking.
// Returns true if permit was acquired, false otherwise.
func (l *Limiter) TryAcquire() bool {
	if l.config.MaxConcurrent <= 0 {
		// No limit
		atomic.AddInt32(&l.active, 1)
		atomic.AddInt64(&l.totalAcquired, 1)
		return true
	}

	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return false
	}
	l.mu.Unlock()

	select {
	case <-l.permits:
		atomic.AddInt32(&l.active, 1)
		atomic.AddInt64(&l.totalAcquired, 1)
		return true
	default:
		atomic.AddInt64(&l.totalRejected, 1)
		return false
	}
}

// Release releases a permit back to the pool.
func (l *Limiter) Release() {
	atomic.AddInt32(&l.active, -1)
	atomic.AddInt64(&l.totalReleased, 1)

	if l.config.MaxConcurrent > 0 && l.permits != nil {
		l.mu.Lock()
		if !l.closed {
			select {
			case l.permits <- struct{}{}:
			default:
				// Channel full - shouldn't happen if acquire/release are balanced
			}
		}
		l.mu.Unlock()
	}
}

// Close closes the limiter and releases any waiting goroutines.
func (l *Limiter) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}
	l.closed = true

	if l.permits != nil {
		close(l.permits)
	}
}

// Stats returns current limiter statistics.
type Stats struct {
	MaxConcurrent int   `json:"max_concurrent"`
	Active        int   `json:"active"`
	Waiting       int   `json:"waiting"`
	TotalAcquired int64 `json:"total_acquired"`
	TotalReleased int64 `json:"total_released"`
	TotalRejected int64 `json:"total_rejected"`
	TotalTimeouts int64 `json:"total_timeouts"`
}

// Stats returns current statistics.
func (l *Limiter) Stats() Stats {
	return Stats{
		MaxConcurrent: l.config.MaxConcurrent,
		Active:        int(atomic.LoadInt32(&l.active)),
		Waiting:       int(atomic.LoadInt32(&l.waiting)),
		TotalAcquired: atomic.LoadInt64(&l.totalAcquired),
		TotalReleased: atomic.LoadInt64(&l.totalReleased),
		TotalRejected: atomic.LoadInt64(&l.totalRejected),
		TotalTimeouts: atomic.LoadInt64(&l.totalTimeouts),
	}
}

// Active returns the number of currently active permits.
func (l *Limiter) Active() int {
	return int(atomic.LoadInt32(&l.active))
}

// Waiting returns the number of goroutines waiting for permits.
func (l *Limiter) Waiting() int {
	return int(atomic.LoadInt32(&l.waiting))
}

// Available returns the number of available permits.
func (l *Limiter) Available() int {
	if l.config.MaxConcurrent <= 0 {
		return -1 // Unlimited
	}
	return l.config.MaxConcurrent - int(atomic.LoadInt32(&l.active))
}

// BusLimiter manages limiters for different bus kinds.
type BusLimiter struct {
	mu       sync.RWMutex
	limiters map[Kind]*Limiter
	configs  map[Kind]LimiterConfig
}

// NewBusLimiter creates a new bus limiter manager.
func NewBusLimiter() *BusLimiter {
	return &BusLimiter{
		limiters: make(map[Kind]*Limiter),
		configs:  make(map[Kind]LimiterConfig),
	}
}

// Configure sets the configuration for a bus kind.
func (bl *BusLimiter) Configure(kind Kind, config LimiterConfig) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	// Close existing limiter if any
	if existing, ok := bl.limiters[kind]; ok {
		existing.Close()
	}

	bl.configs[kind] = config
	bl.limiters[kind] = NewLimiter(config)
}

// Acquire acquires a permit for the specified bus kind.
func (bl *BusLimiter) Acquire(ctx context.Context, kind Kind) error {
	bl.mu.RLock()
	limiter, ok := bl.limiters[kind]
	bl.mu.RUnlock()

	if !ok {
		// No limiter configured, allow operation
		return nil
	}

	return limiter.Acquire(ctx)
}

// TryAcquire attempts to acquire a permit without blocking.
func (bl *BusLimiter) TryAcquire(kind Kind) bool {
	bl.mu.RLock()
	limiter, ok := bl.limiters[kind]
	bl.mu.RUnlock()

	if !ok {
		return true
	}

	return limiter.TryAcquire()
}

// Release releases a permit for the specified bus kind.
func (bl *BusLimiter) Release(kind Kind) {
	bl.mu.RLock()
	limiter, ok := bl.limiters[kind]
	bl.mu.RUnlock()

	if ok {
		limiter.Release()
	}
}

// Stats returns statistics for all bus kinds.
func (bl *BusLimiter) Stats() map[Kind]Stats {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	result := make(map[Kind]Stats, len(bl.limiters))
	for kind, limiter := range bl.limiters {
		result[kind] = limiter.Stats()
	}
	return result
}

// StatsFor returns statistics for a specific bus kind.
func (bl *BusLimiter) StatsFor(kind Kind) (Stats, bool) {
	bl.mu.RLock()
	limiter, ok := bl.limiters[kind]
	bl.mu.RUnlock()

	if !ok {
		return Stats{}, false
	}
	return limiter.Stats(), true
}

// Close closes all limiters.
func (bl *BusLimiter) Close() {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	for _, limiter := range bl.limiters {
		limiter.Close()
	}
	bl.limiters = make(map[Kind]*Limiter)
}

// Guard is a convenience wrapper for acquire/release pattern.
type Guard struct {
	limiter *BusLimiter
	kind    Kind
	acquired bool
}

// NewGuard creates a guard that acquires a permit on creation.
// Returns nil if acquire fails.
func NewGuard(ctx context.Context, bl *BusLimiter, kind Kind) (*Guard, error) {
	if err := bl.Acquire(ctx, kind); err != nil {
		return nil, err
	}
	return &Guard{
		limiter:  bl,
		kind:     kind,
		acquired: true,
	}, nil
}

// Release releases the permit. Safe to call multiple times.
func (g *Guard) Release() {
	if g != nil && g.acquired {
		g.limiter.Release(g.kind)
		g.acquired = false
	}
}

// TryGuard creates a guard using non-blocking acquire.
// Returns nil if acquire fails.
func TryGuard(bl *BusLimiter, kind Kind) *Guard {
	if bl.TryAcquire(kind) {
		return &Guard{
			limiter:  bl,
			kind:     kind,
			acquired: true,
		}
	}
	return nil
}
