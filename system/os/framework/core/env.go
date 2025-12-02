package service

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment helper functions for consistent env var handling.
// These consolidate duplicate implementations across cmd tools.

// EnvDefault returns the value of the environment variable named by key,
// or the default value if the variable is not set or empty.
func EnvDefault(key, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultValue
}

// EnvInt returns the integer value of the environment variable named by key,
// or the default value if the variable is not set, empty, or not a valid integer.
func EnvInt(key string, defaultValue int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

// EnvDuration returns the duration value of the environment variable named by key,
// or the default value if the variable is not set, empty, or not a valid duration.
func EnvDuration(key string, defaultValue time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}
