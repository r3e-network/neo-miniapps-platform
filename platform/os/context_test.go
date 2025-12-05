// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/tee"
)

// TestNewServiceContext tests ServiceContext creation.
func TestNewServiceContext(t *testing.T) {
	// Create a test TrustRoot
	trustRoot, err := tee.NewSimulation("test-enclave")
	if err != nil {
		t.Fatalf("failed to create trust root: %v", err)
	}

	ctx := context.Background()
	if err := trustRoot.Start(ctx); err != nil {
		t.Fatalf("failed to start trust root: %v", err)
	}
	defer trustRoot.Stop(ctx)

	tests := []struct {
		name        string
		manifest    *Manifest
		trustRoot   *tee.TrustRoot
		expectError bool
	}{
		{
			name:        "nil manifest",
			manifest:    nil,
			trustRoot:   trustRoot,
			expectError: true,
		},
		{
			name: "nil trust root",
			manifest: &Manifest{
				ServiceID: "test-service",
			},
			trustRoot:   nil,
			expectError: true,
		},
		{
			name: "valid context",
			manifest: &Manifest{
				ServiceID:            "test-service",
				RequiredCapabilities: []Capability{CapSecrets, CapNetwork},
			},
			trustRoot:   trustRoot,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcCtx, err := NewServiceContext(tt.manifest, tt.trustRoot, nil)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if svcCtx == nil {
				t.Fatal("expected non-nil context")
			}
			if svcCtx.ServiceID() != tt.manifest.ServiceID {
				t.Errorf("expected service ID %s, got %s", tt.manifest.ServiceID, svcCtx.ServiceID())
			}
		})
	}
}

// TestServiceContextCapabilities tests capability checking.
func TestServiceContextCapabilities(t *testing.T) {
	trustRoot, err := tee.NewSimulation("test-enclave")
	if err != nil {
		t.Fatalf("failed to create trust root: %v", err)
	}

	ctx := context.Background()
	if err := trustRoot.Start(ctx); err != nil {
		t.Fatalf("failed to start trust root: %v", err)
	}
	defer trustRoot.Stop(ctx)

	manifest := &Manifest{
		ServiceID:            "test-service",
		RequiredCapabilities: []Capability{CapSecrets, CapNetwork},
		OptionalCapabilities: []Capability{CapKeys},
	}

	svcCtx, err := NewServiceContext(manifest, trustRoot, nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Test HasCapability
	tests := []struct {
		cap      Capability
		expected bool
	}{
		{CapSecrets, true},
		{CapNetwork, true},
		{CapKeys, true},
		{CapCompute, false},
		{CapDatabase, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.cap), func(t *testing.T) {
			if got := svcCtx.HasCapability(tt.cap); got != tt.expected {
				t.Errorf("HasCapability(%s) = %v, want %v", tt.cap, got, tt.expected)
			}
		})
	}

	// Test RequireCapability
	if err := svcCtx.RequireCapability(CapSecrets); err != nil {
		t.Errorf("RequireCapability(CapSecrets) should not error: %v", err)
	}
	if err := svcCtx.RequireCapability(CapCompute); err == nil {
		t.Error("RequireCapability(CapCompute) should error")
	}
}

// TestServiceContextAPIs tests that APIs are lazily initialized.
func TestServiceContextAPIs(t *testing.T) {
	trustRoot, err := tee.NewSimulation("test-enclave")
	if err != nil {
		t.Fatalf("failed to create trust root: %v", err)
	}

	ctx := context.Background()
	if err := trustRoot.Start(ctx); err != nil {
		t.Fatalf("failed to start trust root: %v", err)
	}
	defer trustRoot.Stop(ctx)

	manifest := &Manifest{
		ServiceID: "test-service",
		RequiredCapabilities: []Capability{
			CapSecrets, CapNetwork, CapKeys, CapCompute,
			CapStorage, CapEvents, CapAttestation, CapNeo,
			CapDatabase, CapChain, CapConfig, CapMetrics,
			CapScheduler, CapCache, CapAuth, CapQueue, CapLock,
			CapContract,
		},
	}

	svcCtx, err := NewServiceContext(manifest, trustRoot, nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Test that all APIs are accessible
	if svcCtx.Secrets() == nil {
		t.Error("Secrets() returned nil")
	}
	if svcCtx.Network() == nil {
		t.Error("Network() returned nil")
	}
	if svcCtx.Keys() == nil {
		t.Error("Keys() returned nil")
	}
	if svcCtx.Compute() == nil {
		t.Error("Compute() returned nil")
	}
	if svcCtx.Storage() == nil {
		t.Error("Storage() returned nil")
	}
	if svcCtx.Events() == nil {
		t.Error("Events() returned nil")
	}
	if svcCtx.Attestation() == nil {
		t.Error("Attestation() returned nil")
	}
	if svcCtx.Neo() == nil {
		t.Error("Neo() returned nil")
	}
	if svcCtx.Database() == nil {
		t.Error("Database() returned nil")
	}
	if svcCtx.Chain() == nil {
		t.Error("Chain() returned nil")
	}
	if svcCtx.Config() == nil {
		t.Error("Config() returned nil")
	}
	if svcCtx.Metrics() == nil {
		t.Error("Metrics() returned nil")
	}
	if svcCtx.Scheduler() == nil {
		t.Error("Scheduler() returned nil")
	}
	if svcCtx.Cache() == nil {
		t.Error("Cache() returned nil")
	}
	if svcCtx.Auth() == nil {
		t.Error("Auth() returned nil")
	}
	if svcCtx.Queue() == nil {
		t.Error("Queue() returned nil")
	}
	if svcCtx.Lock() == nil {
		t.Error("Lock() returned nil")
	}
	if svcCtx.Contract() == nil {
		t.Error("Contract() returned nil")
	}
}

// TestServiceContextClose tests context closing.
func TestServiceContextClose(t *testing.T) {
	trustRoot, err := tee.NewSimulation("test-enclave")
	if err != nil {
		t.Fatalf("failed to create trust root: %v", err)
	}

	ctx := context.Background()
	if err := trustRoot.Start(ctx); err != nil {
		t.Fatalf("failed to start trust root: %v", err)
	}
	defer trustRoot.Stop(ctx)

	manifest := &Manifest{
		ServiceID: "test-service",
	}

	svcCtx, err := NewServiceContext(manifest, trustRoot, nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Close should not error
	if err := svcCtx.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}

	// Context should be cancelled
	select {
	case <-svcCtx.Context().Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("context should be cancelled after Close()")
	}
}
