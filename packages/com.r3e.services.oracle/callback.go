package oracle

import (
	"context"
	"sync"
	"time"
)

// RequestResolver inspects a pending oracle request and determines whether it has finished.
type RequestResolver interface {
	Resolve(ctx context.Context, req Request) (done bool, success bool, result string, errMsg string, retryAfter time.Duration, err error)
}

// RequestResolverFunc adapts a function into a RequestResolver.
type RequestResolverFunc func(ctx context.Context, req Request) (bool, bool, string, string, time.Duration, error)

func (f RequestResolverFunc) Resolve(ctx context.Context, req Request) (bool, bool, string, string, time.Duration, error) {
	if f == nil {
		return false, false, "", "", 0, nil
	}
	return f(ctx, req)
}

// TimeoutResolver marks requests as failed after a timeout period.
type TimeoutResolver struct {
	timeout time.Duration
	seen    sync.Map
}

func NewTimeoutResolver(timeout time.Duration) *TimeoutResolver {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &TimeoutResolver{timeout: timeout}
}

func (r *TimeoutResolver) Resolve(ctx context.Context, req Request) (bool, bool, string, string, time.Duration, error) {
	if req.Status == StatusSucceeded || req.Status == StatusFailed {
		return true, req.Status == StatusSucceeded, req.Result, req.Error, 0, nil
	}
	if value, ok := r.seen.Load(req.ID); ok {
		if time.Since(value.(time.Time)) >= r.timeout {
			return true, false, "", "timeout awaiting oracle callback", 0, nil
		}
		return false, false, "", "", r.timeout / 4, nil
	}
	r.seen.Store(req.ID, time.Now())
	return false, false, "", "", r.timeout / 4, nil
}

// Dispatcher updated to use resolver
