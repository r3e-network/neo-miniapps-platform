// Package runtime provides environment/runtime detection helpers shared across the service layer.
package runtime

import (
	"os"
	"strings"
)

// Environment represents the logical deployment environment.
//
// This is intentionally lightweight: it is derived from environment variables
// (primarily MARBLE_ENV) and is safe to use from low-level packages.
type Environment string

const (
	Development Environment = "development"
	Testing     Environment = "testing"
	Production  Environment = "production"
)

// Env returns the current environment derived from MARBLE_ENV (preferred) or
// ENVIRONMENT (legacy fallback). Unknown values default to Development.
func Env() Environment {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("MARBLE_ENV")))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	}

	switch Environment(raw) {
	case Development, Testing, Production:
		return Environment(raw)
	default:
		return Development
	}
}

func IsDevelopment() bool { return Env() == Development }
func IsTesting() bool     { return Env() == Testing }
func IsProduction() bool  { return Env() == Production }

func IsDevelopmentOrTesting() bool {
	env := Env()
	return env == Development || env == Testing
}
