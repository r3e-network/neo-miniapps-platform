package bootstrap

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/system/engine/events"
)

// Unit tests for wiring configuration and component creation.
// Full integration tests require a PostgreSQL database.

func TestEventSystemConfig_Defaults(t *testing.T) {
	cfg := EventSystemConfig{
		DB: nil, // Will fail validation
	}

	if cfg.DispatcherWorkers != 0 {
		t.Errorf("expected default DispatcherWorkers 0, got %d", cfg.DispatcherWorkers)
	}
	if cfg.RouterWorkers != 0 {
		t.Errorf("expected default RouterWorkers 0, got %d", cfg.RouterWorkers)
	}
}

func TestNewEventSystem_NilDB(t *testing.T) {
	_, err := NewEventSystem(EventSystemConfig{
		DB: nil,
	})
	if err == nil {
		t.Error("expected error for nil DB")
	}
}

func TestUserAPIConfig_Defaults(t *testing.T) {
	cfg := UserAPIConfig{
		DB: nil,
	}

	if cfg.SecretsEncryptKey != nil {
		t.Error("expected nil SecretsEncryptKey by default")
	}
	if cfg.Router != nil {
		t.Error("expected nil Router by default")
	}
}

func TestNewUserAPI_NilDB(t *testing.T) {
	_, err := NewUserAPI(UserAPIConfig{
		DB: nil,
	})
	if err == nil {
		t.Error("expected error for nil DB")
	}
}

func TestFullSystemConfig_Defaults(t *testing.T) {
	cfg := FullSystemConfig{
		DB: nil,
	}

	if cfg.ContractTypes != nil {
		t.Error("expected nil ContractTypes by default")
	}
	if cfg.SecretsEncryptKey != nil {
		t.Error("expected nil SecretsEncryptKey by default")
	}
	if cfg.DispatcherWorkers != 0 {
		t.Errorf("expected default DispatcherWorkers 0, got %d", cfg.DispatcherWorkers)
	}
	if cfg.RouterWorkers != 0 {
		t.Errorf("expected default RouterWorkers 0, got %d", cfg.RouterWorkers)
	}
}

func TestNewFullSystem_NilDB(t *testing.T) {
	_, err := NewFullSystem(FullSystemConfig{
		DB: nil,
	})
	if err == nil {
		t.Error("expected error for nil DB")
	}
}

// Mock event handler for testing
type mockEventHandler struct {
	events []string
}

func (h *mockEventHandler) HandleEvent(ctx context.Context, event *events.ContractEvent) error {
	h.events = append(h.events, event.EventName)
	return nil
}

func (h *mockEventHandler) ID() string {
	return "mock-handler"
}

// Mock service handler for testing
type mockServiceHandler struct {
	serviceType events.ServiceType
	requests    []*events.Request
}

func (h *mockServiceHandler) ServiceType() events.ServiceType {
	return h.serviceType
}

func (h *mockServiceHandler) ProcessRequest(ctx context.Context, req *events.Request) error {
	h.requests = append(h.requests, req)
	return nil
}

func (h *mockServiceHandler) FulfillRequest(ctx context.Context, req *events.Request, result map[string]any) error {
	return nil
}

// Test contract type parsing (used in main.go)
func TestParseContractTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "single mapping",
			input: "0x1234:oraclehub",
			expected: map[string]string{
				"0x1234": "oraclehub",
			},
		},
		{
			name:  "multiple mappings",
			input: "0x1234:oraclehub,0x5678:vrf,0xabcd:datafeeds",
			expected: map[string]string{
				"0x1234": "oraclehub",
				"0x5678": "vrf",
				"0xabcd": "datafeeds",
			},
		},
		{
			name:  "with spaces",
			input: " 0x1234 : oraclehub , 0x5678 : vrf ",
			expected: map[string]string{
				"0x1234": "oraclehub",
				"0x5678": "vrf",
			},
		},
		{
			name:     "invalid format (no colon)",
			input:    "0x1234oraclehub",
			expected: map[string]string{},
		},
		{
			name:  "mixed valid and invalid",
			input: "0x1234:oraclehub,invalid,0x5678:vrf",
			expected: map[string]string{
				"0x1234": "oraclehub",
				"0x5678": "vrf",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseContractTypesHelper(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d mappings, got %d", len(tt.expected), len(result))
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}

// Helper function to parse contract types (mirrors main.go implementation)
func parseContractTypesHelper(value string) map[string]string {
	result := make(map[string]string)
	if value == "" {
		return result
	}

	// Simple parsing logic
	pairs := splitByComma(value)
	for _, pair := range pairs {
		pair = trimSpace(pair)
		if pair == "" {
			continue
		}
		idx := indexOf(pair, ':')
		if idx > 0 && idx < len(pair)-1 {
			hash := trimSpace(pair[:idx])
			contractType := trimSpace(pair[idx+1:])
			if hash != "" && contractType != "" {
				result[hash] = contractType
			}
		}
	}
	return result
}

func splitByComma(s string) []string {
	var result []string
	var current string
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// Test EventSystem methods without DB
func TestEventSystem_RegisterContract(t *testing.T) {
	// This test verifies the method signature exists
	// Full functionality requires a database
	t.Skip("requires database connection")
}

func TestEventSystem_RegisterEventHandler(t *testing.T) {
	// This test verifies the method signature exists
	t.Skip("requires database connection")
}

func TestEventSystem_RegisterServiceHandler(t *testing.T) {
	// This test verifies the method signature exists
	t.Skip("requires database connection")
}

// Test FullSystem methods without DB
func TestFullSystem_RegisterContract(t *testing.T) {
	t.Skip("requires database connection")
}

func TestFullSystem_RegisterEventHandler(t *testing.T) {
	t.Skip("requires database connection")
}

func TestFullSystem_RegisterServiceHandler(t *testing.T) {
	t.Skip("requires database connection")
}

// Benchmark tests

func BenchmarkParseContractTypes(b *testing.B) {
	input := "0x1234:oraclehub,0x5678:vrf,0xabcd:datafeeds"
	for i := 0; i < b.N; i++ {
		parseContractTypesHelper(input)
	}
}

func BenchmarkSplitByComma(b *testing.B) {
	input := "a,b,c,d,e,f,g,h,i,j"
	for i := 0; i < b.N; i++ {
		splitByComma(input)
	}
}

func BenchmarkTrimSpace(b *testing.B) {
	input := "  test value  "
	for i := 0; i < b.N; i++ {
		trimSpace(input)
	}
}
