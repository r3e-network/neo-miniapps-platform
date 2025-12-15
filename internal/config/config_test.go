package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid development config",
			envVars: map[string]string{
				"MARBLE_ENV":           "development",
				"SUPABASE_URL":         "https://test.supabase.co",
				"SUPABASE_SERVICE_KEY": "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing supabase url",
			envVars: map[string]string{
				"MARBLE_ENV":           "development",
				"SUPABASE_SERVICE_KEY": "test-key",
			},
			wantErr: true,
		},
		{
			name: "invalid environment",
			envVars: map[string]string{
				"MARBLE_ENV":           "invalid",
				"SUPABASE_URL":         "https://test.supabase.co",
				"SUPABASE_SERVICE_KEY": "test-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && cfg == nil {
				t.Error("Load() returned nil config without error")
			}
		})
	}
}

func TestConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name string
		env  Environment
		want bool
	}{
		{"development", Development, true},
		{"testing", Testing, false},
		{"production", Production, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Env: tt.env}
			if got := c.IsDevelopment(); got != tt.want {
				t.Errorf("IsDevelopment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_IsTesting(t *testing.T) {
	tests := []struct {
		name string
		env  Environment
		want bool
	}{
		{"development", Development, false},
		{"testing", Testing, true},
		{"production", Production, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Env: tt.env}
			if got := c.IsTesting(); got != tt.want {
				t.Errorf("IsTesting() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name string
		env  Environment
		want bool
	}{
		{"development", Development, false},
		{"testing", Testing, false},
		{"production", Production, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Env: tt.env}
			if got := c.IsProduction(); got != tt.want {
				t.Errorf("IsProduction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid development config",
			config: &Config{
				Env:               Development,
				MarbleRunInsecure: true,
				GatewayPort:       8080,
				VRFPort:           8081,
				NeoVaultPort:      8082,
				NeoFeedsPort:      8083,
				NeoFlowPort:       8084,
				NeoAccountsPort:   8085,
				NeoComputePort:    8086,
				SecretsPort:       8087,
				OraclePort:        8088,
			},
			wantErr: false,
		},
		{
			name: "production with insecure mode",
			config: &Config{
				Env:               Production,
				MarbleRunInsecure: true,
				GatewayPort:       8080,
			},
			wantErr: true,
		},
		{
			name: "production with debug endpoints",
			config: &Config{
				Env:                  Production,
				MarbleRunInsecure:    false,
				EnableDebugEndpoints: true,
				RateLimitEnabled:     true,
				GatewayPort:          8080,
			},
			wantErr: true,
		},
		{
			name: "invalid port number",
			config: &Config{
				Env:         Development,
				GatewayPort: 100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_KEY_MISSING",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIntEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		want         int
	}{
		{
			name:         "valid int",
			key:          "TEST_INT",
			defaultValue: 100,
			envValue:     "200",
			want:         200,
		},
		{
			name:         "invalid int",
			key:          "TEST_INT",
			defaultValue: 100,
			envValue:     "invalid",
			want:         100,
		},
		{
			name:         "not set",
			key:          "TEST_INT_MISSING",
			defaultValue: 100,
			envValue:     "",
			want:         100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			got := getIntEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getIntEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBoolEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue bool
		envValue     string
		want         bool
	}{
		{
			name:         "true value",
			key:          "TEST_BOOL",
			defaultValue: false,
			envValue:     "true",
			want:         true,
		},
		{
			name:         "false value",
			key:          "TEST_BOOL",
			defaultValue: true,
			envValue:     "false",
			want:         false,
		},
		{
			name:         "invalid value",
			key:          "TEST_BOOL",
			defaultValue: true,
			envValue:     "invalid",
			want:         true,
		},
		{
			name:         "not set",
			key:          "TEST_BOOL_MISSING",
			defaultValue: true,
			envValue:     "",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			got := getBoolEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getBoolEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  Environment
		want bool
	}{
		{"development", Development, true},
		{"testing", Testing, true},
		{"production", Production, true},
		{"invalid", Environment("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidEnvironment(tt.env); got != tt.want {
				t.Errorf("isValidEnvironment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_LoadFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("SUPABASE_URL", "https://test.supabase.co")
	os.Setenv("SUPABASE_SERVICE_KEY", "test-key")
	os.Setenv("JWT_EXPIRY", "15m")
	os.Setenv("RATE_LIMIT_WINDOW", "1m")
	os.Setenv("DB_IDLE_TIMEOUT", "5m")

	cfg := &Config{Env: Development}
	err := cfg.loadFromEnv()
	if err != nil {
		t.Fatalf("loadFromEnv() error = %v", err)
	}

	if cfg.SupabaseURL != "https://test.supabase.co" {
		t.Errorf("SupabaseURL = %v, want https://test.supabase.co", cfg.SupabaseURL)
	}

	if cfg.JWTExpiry != 15*time.Minute {
		t.Errorf("JWTExpiry = %v, want 15m", cfg.JWTExpiry)
	}
}
