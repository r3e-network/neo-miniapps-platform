package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeshaw/envdecode"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// ServerConfig controls the HTTP server.
type ServerConfig struct {
	Host string `json:"host" env:"SERVER_HOST"`
	Port int    `json:"port" env:"SERVER_PORT"`
}

// DatabaseConfig controls persistence.
type DatabaseConfig struct {
	Driver          string `json:"driver" env:"DATABASE_DRIVER"`
	DSN             string `json:"dsn" env:"DATABASE_DSN"`
	Host            string `json:"host" env:"DATABASE_HOST"`
	Port            int    `json:"port" env:"DATABASE_PORT"`
	User            string `json:"user" env:"DATABASE_USER"`
	Password        string `json:"password" env:"DATABASE_PASSWORD"`
	Name            string `json:"name" env:"DATABASE_NAME"`
	SSLMode         string `json:"sslmode" env:"DATABASE_SSLMODE"`
	MaxOpenConns    int    `json:"max_open_conns" env:"DATABASE_MAX_OPEN_CONNS"`
	MaxIdleConns    int    `json:"max_idle_conns" env:"DATABASE_MAX_IDLE_CONNS"`
	ConnMaxLifetime int    `json:"conn_max_lifetime" env:"DATABASE_CONN_MAX_LIFETIME"`
}

// LoggingConfig controls application logging.
type LoggingConfig struct {
	Level      string `json:"level" env:"LOG_LEVEL"`
	Format     string `json:"format" env:"LOG_FORMAT"`
	Output     string `json:"output" env:"LOG_OUTPUT"`
	FilePrefix string `json:"file_prefix" env:"LOG_FILE_PREFIX"`
}

// SecurityConfig controls encryption-specific parameters.
type SecurityConfig struct {
	SecretEncryptionKey string `json:"secret_encryption_key" env:"SECRET_ENCRYPTION_KEY"`
}

// AuthConfig controls HTTP API authentication.
type AuthConfig struct {
	Tokens    []string   `json:"tokens"`
	JWTSecret string     `json:"jwt_secret" env:"AUTH_JWT_SECRET"`
	Users     []UserSpec `json:"users"`
}

type UserSpec struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Config is the top-level configuration structure.
type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Logging  LoggingConfig  `json:"logging"`
	Runtime  RuntimeConfig  `json:"runtime"`
	Security SecurityConfig `json:"security"`
	Auth     AuthConfig     `json:"auth"`
}

// New returns a configuration populated with defaults.
func New() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Driver:          "postgres",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 300,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     "stdout",
			FilePrefix: "service-layer",
		},
		Runtime:  RuntimeConfig{},
		Security: SecurityConfig{},
		Auth:     AuthConfig{},
	}
}

// ConnectionString builds a PostgreSQL connection string using host parameters.
func (c DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// Load loads configuration from file (if present) and environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := New()

	if path := strings.TrimSpace(os.Getenv("CONFIG_FILE")); path != "" {
		if err := loadFromFile(path, cfg); err != nil {
			return nil, err
		}
	} else {
		_ = loadFromFile("configs/config.yaml", cfg)
	}

	if err := envdecode.Decode(cfg); err != nil {
		// envdecode returns an error when no tagged fields are present in the
		// environment; treat that case as "no overrides" so local runs work
		// without exporting vars.
		if !strings.Contains(err.Error(), "none of the target fields were set") {
			return nil, fmt.Errorf("decode env: %w", err)
		}
	}

	return cfg, nil
}

// LoadFile reads configuration from a YAML file.
func LoadFile(path string) (*Config, error) {
	cfg := New()
	if err := loadFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadFromFile(path string, cfg *Config) error {
	expanded, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return err
	}
	return nil
}

// LoadConfig is a helper used by tests to load JSON config snippets.
func LoadConfig(path string) (*Config, error) {
	cfg := New()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
