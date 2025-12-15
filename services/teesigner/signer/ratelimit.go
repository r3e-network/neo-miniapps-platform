package signer

import (
	"strings"
	"sync"
	"time"
)

type tokenBucket struct {
	mu       sync.Mutex
	rate     float64
	capacity float64
	tokens   float64
	last     time.Time
}

func newTokenBucket(rate float64, capacity float64, now time.Time) *tokenBucket {
	return &tokenBucket{
		rate:     rate,
		capacity: capacity,
		tokens:   capacity,
		last:     now,
	}
}

func (b *tokenBucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if now.Before(b.last) {
		b.last = now
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

type ClientRateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64
	capacity float64
	now      func() time.Time
}

func NewClientRateLimiter(reqPerSecond float64, burst int, now func() time.Time) *ClientRateLimiter {
	if now == nil {
		now = time.Now
	}

	capacity := float64(burst)
	if capacity < 1 {
		capacity = 1
	}
	if reqPerSecond <= 0 {
		reqPerSecond = 1
	}

	return &ClientRateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     reqPerSecond,
		capacity: capacity,
		now:      now,
	}
}

func (l *ClientRateLimiter) Allow(clientCN string) bool {
	clientCN = strings.TrimSpace(clientCN)
	if clientCN == "" {
		clientCN = "unknown"
	}

	l.mu.Lock()
	b, ok := l.buckets[clientCN]
	if !ok {
		b = newTokenBucket(l.rate, l.capacity, l.now())
		l.buckets[clientCN] = b
	}
	l.mu.Unlock()

	return b.allow(l.now())
}
