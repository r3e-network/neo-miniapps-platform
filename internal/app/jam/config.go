package jam

import "strings"

// Config governs whether the JAM HTTP API is mounted and which stores to use.
type Config struct {
	Enabled bool
	Store   string // "memory" (default) or "postgres"
	PGDSN   string
}

// Normalize fills defaults and trims whitespace.
func (c *Config) Normalize() {
	c.Store = strings.TrimSpace(strings.ToLower(c.Store))
	if c.Store == "" {
		c.Store = "memory"
	}
	c.PGDSN = strings.TrimSpace(c.PGDSN)
}
