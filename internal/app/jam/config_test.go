package jam

import (
	"testing"
)

func TestConfig_Normalize(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   Config
	}{
		{
			name:   "empty config gets defaults",
			config: Config{},
			want: Config{
				Store:              "memory",
				RateLimitPerMinute: 60,
				MaxPreimageBytes:   10 * 1024 * 1024,
				MaxPendingPackages: 100,
				AccumulatorHash:    "blake3-256",
			},
		},
		{
			name: "store normalized to lowercase",
			config: Config{
				Store: "  POSTGRES  ",
			},
			want: Config{
				Store:              "postgres",
				RateLimitPerMinute: 60,
				MaxPreimageBytes:   10 * 1024 * 1024,
				MaxPendingPackages: 100,
				AccumulatorHash:    "blake3-256",
			},
		},
		{
			name: "PGDSN trimmed",
			config: Config{
				PGDSN: "  postgres://localhost/test  ",
			},
			want: Config{
				Store:              "memory",
				PGDSN:              "postgres://localhost/test",
				RateLimitPerMinute: 60,
				MaxPreimageBytes:   10 * 1024 * 1024,
				MaxPendingPackages: 100,
				AccumulatorHash:    "blake3-256",
			},
		},
		{
			name: "AllowedTokens trimmed and filtered",
			config: Config{
				AllowedTokens: []string{"  token1  ", "", "  ", "token2"},
			},
			want: Config{
				Store:              "memory",
				AllowedTokens:      []string{"token1", "token2"},
				RateLimitPerMinute: 60,
				MaxPreimageBytes:   10 * 1024 * 1024,
				MaxPendingPackages: 100,
				AccumulatorHash:    "blake3-256",
			},
		},
		{
			name: "custom values preserved",
			config: Config{
				Enabled:             true,
				Store:               "memory",
				RateLimitPerMinute:  120,
				MaxPreimageBytes:    5 * 1024 * 1024,
				MaxPendingPackages:  50,
				AccumulatorHash:     "sha256",
				AccumulatorsEnabled: true,
			},
			want: Config{
				Enabled:             true,
				Store:               "memory",
				RateLimitPerMinute:  120,
				MaxPreimageBytes:    5 * 1024 * 1024,
				MaxPendingPackages:  50,
				AccumulatorHash:     "sha256",
				AccumulatorsEnabled: true,
			},
		},
		{
			name: "AccumulatorHash normalized",
			config: Config{
				AccumulatorHash: "  BLAKE3-256  ",
			},
			want: Config{
				Store:              "memory",
				RateLimitPerMinute: 60,
				MaxPreimageBytes:   10 * 1024 * 1024,
				MaxPendingPackages: 100,
				AccumulatorHash:    "blake3-256",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			cfg.Normalize()

			if cfg.Store != tt.want.Store {
				t.Errorf("Store = %v, want %v", cfg.Store, tt.want.Store)
			}
			if cfg.RateLimitPerMinute != tt.want.RateLimitPerMinute {
				t.Errorf("RateLimitPerMinute = %v, want %v", cfg.RateLimitPerMinute, tt.want.RateLimitPerMinute)
			}
			if cfg.MaxPreimageBytes != tt.want.MaxPreimageBytes {
				t.Errorf("MaxPreimageBytes = %v, want %v", cfg.MaxPreimageBytes, tt.want.MaxPreimageBytes)
			}
			if cfg.MaxPendingPackages != tt.want.MaxPendingPackages {
				t.Errorf("MaxPendingPackages = %v, want %v", cfg.MaxPendingPackages, tt.want.MaxPendingPackages)
			}
			if cfg.AccumulatorHash != tt.want.AccumulatorHash {
				t.Errorf("AccumulatorHash = %v, want %v", cfg.AccumulatorHash, tt.want.AccumulatorHash)
			}
			if cfg.PGDSN != tt.want.PGDSN {
				t.Errorf("PGDSN = %v, want %v", cfg.PGDSN, tt.want.PGDSN)
			}
		})
	}
}

func TestTrimStrings(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "nil input",
			input:  nil,
			expect: nil,
		},
		{
			name:   "empty slice",
			input:  []string{},
			expect: nil,
		},
		{
			name:   "all empty strings",
			input:  []string{"", "  ", "\t"},
			expect: nil,
		},
		{
			name:   "mixed input",
			input:  []string{"  hello  ", "", "world", "  "},
			expect: []string{"hello", "world"},
		},
		{
			name:   "single value",
			input:  []string{"  value  "},
			expect: []string{"value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimStrings(tt.input)
			if len(result) != len(tt.expect) {
				t.Errorf("len(result) = %d, want %d", len(result), len(tt.expect))
				return
			}
			for i := range result {
				if result[i] != tt.expect[i] {
					t.Errorf("result[%d] = %q, want %q", i, result[i], tt.expect[i])
				}
			}
		})
	}
}
