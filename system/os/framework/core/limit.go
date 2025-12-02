package service

import "strconv"

const (
	// DefaultListLimit is the standard default page size used across services.
	DefaultListLimit = 25
	// MaxListLimit is the standard maximum page size used across services.
	MaxListLimit = 500
)

// ClampLimit returns a sane list limit using the provided default and maximum.
// Non-positive values yield the default; values above max clamp to max.
func ClampLimit(limit, defaultLimit, max int) int {
	if defaultLimit <= 0 {
		defaultLimit = DefaultListLimit
	}
	if max <= 0 {
		max = defaultLimit
	}
	if limit <= 0 {
		return defaultLimit
	}
	if limit > max {
		return max
	}
	return limit
}

// ParseLimit parses a limit string and clamps it to the given bounds.
// Empty strings or parse errors return the default limit.
// This replaces all service-specific parseXxxLimit functions.
func ParseLimit(s string, defaultLimit, maxLimit int) int {
	if s == "" {
		return ClampLimit(0, defaultLimit, maxLimit)
	}
	limit, err := strconv.Atoi(s)
	if err != nil {
		return ClampLimit(0, defaultLimit, maxLimit)
	}
	return ClampLimit(limit, defaultLimit, maxLimit)
}

// ParseLimitFromQuery extracts and parses a limit from query parameters.
// Uses standard defaults if not specified.
func ParseLimitFromQuery(query map[string]string) int {
	return ParseLimit(query["limit"], DefaultListLimit, MaxListLimit)
}

// ParseLimitFromQueryWithBounds extracts and parses a limit with custom bounds.
func ParseLimitFromQueryWithBounds(query map[string]string, defaultLimit, maxLimit int) int {
	return ParseLimit(query["limit"], defaultLimit, maxLimit)
}
