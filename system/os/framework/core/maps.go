package service

// Map helper functions for consistent map access.
// These consolidate duplicate implementations across service packages.

// GetString safely retrieves a string value from a map[string]any.
// Returns empty string if key doesn't exist or value is not a string.
func GetString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt safely retrieves an int value from a map[string]any.
// Returns 0 if key doesn't exist or value is not numeric.
func GetInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetBool safely retrieves a bool value from a map[string]any.
// Returns false if key doesn't exist or value is not a bool.
func GetBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// MapOrEmptyBool returns the map if non-nil, otherwise returns an empty map.
func MapOrEmptyBool(m map[string]bool) map[string]bool {
	if m == nil {
		return make(map[string]bool)
	}
	return m
}

// MapOrEmptyString returns the map if non-nil, otherwise returns an empty map.
func MapOrEmptyString(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m
}
