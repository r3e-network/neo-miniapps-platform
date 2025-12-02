package service

import (
	"context"
	"time"
)

// RetryPolicy governs retry behavior.
type RetryPolicy struct {
	Attempts       int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// DefaultRetryPolicy preserves current behavior (single attempt, no backoff).
var DefaultRetryPolicy = RetryPolicy{
	Attempts:       1,
	InitialBackoff: 0,
	MaxBackoff:     0,
	Multiplier:     1,
}

// Retry executes fn with the provided policy. It returns the last error (if any).
func Retry(ctx context.Context, policy RetryPolicy, fn func() error) error {
	if policy.Attempts <= 0 {
		policy.Attempts = 1
	}
	if policy.Multiplier <= 0 {
		policy.Multiplier = 1
	}
	backoff := policy.InitialBackoff
	for attempt := 1; attempt <= policy.Attempts; attempt++ {
		if err := fn(); err != nil {
			if attempt == policy.Attempts {
				return err
			}
			// Apply backoff before next attempt if requested.
			if backoff > 0 {
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return ctx.Err()
				}
				next := time.Duration(float64(backoff) * policy.Multiplier)
				if policy.MaxBackoff > 0 && next > policy.MaxBackoff {
					next = policy.MaxBackoff
				}
				backoff = next
			}
			continue
		}
		return nil
	}
	return nil
}
