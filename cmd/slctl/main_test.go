package main

import (
    "os"
    "path/filepath"
    "reflect"
    "testing"
)

func TestParseKeyValue(t *testing.T) {
	values, err := parseKeyValue("foo=bar,baz=qux")
	if err != nil {
		t.Fatalf("parseKeyValue returned error: %v", err)
	}
	expected := map[string]string{"foo": "bar", "baz": "qux"}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf("expected %v, got %v", expected, values)
	}

	if _, err := parseKeyValue("invalid"); err == nil {
		t.Fatalf("expected error for missing '='")
	}
}

func TestSplitCommaList(t *testing.T) {
	result := splitCommaList(" a , b ,c ")
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}

	if res := splitCommaList("   "); res != nil {
		t.Fatalf("expected nil for blank input, got %v", res)
	}
}

func TestLoadJSONPayload(t *testing.T) {
	inline := `{"number":42,"nested":{"key":"value"}}`
	payload, err := loadJSONPayload(inline, "")
	if err != nil {
		t.Fatalf("loadJSONPayload inline returned error: %v", err)
	}
	payloadMap, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", payload)
	}
	nested, ok := payloadMap["nested"].(map[string]any)
	if !ok || nested["key"] != "value" {
		t.Fatalf("unexpected payload: %v", payloadMap)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(path, []byte(`{"hello":"file"}`), 0o600); err != nil {
		t.Fatalf("write payload file: %v", err)
	}
	filePayload, err := loadJSONPayload("", path)
	if err != nil {
		t.Fatalf("loadJSONPayload file returned error: %v", err)
	}
	fileMap, ok := filePayload.(map[string]any)
	if !ok || fileMap["hello"] != "file" {
		t.Fatalf("unexpected file payload: %v", filePayload)
	}

	if _, err := loadJSONPayload(inline, path); err == nil {
		t.Fatalf("expected error when both inline and file are provided")
	}
}
