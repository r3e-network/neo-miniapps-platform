// Package sdk provides Marble SDK tests.
package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Configuration Tests
// =============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.CoordinatorAddr != "localhost:4433" {
		t.Errorf("CoordinatorAddr = %s, want localhost:4433", cfg.CoordinatorAddr)
	}
	if cfg.RetryConfig.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.RetryConfig.MaxRetries)
	}
	if cfg.HeartbeatInterval != 30*time.Second {
		t.Errorf("HeartbeatInterval = %v, want 30s", cfg.HeartbeatInterval)
	}
}

func TestNew(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if m == nil {
		t.Fatal("New() returned nil")
	}
	if m.MarbleType() != "test-marble" {
		t.Errorf("MarbleType() = %s, want test-marble", m.MarbleType())
	}
}

func TestNew_MissingMarbleType(t *testing.T) {
	cfg := Config{
		SimulationMode: true,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("New() should error when MarbleType is empty")
	}
}

func TestNew_GeneratesUUID(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)
	if m.UUID() == "" {
		t.Error("UUID() should not be empty")
	}
}

func TestNew_UsesProvidedUUID(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		UUID:           "custom-uuid-123",
		SimulationMode: true,
	}

	m, _ := New(cfg)
	if m.UUID() != "custom-uuid-123" {
		t.Errorf("UUID() = %s, want custom-uuid-123", m.UUID())
	}
}

// =============================================================================
// Identity Tests
// =============================================================================

func TestMarble_Identity(t *testing.T) {
	cfg := Config{
		MarbleType:     "oracle",
		UUID:           "test-uuid",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	if m.MarbleType() != "oracle" {
		t.Errorf("MarbleType() = %s, want oracle", m.MarbleType())
	}
	if m.UUID() != "test-uuid" {
		t.Errorf("UUID() = %s, want test-uuid", m.UUID())
	}
}

func TestMarble_IsActivated(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	if m.IsActivated() {
		t.Error("IsActivated() should be false before activation")
	}
}

func TestMarble_ActivatedAt(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	if !m.ActivatedAt().IsZero() {
		t.Error("ActivatedAt() should be zero before activation")
	}
}

// =============================================================================
// EGo Runtime Tests
// =============================================================================

func TestMarble_EgoRuntime(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	if m.EgoRuntime() == nil {
		t.Error("EgoRuntime() should not be nil")
	}
}

func TestMarble_InEnclave(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	// In simulation mode, should not be in enclave
	if m.InEnclave() {
		t.Error("InEnclave() should be false in simulation mode")
	}
}

func TestMarble_IsSimulation(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	if !m.IsSimulation() {
		t.Error("IsSimulation() should be true")
	}
}

// =============================================================================
// Sealing Tests
// =============================================================================

func TestMarble_SealUnseal(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)
	data := []byte("secret data to seal")

	sealed, err := m.Seal(data)
	if err != nil {
		t.Fatalf("Seal() error: %v", err)
	}

	unsealed, err := m.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal() error: %v", err)
	}

	if string(unsealed) != string(data) {
		t.Errorf("Unseal() = %s, want %s", unsealed, data)
	}
}

func TestMarble_GenerateQuote(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)
	data := []byte("attestation data")

	quote, err := m.GenerateQuote(data)
	if err != nil {
		t.Fatalf("GenerateQuote() error: %v", err)
	}

	if len(quote) == 0 {
		t.Error("GenerateQuote() returned empty quote")
	}
}

// =============================================================================
// Health Tests
// =============================================================================

func TestMarble_Health(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)
	health := m.Health()

	if health.MarbleType != "test-marble" {
		t.Errorf("Health.MarbleType = %s, want test-marble", health.MarbleType)
	}
	if health.Status != "not_activated" {
		t.Errorf("Health.Status = %s, want not_activated", health.Status)
	}
	if health.Activated {
		t.Error("Health.Activated should be false")
	}
	if !health.SimulationMode {
		t.Error("Health.SimulationMode should be true")
	}
}

// =============================================================================
// Activation Tests (with mock server)
// =============================================================================

func TestMarble_Activate_Success(t *testing.T) {
	// Create mock coordinator
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/marble/activate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := ActivationResponse{
			Secrets: map[string][]byte{
				"api_key": []byte("secret-key"),
			},
			Env: map[string]string{
				"SUPABASE_URL": "http://localhost:54321",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		MarbleType:      "test-marble",
		CoordinatorAddr: server.Listener.Addr().String(),
		SimulationMode:  true,
		RetryConfig: RetryConfig{
			MaxRetries:        1,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
		},
	}

	m, _ := New(cfg)
	ctx := context.Background()

	err := m.Activate(ctx)
	if err != nil {
		t.Fatalf("Activate() error: %v", err)
	}

	if !m.IsActivated() {
		t.Error("IsActivated() should be true after activation")
	}
}

func TestMarble_Activate_AlreadyActivated(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ActivationResponse{}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		MarbleType:      "test-marble",
		CoordinatorAddr: server.Listener.Addr().String(),
		SimulationMode:  true,
		RetryConfig: RetryConfig{
			MaxRetries:     1,
			InitialBackoff: 10 * time.Millisecond,
		},
	}

	m, _ := New(cfg)
	ctx := context.Background()

	// First activation
	m.Activate(ctx)

	// Second activation should fail
	err := m.Activate(ctx)
	if err == nil {
		t.Error("Activate() should error when already activated")
	}
}

// =============================================================================
// Secret Access Tests
// =============================================================================

func TestMarble_GetSecret_NotActivated(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	_, err := m.GetSecret("api_key")
	if err == nil {
		t.Error("GetSecret() should error when not activated")
	}
}

func TestMarble_HasSecret(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	if m.HasSecret("api_key") {
		t.Error("HasSecret() should return false when not activated")
	}
}

func TestMarble_ListSecrets(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	secrets := m.ListSecrets()
	if len(secrets) != 0 {
		t.Errorf("ListSecrets() = %v, want empty", secrets)
	}
}

// =============================================================================
// Environment Access Tests
// =============================================================================

func TestMarble_GetEnv(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	if m.GetEnv("NONEXISTENT") != "" {
		t.Error("GetEnv() should return empty for non-existent key")
	}
}

func TestMarble_GetEnvWithDefault(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	val := m.GetEnvWithDefault("NONEXISTENT", "default-value")
	if val != "default-value" {
		t.Errorf("GetEnvWithDefault() = %s, want default-value", val)
	}
}

// =============================================================================
// TLS Tests
// =============================================================================

func TestMarble_TLSConfig(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)

	// Before activation, TLS config should be nil
	if m.TLSConfig() != nil {
		t.Error("TLSConfig() should be nil before activation")
	}
}

// =============================================================================
// Heartbeat Tests
// =============================================================================

func TestMarble_StartStopHeartbeat(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)
	ctx := context.Background()

	// Start heartbeat
	m.StartHeartbeat(ctx, 100*time.Millisecond)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Stop heartbeat
	m.StopHeartbeat()

	// Should not panic on double stop
	m.StopHeartbeat()
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestMarble_ConcurrentAccess(t *testing.T) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}

	m, _ := New(cfg)
	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.IsActivated()
			_ = m.MarbleType()
			_ = m.UUID()
			_ = m.Health()
		}()
	}

	wg.Wait()
}

// =============================================================================
// Supabase Client Tests
// =============================================================================

func TestNewSupabaseClient(t *testing.T) {
	client := NewSupabaseClient("http://localhost:54321", "test-key")
	if client == nil {
		t.Fatal("NewSupabaseClient() returned nil")
	}
}

func TestQueryBuilder_Select(t *testing.T) {
	client := NewSupabaseClient("http://localhost:54321", "test-key")
	qb := client.From("users").Select("id,name,email")

	if qb.columns != "id,name,email" {
		t.Errorf("columns = %s, want id,name,email", qb.columns)
	}
}

func TestQueryBuilder_Eq(t *testing.T) {
	client := NewSupabaseClient("http://localhost:54321", "test-key")
	qb := client.From("users").Eq("id", 123)

	if len(qb.filters) != 1 {
		t.Errorf("filters count = %d, want 1", len(qb.filters))
	}
	if qb.filters[0] != "id=eq.123" {
		t.Errorf("filter = %s, want id=eq.123", qb.filters[0])
	}
}

func TestQueryBuilder_Order(t *testing.T) {
	client := NewSupabaseClient("http://localhost:54321", "test-key")
	qb := client.From("users").Order("created_at", false)

	if qb.order != "created_at.desc" {
		t.Errorf("order = %s, want created_at.desc", qb.order)
	}
}

func TestQueryBuilder_Limit(t *testing.T) {
	client := NewSupabaseClient("http://localhost:54321", "test-key")
	qb := client.From("users").Limit(10)

	if qb.limit != 10 {
		t.Errorf("limit = %d, want 10", qb.limit)
	}
}

func TestQueryBuilder_Single(t *testing.T) {
	client := NewSupabaseClient("http://localhost:54321", "test-key")
	qb := client.From("users").Single()

	if !qb.single {
		t.Error("single should be true")
	}
}

func TestQueryBuilder_Chaining(t *testing.T) {
	client := NewSupabaseClient("http://localhost:54321", "test-key")
	qb := client.From("users").
		Select("id,name").
		Eq("status", "active").
		Neq("role", "admin").
		Order("name", true).
		Limit(20)

	if qb.columns != "id,name" {
		t.Errorf("columns = %s, want id,name", qb.columns)
	}
	if len(qb.filters) != 2 {
		t.Errorf("filters count = %d, want 2", len(qb.filters))
	}
	if qb.order != "name.asc" {
		t.Errorf("order = %s, want name.asc", qb.order)
	}
	if qb.limit != 20 {
		t.Errorf("limit = %d, want 20", qb.limit)
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkMarble_Health(b *testing.B) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}
	m, _ := New(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Health()
	}
}

func BenchmarkMarble_Seal(b *testing.B) {
	cfg := Config{
		MarbleType:     "test-marble",
		SimulationMode: true,
	}
	m, _ := New(cfg)
	data := []byte("benchmark data for sealing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Seal(data)
	}
}
