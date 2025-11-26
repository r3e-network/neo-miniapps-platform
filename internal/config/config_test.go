package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionString(t *testing.T) {
	cfg := DatabaseConfig{Host: "localhost", Port: 5432, User: "user", Password: "pass", Name: "db", SSLMode: "disable"}
	want := "host=localhost port=5432 user=user password=pass dbname=db sslmode=disable"
	if got := cfg.ConnectionString(); got != want {
		t.Fatalf("connection string mismatch: %s", got)
	}
}

func TestConnectionString_EmptyFields(t *testing.T) {
	cfg := DatabaseConfig{}
	want := "host= port=0 user= password= dbname= sslmode="
	if got := cfg.ConnectionString(); got != want {
		t.Fatalf("connection string mismatch: %s", got)
	}
}

func TestLoadFromFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"host":"127.0.0.1"}}`), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("expected server host override, got %s", cfg.Server.Host)
	}
}

func TestLoadHandlesMissingFile(t *testing.T) {
	t.Setenv("CONFIG_FILE", "non-existent.yaml")
	t.Setenv("SERVER_PORT", "8080")
	if _, err := Load(); err != nil {
		t.Fatalf("load should ignore missing file: %v", err)
	}
}

func TestNew(t *testing.T) {
	cfg := New()
	if cfg == nil {
		t.Fatal("New() should return non-nil config")
	}

	// Check defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("expected default driver postgres, got %s", cfg.Database.Driver)
	}
	if cfg.Database.MaxOpenConns != 10 {
		t.Errorf("expected default MaxOpenConns 10, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 5 {
		t.Errorf("expected default MaxIdleConns 5, got %d", cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime != 300 {
		t.Errorf("expected default ConnMaxLifetime 300, got %d", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected default log format text, got %s", cfg.Logging.Format)
	}
	if cfg.Logging.Output != "stdout" {
		t.Errorf("expected default log output stdout, got %s", cfg.Logging.Output)
	}
	if cfg.Logging.FilePrefix != "service-layer" {
		t.Errorf("expected default file prefix service-layer, got %s", cfg.Logging.FilePrefix)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(path, []byte(`{invalid json}`), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFile_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yamlContent := `
server:
  host: "192.168.1.1"
  port: 9000
database:
  host: "db.example.com"
  port: 5432
  user: "admin"
  password: "secret"
  name: "testdb"
  sslmode: "require"
logging:
  level: "debug"
  format: "json"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}

	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("expected host 192.168.1.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("expected database host db.example.com, got %s", cfg.Database.Host)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected sslmode require, got %s", cfg.Database.SSLMode)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected log format json, got %s", cfg.Logging.Format)
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(path, []byte(`{not: valid: yaml:`), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadFile_MissingFile(t *testing.T) {
	// LoadFile should return nil error for missing files (via loadFromFile behavior)
	cfg, err := LoadFile("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("LoadFile should not error on missing file: %v", err)
	}
	// Should return defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port, got %d", cfg.Server.Port)
	}
}

func TestLoad_WithEnvOverride(t *testing.T) {
	// Clear any existing config file env var
	t.Setenv("CONFIG_FILE", "")
	t.Setenv("SERVER_HOST", "test.local")
	t.Setenv("SERVER_PORT", "3000")
	t.Setenv("DATABASE_HOST", "db.test.local")
	t.Setenv("LOG_LEVEL", "warn")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Server.Host != "test.local" {
		t.Errorf("expected SERVER_HOST override test.local, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected SERVER_PORT override 3000, got %d", cfg.Server.Port)
	}
	if cfg.Database.Host != "db.test.local" {
		t.Errorf("expected DATABASE_HOST override db.test.local, got %s", cfg.Database.Host)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("expected LOG_LEVEL override warn, got %s", cfg.Logging.Level)
	}
}

func TestLoad_WithConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_config.yaml")
	yamlContent := `
server:
  host: "config-file-host"
  port: 4000
`
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	t.Setenv("CONFIG_FILE", path)
	t.Setenv("SERVER_HOST", "") // Clear any previous override

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Server.Host != "config-file-host" {
		t.Errorf("expected host from config file, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 4000 {
		t.Errorf("expected port from config file, got %d", cfg.Server.Port)
	}
}

func TestLoadConfig_AllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "full_config.json")
	jsonContent := `{
		"server": {"host": "test", "port": 5000},
		"database": {
			"driver": "mysql",
			"dsn": "mysql://localhost/test",
			"host": "db.local",
			"port": 3306,
			"user": "testuser",
			"password": "testpass",
			"name": "testdb",
			"sslmode": "disable",
			"max_open_conns": 20,
			"max_idle_conns": 10,
			"conn_max_lifetime": 600
		},
		"logging": {
			"level": "error",
			"format": "json",
			"output": "file",
			"file_prefix": "test-app"
		},
		"security": {
			"secret_encryption_key": "test-key-123"
		},
		"auth": {
			"tokens": ["token1", "token2"],
			"jwt_secret": "jwt-secret-key",
			"users": [
				{"username": "admin", "password": "admin123", "role": "admin"},
				{"username": "user", "password": "user123", "role": "user"}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	// Server
	if cfg.Server.Host != "test" {
		t.Errorf("server host mismatch")
	}
	if cfg.Server.Port != 5000 {
		t.Errorf("server port mismatch")
	}

	// Database
	if cfg.Database.Driver != "mysql" {
		t.Errorf("database driver mismatch")
	}
	if cfg.Database.DSN != "mysql://localhost/test" {
		t.Errorf("database dsn mismatch")
	}
	if cfg.Database.MaxOpenConns != 20 {
		t.Errorf("database max_open_conns mismatch")
	}

	// Logging
	if cfg.Logging.Level != "error" {
		t.Errorf("logging level mismatch")
	}
	if cfg.Logging.FilePrefix != "test-app" {
		t.Errorf("logging file_prefix mismatch")
	}

	// Security
	if cfg.Security.SecretEncryptionKey != "test-key-123" {
		t.Errorf("security secret_encryption_key mismatch")
	}

	// Auth
	if len(cfg.Auth.Tokens) != 2 {
		t.Errorf("expected 2 auth tokens, got %d", len(cfg.Auth.Tokens))
	}
	if cfg.Auth.JWTSecret != "jwt-secret-key" {
		t.Errorf("auth jwt_secret mismatch")
	}
	if len(cfg.Auth.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(cfg.Auth.Users))
	}
	if cfg.Auth.Users[0].Username != "admin" || cfg.Auth.Users[0].Role != "admin" {
		t.Errorf("first user mismatch")
	}
}
