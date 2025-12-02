package service

import "strings"

// NormalizeMetadata standardizes metadata maps (trim keys/values, lower keys).
func NormalizeMetadata(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	return out
}

// NormalizeTags normalizes tag/signer slices with trimming, lower-casing, and de-duping.
func NormalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := strings.ToLower(strings.TrimSpace(tag))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// CloneAnyMap performs a shallow copy of a map[string]any.
func CloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// ContainsCaseInsensitive checks if a string slice contains a target (case-insensitive).
func ContainsCaseInsensitive(list []string, target string) bool {
	for _, item := range list {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

// TrimAndValidate trims a string and validates it's not empty.
// Deprecated: Use NormalizeRequired instead for consistency.
func TrimAndValidate(value, fieldName string) (string, error) {
	return NormalizeRequired(value, fieldName)
}

// TrimOrDefault trims a string and returns a default if empty.
func TrimOrDefault(value, defaultValue string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

// ExtractMetadataRaw extracts a metadata map from request body without normalization.
// Keys and values are preserved exactly as provided.
// Use this when metadata keys are case-sensitive or must be preserved verbatim.
func ExtractMetadataRaw(body map[string]any, key string) map[string]string {
	if key == "" {
		key = "metadata"
	}
	rawMeta, ok := body[key].(map[string]any)
	if !ok || len(rawMeta) == 0 {
		return nil
	}
	result := make(map[string]string, len(rawMeta))
	for k, v := range rawMeta {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

// ExtractMetadata extracts and normalizes a metadata map from request body.
// Keys are lowercased and trimmed. Use ExtractMetadataRaw to preserve original keys.
func ExtractMetadata(body map[string]any, key string) map[string]string {
	raw := ExtractMetadataRaw(body, key)
	if raw == nil {
		return nil
	}
	return NormalizeMetadata(raw)
}

// ExtractStringSlice extracts a string slice from request body.
// Handles both []string and []any (from JSON unmarshaling).
func ExtractStringSlice(body map[string]any, key string) []string {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil
	}

	// Direct []string (rare but possible)
	if slice, ok := raw.([]string); ok {
		return slice
	}

	// []any from JSON unmarshaling (common case)
	if slice, ok := raw.([]any); ok {
		result := make([]string, 0, len(slice))
		for _, item := range slice {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	return nil
}

// ExtractString extracts a string value from request body with optional default.
func ExtractString(body map[string]any, key string, defaultValue string) string {
	if val, ok := body[key].(string); ok {
		return strings.TrimSpace(val)
	}
	return defaultValue
}

// ExtractInt extracts an integer value from request body.
// Handles both int and float64 (from JSON unmarshaling).
func ExtractInt(body map[string]any, key string, defaultValue int) int {
	raw, ok := body[key]
	if !ok {
		return defaultValue
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return defaultValue
}

// ExtractFloat extracts a float64 value from request body.
func ExtractFloat(body map[string]any, key string, defaultValue float64) float64 {
	raw, ok := body[key]
	if !ok {
		return defaultValue
	}
	switch v := raw.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return defaultValue
}

// ExtractBool extracts a boolean value from request body.
func ExtractBool(body map[string]any, key string, defaultValue bool) bool {
	if val, ok := body[key].(bool); ok {
		return val
	}
	return defaultValue
}

// NormalizeRequired trims a string and returns an error if empty.
// This consolidates the repeated pattern across services:
//
//	value = strings.TrimSpace(value)
//	if value == "" { return ..., fmt.Errorf("field required") }
//
// Usage: accountID, err := core.NormalizeRequired(accountID, "account_id")
func NormalizeRequired(value, fieldName string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", RequiredError(fieldName)
	}
	return trimmed, nil
}

// ValidateRequired checks that all provided fields are non-empty after trimming.
// Returns RequiredError for the first empty field found.
// Usage: err := core.ValidateRequired(map[string]string{"account_id": id, "name": name})
func ValidateRequired(fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return RequiredError(name)
		}
	}
	return nil
}

// ValidateRequiredFields checks multiple fields and returns error for first empty one.
// More efficient than ValidateRequired when field order matters.
// Usage: err := core.ValidateRequiredFields(accountID, "account_id", name, "name")
func ValidateRequiredFields(pairs ...string) error {
	if len(pairs)%2 != 0 {
		return NewValidationError("", "invalid field pairs")
	}
	for i := 0; i < len(pairs); i += 2 {
		value := pairs[i]
		name := pairs[i+1]
		if strings.TrimSpace(value) == "" {
			return RequiredError(name)
		}
	}
	return nil
}

// NormalizeAndValidateAccount is a convenience function for the common pattern
// of validating and normalizing an account ID.
func NormalizeAndValidateAccount(accountID string) (string, error) {
	return NormalizeRequired(accountID, "account_id")
}
