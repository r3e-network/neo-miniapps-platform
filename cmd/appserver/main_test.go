package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/R3E-Network/service_layer/internal/config"
)

func TestResolveDSNPrecedence(t *testing.T) {
	cases := []struct {
		name string
		flag string
		env  string
		cfg  func() *config.Config
		want string
	}{
		{
			name: "flag wins",
			flag: "postgres://flag",
			env:  "postgres://env",
			cfg: func() *config.Config {
				cfg := config.New()
				cfg.Database.DSN = "postgres://cfg"
				return cfg
			},
			want: "postgres://flag",
		},
		{
			name: "env when flag missing",
			flag: "",
			env:  "postgres://env",
			cfg: func() *config.Config {
				cfg := config.New()
				cfg.Database.DSN = "postgres://cfg"
				return cfg
			},
			want: "postgres://env",
		},
		{
			name: "config dsn when flag/env empty",
			flag: "",
			env:  "",
			cfg: func() *config.Config {
				cfg := config.New()
				cfg.Database.DSN = "postgres://cfg"
				return cfg
			},
			want: "postgres://cfg",
		},
		{
			name: "legacy host fallback",
			flag: "",
			env:  "",
			cfg: func() *config.Config {
				cfg := config.New()
				cfg.Database.DSN = ""
				cfg.Database.Host = "localhost"
				cfg.Database.Port = 5432
				cfg.Database.User = "postgres"
				cfg.Database.Password = "postgres"
				cfg.Database.Name = "service_layer"
				cfg.Database.SSLMode = "disable"
				return cfg
			},
			want: "host=localhost port=5432 user=postgres password=postgres dbname=service_layer sslmode=disable",
		},
		{
			name: "empty when nothing provided",
			flag: "",
			env:  "",
			cfg: func() *config.Config {
				return config.New()
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// ensure base config doesn't leak between cases
			cfg := config.New()
			if tc.cfg != nil {
				cfg = tc.cfg()
			}

			if tc.env != "" {
				if err := os.Setenv("DATABASE_URL", tc.env); err != nil {
					t.Fatalf("setenv: %v", err)
				}
				t.Cleanup(func() { os.Unsetenv("DATABASE_URL") })
			} else {
				os.Unsetenv("DATABASE_URL")
			}

			got := resolveDSN(tc.flag, cfg)
			if got != tc.want {
				t.Fatalf("resolveDSN() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadConfigFileSupportsYAML(t *testing.T) {
	path := filepath.Join("testdata", "config-with-dsn.yaml")
	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatalf("loadConfigFile: %v", err)
	}
	if cfg.Database.DSN == "" {
		t.Fatalf("expected DSN populated from YAML config")
	}
}
