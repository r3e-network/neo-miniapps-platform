// Package ego provides EGo runtime integration tests.
package ego

import (
	"bytes"
	"sync"
	"testing"
)

// =============================================================================
// Runtime Tests
// =============================================================================

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := Config{
		SimulationMode:  true,
		ProductID:       42,
		SecurityVersion: 2,
		SealKeyPath:     "test_sealing.key",
	}

	r := NewWithConfig(cfg)
	if r == nil {
		t.Fatal("NewWithConfig() returned nil")
	}
	if r.productID != 42 {
		t.Errorf("productID = %d, want 42", r.productID)
	}
	if r.securityVersion != 2 {
		t.Errorf("securityVersion = %d, want 2", r.securityVersion)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ProductID != 1 {
		t.Errorf("ProductID = %d, want 1", cfg.ProductID)
	}
	if cfg.SecurityVersion != 1 {
		t.Errorf("SecurityVersion = %d, want 1", cfg.SecurityVersion)
	}
	if cfg.SealKeyPath != "sealing.key" {
		t.Errorf("SealKeyPath = %s, want sealing.key", cfg.SealKeyPath)
	}
}

func TestRuntime_InEnclave(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true})
	// In simulation mode, should not be in enclave
	if r.InEnclave() {
		t.Error("InEnclave() should return false in simulation mode")
	}
}

func TestRuntime_IsSimulation(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true})
	if !r.IsSimulation() {
		t.Error("IsSimulation() should return true when SimulationMode is true")
	}
}

// =============================================================================
// Sealing Tests
// =============================================================================

func TestRuntime_SealUnseal(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true, SealKeyPath: "test_seal.key"})

	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello world")},
		{"medium", bytes.Repeat([]byte("test"), 100)},
		{"large", bytes.Repeat([]byte("x"), 10000)},
		{"binary", []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sealed, err := r.Seal(tc.data, SealPolicyProduct)
			if err != nil {
				t.Fatalf("Seal() error: %v", err)
			}

			if len(sealed) == 0 && len(tc.data) > 0 {
				t.Error("Seal() returned empty data for non-empty input")
			}

			unsealed, err := r.Unseal(sealed)
			if err != nil {
				t.Fatalf("Unseal() error: %v", err)
			}

			if !bytes.Equal(unsealed, tc.data) {
				t.Errorf("Unseal() = %v, want %v", unsealed, tc.data)
			}
		})
	}
}

func TestRuntime_SealPolicies(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true, SealKeyPath: "test_seal.key"})
	data := []byte("test data for sealing")

	policies := []SealPolicy{SealPolicyUnique, SealPolicyProduct}

	for _, policy := range policies {
		t.Run(policy.String(), func(t *testing.T) {
			sealed, err := r.Seal(data, policy)
			if err != nil {
				t.Fatalf("Seal() with policy %s error: %v", policy, err)
			}

			unsealed, err := r.Unseal(sealed)
			if err != nil {
				t.Fatalf("Unseal() error: %v", err)
			}

			if !bytes.Equal(unsealed, data) {
				t.Error("Unsealed data doesn't match original")
			}
		})
	}
}

func TestRuntime_UnsealInvalidData(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true, SealKeyPath: "test_seal.key"})

	testCases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too_short", []byte("short")},
		{"random", []byte("this is not valid sealed data at all")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Unseal(tc.data)
			if err == nil {
				t.Error("Unseal() should error on invalid data")
			}
		})
	}
}

func TestRuntime_SealConcurrent(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true, SealKeyPath: "test_seal.key"})

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := []byte("concurrent test data")
			sealed, err := r.Seal(data, SealPolicyProduct)
			if err != nil {
				errors <- err
				return
			}
			unsealed, err := r.Unseal(sealed)
			if err != nil {
				errors <- err
				return
			}
			if !bytes.Equal(unsealed, data) {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent seal/unseal error: %v", err)
		}
	}
}

// =============================================================================
// Quote Tests
// =============================================================================

func TestRuntime_GenerateQuote(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true})

	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("report data")},
		{"64_bytes", bytes.Repeat([]byte("x"), 64)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			quote, err := r.GenerateQuote(tc.data)
			if err != nil {
				t.Fatalf("GenerateQuote() error: %v", err)
			}

			if len(quote) == 0 {
				t.Error("GenerateQuote() returned empty quote")
			}
		})
	}
}

func TestRuntime_VerifyQuote(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true})

	data := []byte("test report data")
	quote, err := r.GenerateQuote(data)
	if err != nil {
		t.Fatalf("GenerateQuote() error: %v", err)
	}

	report, err := r.VerifyQuote(quote)
	if err != nil {
		t.Fatalf("VerifyQuote() error: %v", err)
	}

	if report == nil {
		t.Error("VerifyQuote() returned nil report")
	}
}

func TestRuntime_VerifyQuoteInvalid(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true})

	testCases := []struct {
		name  string
		quote []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"random", []byte("not a valid quote")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.VerifyQuote(tc.quote)
			if err == nil {
				t.Error("VerifyQuote() should error on invalid quote")
			}
		})
	}
}

// =============================================================================
// Attestation Tests
// =============================================================================

func TestRuntime_GetAttestationInfo(t *testing.T) {
	r := NewWithConfig(Config{
		SimulationMode:  true,
		ProductID:       42,
		SecurityVersion: 3,
	})

	info, err := r.GetAttestationInfo()
	if err != nil {
		t.Fatalf("GetAttestationInfo() error: %v", err)
	}

	if info == nil {
		t.Fatal("GetAttestationInfo() returned nil")
	}

	if info.ProductID != 42 {
		t.Errorf("ProductID = %d, want 42", info.ProductID)
	}
	if info.SecurityVersion != 3 {
		t.Errorf("SecurityVersion = %d, want 3", info.SecurityVersion)
	}
	if !info.SimulationMode {
		t.Error("SimulationMode should be true")
	}
}

func TestRuntime_GetReport(t *testing.T) {
	r := NewWithConfig(Config{SimulationMode: true})

	data := []byte("report data")
	report, err := r.GetReport(data)
	if err != nil {
		t.Fatalf("GetReport() error: %v", err)
	}

	if report == nil {
		t.Fatal("GetReport() returned nil")
	}
}

// =============================================================================
// SealPolicy Tests
// =============================================================================

func TestSealPolicy_String(t *testing.T) {
	testCases := []struct {
		policy SealPolicy
		want   string
	}{
		{SealPolicyUnique, "unique"},
		{SealPolicyProduct, "product"},
		{SealPolicy(99), "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.policy.String(); got != tc.want {
				t.Errorf("String() = %s, want %s", got, tc.want)
			}
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkSeal(b *testing.B) {
	r := NewWithConfig(Config{SimulationMode: true, SealKeyPath: "bench_seal.key"})
	data := bytes.Repeat([]byte("benchmark data"), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Seal(data, SealPolicyProduct)
	}
}

func BenchmarkUnseal(b *testing.B) {
	r := NewWithConfig(Config{SimulationMode: true, SealKeyPath: "bench_seal.key"})
	data := bytes.Repeat([]byte("benchmark data"), 100)
	sealed, _ := r.Seal(data, SealPolicyProduct)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Unseal(sealed)
	}
}

func BenchmarkGenerateQuote(b *testing.B) {
	r := NewWithConfig(Config{SimulationMode: true})
	data := []byte("benchmark report data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.GenerateQuote(data)
	}
}
