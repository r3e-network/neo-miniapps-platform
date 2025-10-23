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
