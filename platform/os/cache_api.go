// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// cacheEntry represents a cached value with expiration.
type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// cacheAPIImpl implements CacheAPI.
type cacheAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	mu        sync.RWMutex
	cache     map[string]*cacheEntry
}

func newCacheAPI(ctx *ServiceContext, serviceID string) *cacheAPIImpl {
	return &cacheAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
		cache:     make(map[string]*cacheEntry),
	}
}

func (c *cacheAPIImpl) Get(ctx context.Context, key string) ([]byte, error) {
	if err := c.ctx.RequireCapability(CapCache); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, nil
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return nil, nil
	}
	return entry.value, nil
}

func (c *cacheAPIImpl) GetInto(ctx context.Context, key string, dest any) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if data == nil {
		return NewOSError("NOT_FOUND", "cache key not found: "+key)
	}
	return json.Unmarshal(data, dest)
}

func (c *cacheAPIImpl) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := c.ctx.RequireCapability(CapCache); err != nil {
		return err
	}

	var data []byte
	var err error
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		data, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal cache value: %w", err)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &cacheEntry{value: data}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.cache[key] = entry
	return nil
}

func (c *cacheAPIImpl) Delete(ctx context.Context, key string) error {
	if err := c.ctx.RequireCapability(CapCache); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, key)
	return nil
}

func (c *cacheAPIImpl) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.ctx.RequireCapability(CapCache); err != nil {
		return false, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return false, nil
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return false, nil
	}
	return true, nil
}

func (c *cacheAPIImpl) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.ctx.RequireCapability(CapCache); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if !exists {
		return NewOSError("NOT_FOUND", "cache key not found: "+key)
	}
	entry.expiresAt = time.Now().Add(ttl)
	return nil
}

func (c *cacheAPIImpl) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.ctx.RequireCapability(CapCache); err != nil {
		return 0, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return 0, NewOSError("NOT_FOUND", "cache key not found: "+key)
	}
	if entry.expiresAt.IsZero() {
		return -1, nil // No expiration
	}
	remaining := time.Until(entry.expiresAt)
	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}

func (c *cacheAPIImpl) Clear(ctx context.Context) error {
	if err := c.ctx.RequireCapability(CapCache); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cacheEntry)
	return nil
}

func (c *cacheAPIImpl) GetOrSet(ctx context.Context, key string, fn func() (any, error), ttl time.Duration) ([]byte, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data != nil {
		return data, nil
	}

	value, err := fn()
	if err != nil {
		return nil, err
	}

	if err := c.Set(ctx, key, value, ttl); err != nil {
		return nil, err
	}

	return c.Get(ctx, key)
}
