// Package config provides environment-aware configuration management
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Environment represents the deployment environment
type Environment string

const (
	Development Environment = "development"
	Testing     Environment = "testing"
	Production  Environment = "production"
)

// Config holds all application configuration
type Config struct {
	// Environment
	Env Environment

	// MarbleRun
	CoordinatorAddr   string
	MarbleType        string
	MarbleDNSNames    []string
	MarbleRunInsecure bool

	// Neo N3
	NeoRPCURL       string
	NeoNetworkMagic uint32

	// Supabase
	SupabaseURL        string
	SupabaseServiceKey string

	// Service Ports
	GatewayPort     int
	VRFPort         int
	NeoVaultPort    int
	NeoFeedsPort    int
	NeoFlowPort     int
	NeoAccountsPort int
	NeoComputePort  int
	SecretsPort     int
	OraclePort      int

	// Logging
	LogLevel  string
	LogFormat string

	// Security
	JWTExpiry         time.Duration
	RateLimitEnabled  bool
	RateLimitRequests int
	RateLimitWindow   time.Duration
	CORSOrigins       []string

	// Database
	DBMaxConnections int
	DBIdleTimeout    time.Duration

	// Features
	EnableProfiling      bool
	EnableDebugEndpoints bool
	TestMode             bool
	MetricsEnabled       bool
	MetricsPort          int
	TracingEnabled       bool
	TracingEndpoint      string
}

// Load loads configuration based on MARBLE_ENV environment variable
func Load() (*Config, error) {
	// Get environment from MARBLE_ENV
	envStr := os.Getenv("MARBLE_ENV")
	if envStr == "" {
		envStr = "development"
	}

	env := Environment(envStr)
	if !isValidEnvironment(env) {
		return nil, fmt.Errorf("invalid MARBLE_ENV: %s (must be development, testing, or production)", envStr)
	}

	// Load environment-specific .env file
	configFile := filepath.Join("config", fmt.Sprintf("%s.env", env))
	if err := godotenv.Load(configFile); err != nil {
		// Config file is optional; only warn on non-"file not found" errors
		// (e.g. parse errors) to avoid noisy logs during tests and CI runs.
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Printf("Warning: Could not load %s: %v\n", configFile, err)
		}
	}

	cfg := &Config{
		Env: env,
	}

	// Load all configuration values
	if err := cfg.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return cfg, nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() error {
	// MarbleRun
	c.CoordinatorAddr = getEnv("COORDINATOR_CLIENT_ADDR", "")
	if c.CoordinatorAddr == "" {
		c.CoordinatorAddr = getEnv("COORDINATOR_ADDR", "localhost:4433")
	}
	c.MarbleType = getEnv("MARBLE_TYPE", "gateway")
	c.MarbleDNSNames = strings.Split(getEnv("MARBLE_DNS_NAMES", "localhost"), ",")
	c.MarbleRunInsecure = getBoolEnv("MARBLERUN_INSECURE", true)

	// Neo N3
	c.NeoRPCURL = getEnv("NEO_RPC_URL", "https://testnet1.neo.coz.io:443")
	magic, err := strconv.ParseUint(getEnv("NEO_NETWORK_MAGIC", "894710606"), 10, 32)
	if err != nil {
		return fmt.Errorf("invalid NEO_NETWORK_MAGIC: %w", err)
	}
	c.NeoNetworkMagic = uint32(magic)

	// Supabase
	c.SupabaseURL = getEnv("SUPABASE_URL", "")
	c.SupabaseServiceKey = getEnv("SUPABASE_SERVICE_KEY", "")
	if c.SupabaseURL == "" || c.SupabaseServiceKey == "" {
		return fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY are required")
	}

	// Service Ports
	c.GatewayPort = getIntEnv("GATEWAY_PORT", 8080)
	c.VRFPort = getIntEnv("VRF_PORT", 8081)
	c.NeoVaultPort = getIntEnv("NEOVAULT_PORT", 8082)
	c.NeoFeedsPort = getIntEnv("NEOFEEDS_PORT", 8083)
	c.NeoFlowPort = getIntEnv("NEOFLOW_PORT", 8084)
	c.NeoAccountsPort = getIntEnv("NEOACCOUNTS_PORT", 8085)
	c.NeoComputePort = getIntEnv("NEOCOMPUTE_PORT", 8086)
	c.SecretsPort = getIntEnv("SECRETS_PORT", 8087)
	c.OraclePort = getIntEnv("ORACLE_PORT", 8088)

	// Logging
	c.LogLevel = getEnv("LOG_LEVEL", "info")
	c.LogFormat = getEnv("LOG_FORMAT", "json")

	// Security
	jwtExpiry := getEnv("JWT_EXPIRY", "15m")
	c.JWTExpiry, err = time.ParseDuration(jwtExpiry)
	if err != nil {
		return fmt.Errorf("invalid JWT_EXPIRY: %w", err)
	}
	c.RateLimitEnabled = getBoolEnv("RATE_LIMIT_ENABLED", true)
	c.RateLimitRequests = getIntEnv("RATE_LIMIT_REQUESTS", 100)
	rateLimitWindow := getEnv("RATE_LIMIT_WINDOW", "1m")
	c.RateLimitWindow, err = time.ParseDuration(rateLimitWindow)
	if err != nil {
		return fmt.Errorf("invalid RATE_LIMIT_WINDOW: %w", err)
	}
	c.CORSOrigins = strings.Split(getEnv("CORS_ALLOWED_ORIGINS", getEnv("CORS_ORIGINS", "*")), ",")

	// Database
	c.DBMaxConnections = getIntEnv("DB_MAX_CONNECTIONS", 20)
	dbIdleTimeout := getEnv("DB_IDLE_TIMEOUT", "5m")
	c.DBIdleTimeout, err = time.ParseDuration(dbIdleTimeout)
	if err != nil {
		return fmt.Errorf("invalid DB_IDLE_TIMEOUT: %w", err)
	}

	// Features
	c.EnableProfiling = getBoolEnv("ENABLE_PROFILING", false)
	c.EnableDebugEndpoints = getBoolEnv("ENABLE_DEBUG_ENDPOINTS", false)
	c.TestMode = getBoolEnv("TEST_MODE", false)
	c.MetricsEnabled = getBoolEnv("METRICS_ENABLED", c.Env == Production)
	c.MetricsPort = getIntEnv("METRICS_PORT", 9090)
	c.TracingEnabled = getBoolEnv("TRACING_ENABLED", c.Env == Production)
	c.TracingEndpoint = getEnv("TRACING_ENDPOINT", "")

	return nil
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Env == Development
}

// IsTesting returns true if running in testing environment
func (c *Config) IsTesting() bool {
	return c.Env == Testing
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Env == Production
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.IsProduction() {
		// Production-specific validations
		if c.MarbleRunInsecure {
			return fmt.Errorf("MARBLERUN_INSECURE must be false in production")
		}
		if c.EnableDebugEndpoints {
			return fmt.Errorf("ENABLE_DEBUG_ENDPOINTS must be false in production")
		}
		if c.TestMode {
			return fmt.Errorf("TEST_MODE must be false in production")
		}
		if !c.RateLimitEnabled {
			return fmt.Errorf("RATE_LIMIT_ENABLED must be true in production")
		}
	}

	// Port validations
	ports := []int{
		c.GatewayPort, c.VRFPort, c.NeoVaultPort, c.NeoFeedsPort,
		c.NeoFlowPort, c.NeoAccountsPort, c.NeoComputePort,
		c.SecretsPort, c.OraclePort,
	}
	for _, port := range ports {
		if port < 1024 || port > 65535 {
			return fmt.Errorf("invalid port number: %d (must be between 1024 and 65535)", port)
		}
	}

	return nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func isValidEnvironment(env Environment) bool {
	return env == Development || env == Testing || env == Production
}
