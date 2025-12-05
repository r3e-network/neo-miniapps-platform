// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/tee"
)

func setupConfigTest(t *testing.T) (*ServiceContext, func()) {
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
		RequiredCapabilities: []Capability{CapConfig},
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

func TestConfigAPI_SetAndGet(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	// Set a value
	err := config.Set(ctx, "test-key", "test-value")
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Get the value
	value, err := config.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if value != "test-value" {
		t.Errorf("Get() = %v, want test-value", value)
	}
}

func TestConfigAPI_GetString(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	config.Set(ctx, "string-key", "hello")
	config.Set(ctx, "int-key", 42)

	// Get string
	str, err := config.GetString(ctx, "string-key")
	if err != nil {
		t.Fatalf("GetString() error: %v", err)
	}
	if str != "hello" {
		t.Errorf("GetString() = %s, want hello", str)
	}

	// Get int as string
	str, err = config.GetString(ctx, "int-key")
	if err != nil {
		t.Fatalf("GetString() error: %v", err)
	}
	if str != "42" {
		t.Errorf("GetString() = %s, want 42", str)
	}
}

func TestConfigAPI_GetInt(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	config.Set(ctx, "int-key", 42)
	config.Set(ctx, "int64-key", int64(100))
	config.Set(ctx, "float-key", 3.14)

	// Get int
	val, err := config.GetInt(ctx, "int-key")
	if err != nil {
		t.Fatalf("GetInt() error: %v", err)
	}
	if val != 42 {
		t.Errorf("GetInt() = %d, want 42", val)
	}

	// Get int64 as int
	val, err = config.GetInt(ctx, "int64-key")
	if err != nil {
		t.Fatalf("GetInt() error: %v", err)
	}
	if val != 100 {
		t.Errorf("GetInt() = %d, want 100", val)
	}

	// Get float as int
	val, err = config.GetInt(ctx, "float-key")
	if err != nil {
		t.Fatalf("GetInt() error: %v", err)
	}
	if val != 3 {
		t.Errorf("GetInt() = %d, want 3", val)
	}
}

func TestConfigAPI_GetBool(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	config.Set(ctx, "true-key", true)
	config.Set(ctx, "false-key", false)

	// Get true
	val, err := config.GetBool(ctx, "true-key")
	if err != nil {
		t.Fatalf("GetBool() error: %v", err)
	}
	if !val {
		t.Error("GetBool() = false, want true")
	}

	// Get false
	val, err = config.GetBool(ctx, "false-key")
	if err != nil {
		t.Fatalf("GetBool() error: %v", err)
	}
	if val {
		t.Error("GetBool() = true, want false")
	}
}

func TestConfigAPI_GetDuration(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	config.Set(ctx, "duration-key", 5*time.Second)
	config.Set(ctx, "string-duration", "10s")

	// Get duration
	val, err := config.GetDuration(ctx, "duration-key")
	if err != nil {
		t.Fatalf("GetDuration() error: %v", err)
	}
	if val != 5*time.Second {
		t.Errorf("GetDuration() = %v, want 5s", val)
	}

	// Get string as duration
	val, err = config.GetDuration(ctx, "string-duration")
	if err != nil {
		t.Fatalf("GetDuration() error: %v", err)
	}
	if val != 10*time.Second {
		t.Errorf("GetDuration() = %v, want 10s", val)
	}
}

func TestConfigAPI_Delete(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	config.Set(ctx, "test-key", "test-value")
	err := config.Delete(ctx, "test-key")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify deleted
	value, _ := config.Get(ctx, "test-key")
	if value != nil {
		t.Error("key should be deleted")
	}
}

func TestConfigAPI_All(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	config.Set(ctx, "key1", "value1")
	config.Set(ctx, "key2", "value2")

	all, err := config.All(ctx)
	if err != nil {
		t.Fatalf("All() error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("All() returned %d items, want 2", len(all))
	}
}

func TestConfigAPI_Watch(t *testing.T) {
	svcCtx, cleanup := setupConfigTest(t)
	defer cleanup()

	ctx := context.Background()
	config := svcCtx.Config()

	// Set up watcher
	watchedKey := ""
	watchedValue := any(nil)
	err := config.Watch(ctx, "watched-key", func(key string, value any) {
		watchedKey = key
		watchedValue = value
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	// Set value to trigger watcher
	config.Set(ctx, "watched-key", "new-value")

	// Verify watcher was called
	if watchedKey != "watched-key" {
		t.Errorf("watcher key = %s, want watched-key", watchedKey)
	}
	if watchedValue != "new-value" {
		t.Errorf("watcher value = %v, want new-value", watchedValue)
	}
}
