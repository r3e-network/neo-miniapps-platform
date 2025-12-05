// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"sync"
	"time"
)

// lockEntry represents a distributed lock entry.
type lockEntry struct {
	key       string
	owner     string
	expiresAt time.Time
}

// lockAPIImpl implements LockAPI.
type lockAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	mu        sync.Mutex
	locks     map[string]*lockEntry
}

func newLockAPI(ctx *ServiceContext, serviceID string) *lockAPIImpl {
	return &lockAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
		locks:     make(map[string]*lockEntry),
	}
}

func (l *lockAPIImpl) Acquire(ctx context.Context, key string, ttl time.Duration) (Lock, error) {
	if err := l.ctx.RequireCapability(CapLock); err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if lock exists and is not expired
	if entry, exists := l.locks[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			// Lock is held, wait or return error
			return nil, NewOSError("LOCK_HELD", "lock is already held: "+key)
		}
	}

	// Acquire lock
	entry := &lockEntry{
		key:       key,
		owner:     l.serviceID,
		expiresAt: time.Now().Add(ttl),
	}
	l.locks[key] = entry

	return &lockImpl{
		entry:   entry,
		lockAPI: l,
	}, nil
}

func (l *lockAPIImpl) TryAcquire(ctx context.Context, key string, ttl time.Duration) (Lock, bool, error) {
	if err := l.ctx.RequireCapability(CapLock); err != nil {
		return nil, false, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if lock exists and is not expired
	if entry, exists := l.locks[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			return nil, false, nil
		}
	}

	// Acquire lock
	entry := &lockEntry{
		key:       key,
		owner:     l.serviceID,
		expiresAt: time.Now().Add(ttl),
	}
	l.locks[key] = entry

	return &lockImpl{
		entry:   entry,
		lockAPI: l,
	}, true, nil
}

func (l *lockAPIImpl) IsLocked(ctx context.Context, key string) (bool, error) {
	if err := l.ctx.RequireCapability(CapLock); err != nil {
		return false, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.locks[key]
	if !exists {
		return false, nil
	}
	return time.Now().Before(entry.expiresAt), nil
}

type lockImpl struct {
	entry   *lockEntry
	lockAPI *lockAPIImpl
}

func (l *lockImpl) Release(ctx context.Context) error {
	l.lockAPI.mu.Lock()
	defer l.lockAPI.mu.Unlock()

	delete(l.lockAPI.locks, l.entry.key)
	return nil
}

func (l *lockImpl) Extend(ctx context.Context, ttl time.Duration) error {
	l.lockAPI.mu.Lock()
	defer l.lockAPI.mu.Unlock()

	l.entry.expiresAt = time.Now().Add(ttl)
	return nil
}

func (l *lockImpl) Key() string {
	return l.entry.key
}
