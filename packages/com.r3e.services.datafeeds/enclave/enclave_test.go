package enclave

import (
	"testing"
)

func TestExtractValue_SimpleField(t *testing.T) {
	body := []byte(`{"price": 42000.50}`)
	value, err := extractValue(body, "$.price")
	if err != nil {
		t.Fatalf("extractValue failed: %v", err)
	}
	if value != "42000.5" {
		t.Errorf("Expected '42000.5', got '%s'", value)
	}
}

func TestExtractValue_NestedField(t *testing.T) {
	body := []byte(`{"data": {"result": {"value": 123.45}}}`)
	value, err := extractValue(body, "$.data.result.value")
	if err != nil {
		t.Fatalf("extractValue failed: %v", err)
	}
	if value != "123.45" {
		t.Errorf("Expected '123.45', got '%s'", value)
	}
}

func TestExtractValue_ArrayIndex(t *testing.T) {
	body := []byte(`{"prices": [100, 200, 300]}`)
	value, err := extractValue(body, "$.prices[1]")
	if err != nil {
		t.Fatalf("extractValue failed: %v", err)
	}
	if value != "200" {
		t.Errorf("Expected '200', got '%s'", value)
	}
}

func TestExtractValue_StringValue(t *testing.T) {
	body := []byte(`{"status": "active"}`)
	value, err := extractValue(body, "$.status")
	if err != nil {
		t.Fatalf("extractValue failed: %v", err)
	}
	if value != "active" {
		t.Errorf("Expected 'active', got '%s'", value)
	}
}

func TestExtractValue_NoPath(t *testing.T) {
	body := []byte(`plain text response`)
	value, err := extractValue(body, "")
	if err != nil {
		t.Fatalf("extractValue failed: %v", err)
	}
	if value != "plain text response" {
		t.Errorf("Expected 'plain text response', got '%s'", value)
	}
}

func TestExtractValue_InvalidJSON(t *testing.T) {
	body := []byte(`not json`)
	_, err := extractValue(body, "$.field")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestExtractValue_FieldNotFound(t *testing.T) {
	body := []byte(`{"other": 123}`)
	_, err := extractValue(body, "$.missing")
	if err == nil {
		t.Error("Expected error for missing field")
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"field", []string{"field"}},
		{"a.b.c", []string{"a", "b", "c"}},
		{"arr[0]", []string{"arr", "[0]"}},
		{"data.items[2].value", []string{"data", "items", "[2]", "value"}},
	}

	for _, tc := range tests {
		result := splitPath(tc.path)
		if len(result) != len(tc.expected) {
			t.Errorf("splitPath(%s): expected %v, got %v", tc.path, tc.expected, result)
			continue
		}
		for i, v := range result {
			if v != tc.expected[i] {
				t.Errorf("splitPath(%s)[%d]: expected '%s', got '%s'", tc.path, i, tc.expected[i], v)
			}
		}
	}
}

func TestPow10(t *testing.T) {
	tests := []struct {
		n        int
		expected int
	}{
		{0, 1},
		{1, 10},
		{2, 100},
		{3, 1000},
		{8, 100000000},
	}

	for _, tc := range tests {
		result := pow10(tc.n)
		if result != tc.expected {
			t.Errorf("pow10(%d): expected %d, got %d", tc.n, tc.expected, result)
		}
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value    float64
		decimals int
		expected string
	}{
		{42000.5, 2, "42000.50"},
		{123.456789, 4, "123.4568"},
		{100.0, 0, "100"},
		{0.123456, 8, "0.12345600"},
	}

	for _, tc := range tests {
		result := formatValue(tc.value, tc.decimals)
		if result != tc.expected {
			t.Errorf("formatValue(%f, %d): expected '%s', got '%s'", tc.value, tc.decimals, tc.expected, result)
		}
	}
}

// Tests for the aggregate function (pure function, no enclave instance needed)
func TestAggregate_Median(t *testing.T) {
	// Odd number of values
	values := []float64{100, 200, 300, 400, 500}
	result := aggregate(values, "median")
	if result != 300 {
		t.Errorf("Expected median 300, got %f", result)
	}

	// Even number of values
	values = []float64{100, 200, 300, 400}
	result = aggregate(values, "median")
	if result != 250 {
		t.Errorf("Expected median 250, got %f", result)
	}
}

func TestAggregate_Mean(t *testing.T) {
	values := []float64{100, 200, 300}
	result := aggregate(values, "mean")
	if result != 200 {
		t.Errorf("Expected mean 200, got %f", result)
	}
}

func TestAggregate_Average(t *testing.T) {
	values := []float64{100, 200, 300}
	result := aggregate(values, "average")
	if result != 200 {
		t.Errorf("Expected average 200, got %f", result)
	}
}

func TestAggregate_Min(t *testing.T) {
	values := []float64{300, 100, 200}
	result := aggregate(values, "min")
	if result != 100 {
		t.Errorf("Expected min 100, got %f", result)
	}
}

func TestAggregate_Max(t *testing.T) {
	values := []float64{100, 300, 200}
	result := aggregate(values, "max")
	if result != 300 {
		t.Errorf("Expected max 300, got %f", result)
	}
}

func TestAggregate_Empty(t *testing.T) {
	values := []float64{}
	result := aggregate(values, "median")
	if result != 0 {
		t.Errorf("Expected 0 for empty values, got %f", result)
	}
}

func TestAggregate_Default(t *testing.T) {
	// Unknown method should default to median
	values := []float64{100, 200, 300, 400, 500}
	result := aggregate(values, "unknown")
	if result != 300 {
		t.Errorf("Expected default (median) 300, got %f", result)
	}
}

// Tests for calculateConfidence function
func TestCalculateConfidence_SingleValue(t *testing.T) {
	values := []float64{100}
	result := calculateConfidence(values, 100)
	if result != 1.0 {
		t.Errorf("Expected confidence 1.0 for single value, got %f", result)
	}
}

func TestCalculateConfidence_IdenticalValues(t *testing.T) {
	values := []float64{100, 100, 100}
	result := calculateConfidence(values, 100)
	if result != 1.0 {
		t.Errorf("Expected confidence 1.0 for identical values, got %f", result)
	}
}

func TestCalculateConfidence_DifferentValues(t *testing.T) {
	values := []float64{90, 100, 110}
	result := calculateConfidence(values, 100)
	// Should be less than 1.0 due to variance
	if result >= 1.0 || result < 0 {
		t.Errorf("Expected confidence between 0 and 1, got %f", result)
	}
}

func TestCalculateConfidence_ZeroAggregated(t *testing.T) {
	values := []float64{0, 0, 0}
	result := calculateConfidence(values, 0)
	if result != 1.0 {
		t.Errorf("Expected confidence 1.0 for zero aggregated, got %f", result)
	}
}

// Tests for toBigInt function
func TestToBigInt(t *testing.T) {
	tests := []struct {
		value    float64
		decimals int
		expected string
	}{
		{100.0, 8, "10000000000"},
		{1.5, 2, "150"},
		{0.001, 3, "1"},
		{42000.50, 2, "4200050"},
	}

	for _, tc := range tests {
		result := toBigInt(tc.value, tc.decimals)
		if result.String() != tc.expected {
			t.Errorf("toBigInt(%f, %d): expected %s, got %s", tc.value, tc.decimals, tc.expected, result.String())
		}
	}
}
