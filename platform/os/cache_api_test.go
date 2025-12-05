// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/tee"
)

func setupCacheTest(t *testing.T) (*ServiceContext, func()) {
	trustRoot, err := tee.NewSimulation("test-enclave")
	if err != nil {
		t.Fatalf("failed to create trust root: %v", err)
	}

	ctx := context.Background()
	if err := trustRoot.Start(ctx); err != nil {
		t.Fatalf("failed to start trust root: %v", err)
	}

	manifest := &Manifest{
		ServiceID:            "test-service",
		RequiredCapabilities: []Capability{CapCache},
	}

	svcCtx, err := NewServiceContext(manifest, trustRoot, nil)
	if err != nil {
		trustRoot.Stop(ctx)
		t.Fatalf("failed to create context: %v", err)
	}

	cleanup := func() {
		svcCtx.Close()
		trustRoot.Stop(ctx)
	}

	return svcCtx, cleanup
}

func TestCacheAPI_SetAndGet(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	// Set a value
	err := cache.Set(ctx, "test-key", []byte("test-value"), 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Get the value
	value, err := cache.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if string(value) != "test-value" {
		t.Errorf("Get() = %s, want test-value", string(value))
	}
}

func TestCacheAPI_GetNonExistent(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	// Get non-existent key
	value, err := cache.Get(ctx, "non-existent")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if value != nil {
		t.Errorf("Get() = %v, want nil", value)
	}
}

func TestCacheAPI_Delete(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	// Set and delete
	cache.Set(ctx, "test-key", []byte("test-value"), 0)
	err := cache.Delete(ctx, "test-key")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify deleted
	value, _ := cache.Get(ctx, "test-key")
	if value != nil {
		t.Error("key should be deleted")
	}
}

func TestCacheAPI_Exists(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	// Check non-existent
	exists, err := cache.Exists(ctx, "test-key")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}

	// Set and check
	cache.Set(ctx, "test-key", []byte("test-value"), 0)
	exists, err = cache.Exists(ctx, "test-key")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}
}

func TestCacheAPI_TTL(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	// Set with TTL
	err := cache.Set(ctx, "test-key", []byte("test-value"), 1*time.Hour)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Check TTL
	ttl, err := cache.TTL(ctx, "test-key")
	if err != nil {
		t.Fatalf("TTL() error: %v", err)
	}
	if ttl < 59*time.Minute || ttl > 1*time.Hour {
		t.Errorf("TTL() = %v, want ~1h", ttl)
	}
}

func TestCacheAPI_Expiration(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	// Set with very short TTL
	err := cache.Set(ctx, "test-key", []byte("test-value"), 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Should be expired
	value, _ := cache.Get(ctx, "test-key")
	if value != nil {
		t.Error("value should be expired")
	}
}

func TestCacheAPI_Clear(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	// Set multiple values
	cache.Set(ctx, "key1", []byte("value1"), 0)
	cache.Set(ctx, "key2", []byte("value2"), 0)
	cache.Set(ctx, "key3", []byte("value3"), 0)

	// Clear all
	err := cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	// Verify all cleared
	for _, key := range []string{"key1", "key2", "key3"} {
		value, _ := cache.Get(ctx, key)
		if value != nil {
			t.Errorf("key %s should be cleared", key)
		}
	}
}

func TestCacheAPI_GetOrSet(t *testing.T) {
	svcCtx, cleanup := setupCacheTest(t)
	defer cleanup()

	ctx := context.Background()
	cache := svcCtx.Cache()

	callCount := 0
	factory := func() (any, error) {
		callCount++
		return []byte("computed-value"), nil
	}

	// First call should invoke factory
	value, err := cache.GetOrSet(ctx, "test-key", factory, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error: %v", err)
	}
	if string(value) != "computed-value" {
		t.Errorf("GetOrSet() = %s, want computed-value", string(value))
	}
	if callCount != 1 {
		t.Errorf("factory called %d times, want 1", callCount)
	}

	// Second call should use cached value
	value, err = cache.GetOrSet(ctx, "test-key", factory, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error: %v", err)
	}
	if string(value) != "computed-value" {
		t.Errorf("GetOrSet() = %s, want computed-value", string(value))
	}
	if callCount != 1 {
		t.Errorf("factory called %d times, want 1 (should use cache)", callCount)
	}
}
