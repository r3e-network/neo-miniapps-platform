package signer

import (
	"testing"
	"time"
)

func TestClientRateLimiter_PerClientIsolation(t *testing.T) {
	now := time.Unix(0, 0)
	lim := NewClientRateLimiter(1, 1, func() time.Time { return now })

	if !lim.Allow("client-a") {
		t.Fatal("expected first request to pass")
	}
	if lim.Allow("client-a") {
		t.Fatal("expected second request to be rate limited")
	}
	if !lim.Allow("client-b") {
		t.Fatal("expected other client to have independent bucket")
	}
}

func TestClientRateLimiter_Refill(t *testing.T) {
	now := time.Unix(0, 0)
	lim := NewClientRateLimiter(2, 2, func() time.Time { return now })

	if !lim.Allow("c") || !lim.Allow("c") {
		t.Fatal("expected burst to pass")
	}
	if lim.Allow("c") {
		t.Fatal("expected to be rate limited after tokens exhausted")
	}

	now = now.Add(500 * time.Millisecond) // +1 token at 2/s
	if !lim.Allow("c") {
		t.Fatal("expected refill to allow request")
	}
}

func TestClientRateLimiter_UnknownClientKey(t *testing.T) {
	now := time.Unix(0, 0)
	lim := NewClientRateLimiter(1, 1, func() time.Time { return now })

	if !lim.Allow("") {
		t.Fatal("expected empty CN to be treated as unknown and allowed initially")
	}
	if lim.Allow("unknown") {
		t.Fatal("expected unknown bucket to be shared")
	}
}

func TestNewClientRateLimiter_DefaultsAndTimeGoesBackwards(t *testing.T) {
	now := time.Unix(10, 0)
	lim := NewClientRateLimiter(0, 0, nil)
	if lim == nil {
		t.Fatal("expected limiter")
	}

	// Force a bucket to exist with a known last time, then move time backwards.
	lim.now = func() time.Time { return now }
	if !lim.Allow("client") {
		t.Fatal("expected initial allow")
	}

	now = now.Add(-5 * time.Second)
	if lim.Allow("client") {
		// With capacity=1 by default, allowing twice without time passing would be unexpected.
		t.Fatal("expected rate limit when time goes backwards")
	}
}
