package chain

import (
	"os"
	"testing"
)

func TestContractAddressesLoadFromEnv(t *testing.T) {
	// Save and restore environment
	envVars := []string{
		"CONTRACT_PAYMENTHUB_HASH",
		"CONTRACT_PAYMENT_HUB_HASH",
		"CONTRACT_GOVERNANCE_HASH",
		"CONTRACT_PRICEFEED_HASH",
		"CONTRACT_PRICE_FEED_HASH",
		"CONTRACT_RANDOMNESSLOG_HASH",
		"CONTRACT_RANDOMNESS_LOG_HASH",
		"CONTRACT_APPREGISTRY_HASH",
		"CONTRACT_APP_REGISTRY_HASH",
		"CONTRACT_AUTOMATIONANCHOR_HASH",
		"CONTRACT_AUTOMATION_ANCHOR_HASH",
	}
	saved := make(map[string]string)
	for _, k := range envVars {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	t.Run("primary env vars", func(t *testing.T) {
		os.Setenv("CONTRACT_PAYMENTHUB_HASH", "0xpayment")
		os.Setenv("CONTRACT_GOVERNANCE_HASH", "0xgov")
		os.Setenv("CONTRACT_PRICEFEED_HASH", "0xprice")
		os.Setenv("CONTRACT_RANDOMNESSLOG_HASH", "0xrandom")
		os.Setenv("CONTRACT_APPREGISTRY_HASH", "0xapp")
		os.Setenv("CONTRACT_AUTOMATIONANCHOR_HASH", "0xauto")
		defer func() {
			for _, k := range envVars {
				os.Unsetenv(k)
			}
		}()

		c := ContractAddresses{}
		c.LoadFromEnv()

		if c.PaymentHub != "0xpayment" {
			t.Errorf("PaymentHub = %s, want 0xpayment", c.PaymentHub)
		}
		if c.Governance != "0xgov" {
			t.Errorf("Governance = %s, want 0xgov", c.Governance)
		}
		if c.PriceFeed != "0xprice" {
			t.Errorf("PriceFeed = %s, want 0xprice", c.PriceFeed)
		}
		if c.RandomnessLog != "0xrandom" {
			t.Errorf("RandomnessLog = %s, want 0xrandom", c.RandomnessLog)
		}
		if c.AppRegistry != "0xapp" {
			t.Errorf("AppRegistry = %s, want 0xapp", c.AppRegistry)
		}
		if c.AutomationAnchor != "0xauto" {
			t.Errorf("AutomationAnchor = %s, want 0xauto", c.AutomationAnchor)
		}
	})

	t.Run("fallback env vars", func(t *testing.T) {
		os.Setenv("CONTRACT_PAYMENT_HUB_HASH", "0xpayment2")
		os.Setenv("CONTRACT_PRICE_FEED_HASH", "0xprice2")
		os.Setenv("CONTRACT_RANDOMNESS_LOG_HASH", "0xrandom2")
		os.Setenv("CONTRACT_APP_REGISTRY_HASH", "0xapp2")
		os.Setenv("CONTRACT_AUTOMATION_ANCHOR_HASH", "0xauto2")
		defer func() {
			for _, k := range envVars {
				os.Unsetenv(k)
			}
		}()

		c := ContractAddresses{}
		c.LoadFromEnv()

		if c.PaymentHub != "0xpayment2" {
			t.Errorf("PaymentHub = %s, want 0xpayment2", c.PaymentHub)
		}
		if c.PriceFeed != "0xprice2" {
			t.Errorf("PriceFeed = %s, want 0xprice2", c.PriceFeed)
		}
		if c.RandomnessLog != "0xrandom2" {
			t.Errorf("RandomnessLog = %s, want 0xrandom2", c.RandomnessLog)
		}
		if c.AppRegistry != "0xapp2" {
			t.Errorf("AppRegistry = %s, want 0xapp2", c.AppRegistry)
		}
		if c.AutomationAnchor != "0xauto2" {
			t.Errorf("AutomationAnchor = %s, want 0xauto2", c.AutomationAnchor)
		}
	})

	t.Run("empty env vars", func(t *testing.T) {
		c := ContractAddresses{}
		c.LoadFromEnv()

		if c.PaymentHub != "" {
			t.Errorf("PaymentHub should be empty, got %s", c.PaymentHub)
		}
		if c.Governance != "" {
			t.Errorf("Governance should be empty, got %s", c.Governance)
		}
	})
}

func TestContractAddressesFromEnv(t *testing.T) {
	os.Setenv("CONTRACT_GOVERNANCE_HASH", "0xtest")
	defer os.Unsetenv("CONTRACT_GOVERNANCE_HASH")

	c := ContractAddressesFromEnv()
	if c.Governance != "0xtest" {
		t.Errorf("Governance = %s, want 0xtest", c.Governance)
	}
}

func TestIsHaltState(t *testing.T) {
	tests := []struct {
		state    string
		expected bool
	}{
		{"HALT", true},
		{"HALT, BREAK", true},
		{" HALT", true},
		{"FAULT", false},
		{"", false},
		{"halt", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := isHaltState(tt.state); got != tt.expected {
				t.Errorf("isHaltState(%q) = %v, want %v", tt.state, got, tt.expected)
			}
		})
	}
}

func TestRequireHalt(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		err := requireHalt("test", nil)
		if err == nil {
			t.Error("expected error for nil result")
		}
	})

	t.Run("HALT state", func(t *testing.T) {
		result := &InvokeResult{State: "HALT"}
		err := requireHalt("test", result)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("FAULT with exception", func(t *testing.T) {
		result := &InvokeResult{State: "FAULT", Exception: "test error"}
		err := requireHalt("test", result)
		if err == nil {
			t.Error("expected error for FAULT state")
		}
		if err.Error() != "test: execution failed (FAULT): test error" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("FAULT without exception", func(t *testing.T) {
		result := &InvokeResult{State: "FAULT"}
		err := requireHalt("test", result)
		if err == nil {
			t.Error("expected error for FAULT state")
		}
	})
}

func TestRequireStack(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		err := requireStack("test", nil)
		if err == nil {
			t.Error("expected error for nil result")
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		result := &InvokeResult{Stack: []StackItem{}}
		err := requireStack("test", result)
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})

	t.Run("non-empty stack", func(t *testing.T) {
		result := &InvokeResult{Stack: []StackItem{{Type: "Integer"}}}
		err := requireStack("test", result)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestFirstStackItem(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		_, err := firstStackItem("test", nil)
		if err == nil {
			t.Error("expected error for nil result")
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		result := &InvokeResult{Stack: []StackItem{}}
		_, err := firstStackItem("test", result)
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})

	t.Run("success", func(t *testing.T) {
		expected := StackItem{Type: "Integer"}
		result := &InvokeResult{Stack: []StackItem{expected, {Type: "Boolean"}}}
		item, err := firstStackItem("test", result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.Type != expected.Type {
			t.Errorf("item.Type = %s, want %s", item.Type, expected.Type)
		}
	})
}

func TestFirstNonEmptyEnv(t *testing.T) {
	os.Setenv("TEST_ENV_A", "")
	os.Setenv("TEST_ENV_B", "value_b")
	os.Setenv("TEST_ENV_C", "value_c")
	defer func() {
		os.Unsetenv("TEST_ENV_A")
		os.Unsetenv("TEST_ENV_B")
		os.Unsetenv("TEST_ENV_C")
	}()

	t.Run("first non-empty", func(t *testing.T) {
		result := firstNonEmptyEnv("TEST_ENV_A", "TEST_ENV_B", "TEST_ENV_C")
		if result != "value_b" {
			t.Errorf("result = %s, want value_b", result)
		}
	})

	t.Run("all empty", func(t *testing.T) {
		result := firstNonEmptyEnv("TEST_ENV_A", "NONEXISTENT")
		if result != "" {
			t.Errorf("result = %s, want empty", result)
		}
	})

	t.Run("no keys", func(t *testing.T) {
		result := firstNonEmptyEnv()
		if result != "" {
			t.Errorf("result = %s, want empty", result)
		}
	})
}
