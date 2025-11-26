package httpapi

import (
	"testing"
	"time"
)

func TestMinuteLimiter_Allow(t *testing.T) {
	// Test nil limiter always allows
	var nilLimiter *minuteLimiter
	ok, retry := nilLimiter.Allow("key")
	if !ok || retry != 0 {
		t.Fatalf("nil limiter should allow all")
	}

	// Test basic limiter
	limiter := newMinuteLimiter(2, 0)
	if limiter == nil {
		t.Fatalf("expected limiter to be created")
	}

	// First two requests should be allowed
	for i := 0; i < 2; i++ {
		ok, _ := limiter.Allow("user1")
		if !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Third request should be denied
	ok, retry = limiter.Allow("user1")
	if ok {
		t.Fatalf("third request should be denied")
	}
	if retry <= 0 {
		t.Fatalf("retry should be positive")
	}

	// Different user should be allowed
	ok, _ = limiter.Allow("user2")
	if !ok {
		t.Fatalf("different user should be allowed")
	}
}

func TestMinuteLimiter_AllowWithBurst(t *testing.T) {
	limiter := newMinuteLimiter(2, 1) // 2 per minute + 1 burst
	if limiter == nil {
		t.Fatalf("expected limiter to be created")
	}

	// Should allow 3 requests (2 + 1 burst)
	for i := 0; i < 3; i++ {
		ok, _ := limiter.Allow("user1")
		if !ok {
			t.Fatalf("request %d should be allowed with burst", i+1)
		}
	}

	// Fourth request should be denied
	ok, _ := limiter.Allow("user1")
	if ok {
		t.Fatalf("fourth request should be denied")
	}
}

func TestMinuteLimiter_EmptyKey(t *testing.T) {
	limiter := newMinuteLimiter(2, 0)

	// Empty key should use anonymous
	ok, _ := limiter.Allow("")
	if !ok {
		t.Fatalf("empty key should be allowed")
	}
	ok, _ = limiter.Allow("   ")
	if !ok {
		t.Fatalf("whitespace key should be allowed")
	}
	ok, _ = limiter.Allow("")
	if ok {
		t.Fatalf("anonymous requests should share limit")
	}
}

func TestMinuteLimiter_ZeroOrNegativeLimit(t *testing.T) {
	// Zero limit should return nil
	limiter := newMinuteLimiter(0, 0)
	if limiter != nil {
		t.Fatalf("zero limit should return nil limiter")
	}

	// Negative limit should return nil
	limiter = newMinuteLimiter(-1, 0)
	if limiter != nil {
		t.Fatalf("negative limit should return nil limiter")
	}
}

func TestMinuteLimiter_NegativeBurst(t *testing.T) {
	// Negative burst should be treated as 0
	limiter := newMinuteLimiter(2, -5)
	if limiter == nil {
		t.Fatalf("expected limiter to be created")
	}
	if limiter.burst != 0 {
		t.Fatalf("expected burst to be 0, got %d", limiter.burst)
	}
}

func TestNewRPCPolicy(t *testing.T) {
	// Empty policy should return nil
	policy := newRPCPolicy(RPCPolicy{})
	if policy != nil {
		t.Fatalf("empty policy should return nil")
	}

	// Policy with only allowed methods
	policy = newRPCPolicy(RPCPolicy{
		AllowedMethods: map[string][]string{
			"neo": {"getblock", "getblockcount"},
		},
	})
	if policy == nil {
		t.Fatalf("policy with allowed methods should not be nil")
	}
	if policy.requireTenant {
		t.Fatalf("should not require tenant")
	}

	// Policy with tenant requirement
	policy = newRPCPolicy(RPCPolicy{
		RequireTenant: true,
	})
	if policy == nil {
		t.Fatalf("policy with tenant requirement should not be nil")
	}
	if !policy.requireTenant {
		t.Fatalf("should require tenant")
	}

	// Policy with rate limits
	policy = newRPCPolicy(RPCPolicy{
		PerTenantPerMinute: 100,
		PerTokenPerMinute:  50,
		Burst:              10,
	})
	if policy == nil {
		t.Fatalf("policy with rate limits should not be nil")
	}
	if policy.tenantLimiter == nil {
		t.Fatalf("tenant limiter should be created")
	}
	if policy.tokenLimiter == nil {
		t.Fatalf("token limiter should be created")
	}
}

func TestRPCPolicy_Allow(t *testing.T) {
	// Nil policy should allow all
	var nilPolicy *rpcPolicy
	ok, retry, reason := nilPolicy.allow("tenant", "token")
	if !ok || retry != 0 || reason != "" {
		t.Fatalf("nil policy should allow all")
	}

	// Policy requiring tenant
	policy := newRPCPolicy(RPCPolicy{RequireTenant: true})
	ok, _, reason = policy.allow("", "token")
	if ok || reason != "tenant-required" {
		t.Fatalf("should require tenant")
	}
	ok, _, _ = policy.allow("tenant", "token")
	if !ok {
		t.Fatalf("should allow with tenant")
	}

	// Policy with rate limits
	policy = newRPCPolicy(RPCPolicy{
		PerTenantPerMinute: 1,
		PerTokenPerMinute:  1,
	})
	ok, _, _ = policy.allow("tenant", "token")
	if !ok {
		t.Fatalf("first request should be allowed")
	}
	ok, _, reason = policy.allow("tenant", "token")
	if ok || reason == "" {
		t.Fatalf("second request should be rate limited")
	}
}

func TestRPCPolicy_MethodAllowed(t *testing.T) {
	// Nil policy allows all methods
	var nilPolicy *rpcPolicy
	if !nilPolicy.methodAllowed("neo", "getblock") {
		t.Fatalf("nil policy should allow all methods")
	}

	// Policy with no allowed methods allows all
	policy := &rpcPolicy{}
	if !policy.methodAllowed("neo", "getblock") {
		t.Fatalf("empty allowed map should allow all")
	}

	// Policy with specific allowed methods
	policy = newRPCPolicy(RPCPolicy{
		AllowedMethods: map[string][]string{
			"neo": {"getblock", "getblockcount"},
		},
	})
	if !policy.methodAllowed("neo", "getblock") {
		t.Fatalf("should allow getblock")
	}
	if !policy.methodAllowed("NEO", "GETBLOCK") {
		t.Fatalf("should allow case insensitive")
	}
	if policy.methodAllowed("neo", "sendrawtransaction") {
		t.Fatalf("should not allow sendrawtransaction")
	}
	if !policy.methodAllowed("eth", "eth_call") {
		t.Fatalf("unlisted chain should allow all methods")
	}

	// Empty chain or method should be disallowed
	if policy.methodAllowed("", "getblock") {
		t.Fatalf("empty chain should not be allowed")
	}
	if policy.methodAllowed("neo", "") {
		t.Fatalf("empty method should not be allowed")
	}
}

func TestNormalizeAllowed(t *testing.T) {
	// Nil input
	if normalizeAllowed(nil) != nil {
		t.Fatalf("nil input should return nil")
	}

	// Empty input
	if normalizeAllowed(map[string][]string{}) != nil {
		t.Fatalf("empty input should return nil")
	}

	// Normal input
	input := map[string][]string{
		"Neo":  {"GetBlock", "getBlockCount"},
		"ETH":  {"eth_call"},
		"":     {"ignored"},
		"bsc":  {"", "  ", "eth_call"},
		"  ":   {"also ignored"},
	}
	result := normalizeAllowed(input)
	if result == nil {
		t.Fatalf("should have result")
	}

	// Check normalization
	if !result["neo"]["getblock"] {
		t.Fatalf("should have normalized getblock")
	}
	if !result["neo"]["getblockcount"] {
		t.Fatalf("should have normalized getblockcount")
	}
	if !result["eth"]["eth_call"] {
		t.Fatalf("should have eth_call")
	}
	if result[""]["ignored"] {
		t.Fatalf("empty chain should be ignored")
	}
	if len(result["bsc"]) != 1 || !result["bsc"]["eth_call"] {
		t.Fatalf("empty methods should be filtered")
	}
}

func TestRPCPolicy_TenantRateLimiting(t *testing.T) {
	policy := newRPCPolicy(RPCPolicy{
		PerTenantPerMinute: 2,
		Burst:              0,
	})

	// Two requests from same tenant should succeed
	for i := 0; i < 2; i++ {
		ok, _, _ := policy.allow("tenant1", "token")
		if !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Third should fail
	ok, retry, reason := policy.allow("tenant1", "token")
	if ok {
		t.Fatalf("third request should be denied")
	}
	if reason != "tenant-limit" {
		t.Fatalf("expected tenant-limit reason, got %s", reason)
	}
	if retry <= 0 {
		t.Fatalf("retry should be positive")
	}

	// Different tenant should succeed
	ok, _, _ = policy.allow("tenant2", "token")
	if !ok {
		t.Fatalf("different tenant should be allowed")
	}
}

func TestRPCPolicy_TokenRateLimiting(t *testing.T) {
	policy := newRPCPolicy(RPCPolicy{
		PerTokenPerMinute: 2,
		Burst:             0,
	})

	// Two requests from same token should succeed
	for i := 0; i < 2; i++ {
		ok, _, _ := policy.allow("tenant", "token1")
		if !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Third should fail
	ok, _, reason := policy.allow("tenant", "token1")
	if ok {
		t.Fatalf("third request should be denied")
	}
	if reason != "token-limit" {
		t.Fatalf("expected token-limit reason, got %s", reason)
	}

	// Different token should succeed
	ok, _, _ = policy.allow("tenant", "token2")
	if !ok {
		t.Fatalf("different token should be allowed")
	}
}

func TestRPCPolicy_WindowReset(t *testing.T) {
	limiter := newMinuteLimiter(1, 0)

	// First request succeeds
	ok, _ := limiter.Allow("user")
	if !ok {
		t.Fatalf("first request should succeed")
	}

	// Second fails
	ok, _ = limiter.Allow("user")
	if ok {
		t.Fatalf("second request should fail")
	}

	// Simulate time passing by directly manipulating bucket
	limiter.mu.Lock()
	b := limiter.buckets["user"]
	b.start = b.start.Add(-2 * time.Minute) // Move window back
	limiter.buckets["user"] = b
	limiter.mu.Unlock()

	// Should succeed after window reset
	ok, _ = limiter.Allow("user")
	if !ok {
		t.Fatalf("request after window reset should succeed")
	}
}
