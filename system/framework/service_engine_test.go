package framework

import (
	"context"
	"errors"
	"testing"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
)

type mockAccountChecker struct {
	accounts map[string]bool
	tenants  map[string]string
}

func (m *mockAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	if m.accounts[accountID] {
		return nil
	}
	return errors.New("account not found")
}

func (m *mockAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	if m.tenants != nil {
		return m.tenants[accountID]
	}
	return ""
}

type mockWalletChecker struct {
	wallets map[string]map[string]bool
}

func (m *mockWalletChecker) WalletOwnedBy(ctx context.Context, accountID, wallet string) error {
	if wallets, ok := m.wallets[accountID]; ok {
		if wallets[wallet] {
			return nil
		}
	}
	return errors.New("wallet not owned by account")
}

func TestNewServiceEngine(t *testing.T) {
	accounts := &mockAccountChecker{accounts: map[string]bool{"acct-1": true}}

	eng := NewServiceEngine(ServiceConfig{
		Name:        "testservice",
		Description: "Test service",
		Accounts:    accounts,
	})

	if eng.Name() != "testservice" {
		t.Errorf("Name() = %q, want %q", eng.Name(), "testservice")
	}
	if eng.Domain() != "testservice" {
		t.Errorf("Domain() = %q, want %q", eng.Domain(), "testservice")
	}

	manifest := eng.Manifest()
	if manifest.Name != "testservice" {
		t.Errorf("Manifest.Name = %q, want %q", manifest.Name, "testservice")
	}
	if manifest.Description != "Test service" {
		t.Errorf("Manifest.Description = %q, want %q", manifest.Description, "Test service")
	}
}

func TestServiceEngine_CustomDomain(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:        "testservice",
		Domain:      "custom-domain",
		Description: "Test service",
	})

	if eng.Domain() != "custom-domain" {
		t.Errorf("Domain() = %q, want %q", eng.Domain(), "custom-domain")
	}
}

func TestServiceEngine_CustomAPIs(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:         "testservice",
		Description:  "Test service",
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore, engine.APISurfaceData},
	})

	manifest := eng.Manifest()
	if len(manifest.RequiresAPIs) != 2 {
		t.Errorf("RequiresAPIs length = %d, want 2", len(manifest.RequiresAPIs))
	}
}

func TestServiceEngine_ValidateAccount(t *testing.T) {
	accounts := &mockAccountChecker{accounts: map[string]bool{"acct-1": true}}
	eng := NewServiceEngine(ServiceConfig{
		Name:     "testservice",
		Accounts: accounts,
	})

	tests := []struct {
		name      string
		accountID string
		wantID    string
		wantErr   bool
	}{
		{"valid account", "acct-1", "acct-1", false},
		{"valid with spaces", "  acct-1  ", "acct-1", false},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"not found", "acct-2", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eng.ValidateAccount(context.Background(), tt.accountID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantID {
				t.Errorf("ValidateAccount() = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestServiceEngine_ValidateAccount_NilAccounts(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name: "testservice",
	})

	// Should still validate empty
	_, err := eng.ValidateAccount(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty account ID")
	}

	// Should pass for non-empty when accounts is nil
	got, err := eng.ValidateAccount(context.Background(), "any-account")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got != "any-account" {
		t.Errorf("expected 'any-account', got %q", got)
	}
}

func TestServiceEngine_ValidateOwnership(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	tests := []struct {
		name            string
		resourceAccount string
		requestAccount  string
		wantErr         bool
	}{
		{"same account", "acct-1", "acct-1", false},
		{"different account", "acct-1", "acct-2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eng.ValidateOwnership(tt.resourceAccount, tt.requestAccount, "resource", "res-1")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOwnership() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceEngine_ValidateAccountAndOwnership(t *testing.T) {
	accounts := &mockAccountChecker{accounts: map[string]bool{"acct-1": true, "acct-2": true}}
	eng := NewServiceEngine(ServiceConfig{
		Name:     "testservice",
		Accounts: accounts,
	})

	tests := []struct {
		name            string
		resourceAccount string
		requestAccount  string
		wantErr         bool
	}{
		{"valid and owns", "acct-1", "acct-1", false},
		{"valid but doesn't own", "acct-1", "acct-2", true},
		{"invalid requester", "acct-1", "acct-3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eng.ValidateAccountAndOwnership(context.Background(), tt.resourceAccount, tt.requestAccount, "resource", "res-1")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccountAndOwnership() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceEngine_ValidateSigners(t *testing.T) {
	wallets := &mockWalletChecker{
		wallets: map[string]map[string]bool{
			"acct-1": {"wallet-a": true, "wallet-b": true},
		},
	}
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})
	eng.WithWalletChecker(wallets)

	tests := []struct {
		name      string
		accountID string
		signers   []string
		wantErr   bool
	}{
		{"valid signers", "acct-1", []string{"wallet-a", "wallet-b"}, false},
		{"one invalid", "acct-1", []string{"wallet-a", "wallet-c"}, true},
		{"empty signers", "acct-1", []string{}, false},
		{"nil signers", "acct-1", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eng.ValidateSigners(context.Background(), tt.accountID, tt.signers)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSigners() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceEngine_ValidateRequired(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	tests := []struct {
		name    string
		value   string
		field   string
		want    string
		wantErr bool
	}{
		{"valid", "hello", "name", "hello", false},
		{"with spaces", "  hello  ", "name", "hello", false},
		{"empty", "", "name", "", true},
		{"whitespace", "   ", "name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eng.ValidateRequired(tt.value, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequired() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceEngine_ClampLimit(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"zero uses default", 0, 25},    // DefaultListLimit = 25
		{"negative uses default", -1, 25},
		{"within range", 25, 25},
		{"above max", 1000, 500}, // MaxListLimit = 500
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eng.ClampLimit(tt.limit)
			if got != tt.want {
				t.Errorf("ClampLimit(%d) = %d, want %d", tt.limit, got, tt.want)
			}
		})
	}
}

func TestServiceEngine_Descriptor(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name:        "testservice",
		Description: "Test service",
	})

	desc := eng.Descriptor()
	if desc.Name != "testservice" {
		t.Errorf("Descriptor.Name = %q, want %q", desc.Name, "testservice")
	}
}

func TestServiceEngine_Logger(t *testing.T) {
	customLog := logger.NewDefault("custom")
	eng := NewServiceEngine(ServiceConfig{
		Name:   "testservice",
		Logger: customLog,
	})

	if eng.Logger() != customLog {
		t.Error("Logger() did not return the custom logger")
	}
}

func TestServiceEngine_DefaultLogger(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{
		Name: "testservice",
	})

	if eng.Logger() == nil {
		t.Error("Logger() should not be nil")
	}
}

func TestServiceEngine_Lifecycle(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	if err := eng.Start(context.Background()); err != nil {
		t.Errorf("Start() error = %v", err)
	}
	if err := eng.Ready(context.Background()); err != nil {
		t.Errorf("Ready() error = %v", err)
	}
	if err := eng.Stop(context.Background()); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
	if eng.Ready(context.Background()) == nil {
		t.Error("Ready() should return error after Stop()")
	}
}

func TestServiceEngine_NormalizeMetadata(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	input := map[string]string{
		"  key1  ": "  value1  ",
		"key2":     "value2",
		"":         "empty-key",
	}

	result := eng.NormalizeMetadata(input)
	if result["key1"] != "value1" {
		t.Errorf("expected trimmed key and value")
	}
	if _, ok := result[""]; ok {
		t.Error("empty key should be removed")
	}
}

func TestServiceEngine_NormalizeTags(t *testing.T) {
	eng := NewServiceEngine(ServiceConfig{Name: "testservice"})

	input := []string{"  tag1  ", "TAG1", "tag2", ""}
	result := eng.NormalizeTags(input)

	if len(result) != 2 {
		t.Errorf("expected 2 tags after dedup, got %d", len(result))
	}
}
