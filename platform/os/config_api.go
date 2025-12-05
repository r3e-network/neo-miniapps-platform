// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// configAPIImpl implements ConfigAPI.
type configAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	mu        sync.RWMutex
	config    map[string]any
	watchers  map[string][]func(string, any)
}

func newConfigAPI(ctx *ServiceContext, serviceID string) *configAPIImpl {
	return &configAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
		config:    make(map[string]any),
		watchers:  make(map[string][]func(string, any)),
	}
}

func (c *configAPIImpl) Get(ctx context.Context, key string) (any, error) {
	if err := c.ctx.RequireCapability(CapConfig); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config[key], nil
}

func (c *configAPIImpl) GetString(ctx context.Context, key string) (string, error) {
	val, err := c.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	if s, ok := val.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", val), nil
}

func (c *configAPIImpl) GetInt(ctx context.Context, key string) (int, error) {
	val, err := c.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, NewOSError("TYPE_ERROR", "value is not an integer")
	}
}

func (c *configAPIImpl) GetBool(ctx context.Context, key string) (bool, error) {
	val, err := c.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if val == nil {
		return false, nil
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, NewOSError("TYPE_ERROR", "value is not a boolean")
}

func (c *configAPIImpl) GetDuration(ctx context.Context, key string) (time.Duration, error) {
	val, err := c.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case time.Duration:
		return v, nil
	case int64:
		return time.Duration(v), nil
	case string:
		return time.ParseDuration(v)
	default:
		return 0, NewOSError("TYPE_ERROR", "value is not a duration")
	}
}

func (c *configAPIImpl) Set(ctx context.Context, key string, value any) error {
	if err := c.ctx.RequireCapability(CapConfig); err != nil {
		return err
	}
	c.mu.Lock()
	c.config[key] = value
	watchers := c.watchers[key]
	c.mu.Unlock()

	// Notify watchers
	for _, w := range watchers {
		w(key, value)
	}
	return nil
}

func (c *configAPIImpl) Delete(ctx context.Context, key string) error {
	if err := c.ctx.RequireCapability(CapConfig); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.config, key)
	return nil
}

func (c *configAPIImpl) All(ctx context.Context) (map[string]any, error) {
	if err := c.ctx.RequireCapability(CapConfig); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]any, len(c.config))
	for k, v := range c.config {
		result[k] = v
	}
	return result, nil
}

func (c *configAPIImpl) Watch(ctx context.Context, key string, handler func(key string, value any)) error {
	if err := c.ctx.RequireCapability(CapConfig); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchers[key] = append(c.watchers[key], handler)
	return nil
}
