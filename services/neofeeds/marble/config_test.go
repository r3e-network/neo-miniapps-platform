// Package neofeeds provides configuration tests.
package neofeeds

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", cfg.Version)
	}

	if len(cfg.Sources) != 1 {
		t.Errorf("len(Sources) = %d, want 1", len(cfg.Sources))
	}

	if len(cfg.Feeds) != 20 {
		t.Errorf("len(Feeds) = %d, want 20", len(cfg.Feeds))
	}

	if cfg.UpdateInterval != 60*time.Second {
		t.Errorf("UpdateInterval = %v, want 60s", cfg.UpdateInterval)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     NeoFeedsConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: NeoFeedsConfig{
				Sources: []SourceConfig{
					{ID: "test", URL: "http://example.com", JSONPath: "price", Weight: 1},
				},
				Feeds: []FeedConfig{
					{ID: "TEST/USD", Sources: []string{"test"}, Enabled: true},
				},
			},
			wantErr: false,
		},
		{
			name: "missing sources",
			cfg: NeoFeedsConfig{
				Sources: []SourceConfig{},
			},
			wantErr: true,
		},
		{
			name: "source missing id",
			cfg: NeoFeedsConfig{
				Sources: []SourceConfig{
					{URL: "http://example.com", JSONPath: "price"},
				},
			},
			wantErr: true,
		},
		{
			name: "source missing url",
			cfg: NeoFeedsConfig{
				Sources: []SourceConfig{
					{ID: "test", JSONPath: "price"},
				},
			},
			wantErr: true,
		},
		{
			name: "source missing json_path",
			cfg: NeoFeedsConfig{
				Sources: []SourceConfig{
					{ID: "test", URL: "http://example.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "feed references unknown source",
			cfg: NeoFeedsConfig{
				Sources: []SourceConfig{
					{ID: "test", URL: "http://example.com", JSONPath: "price"},
				},
				Feeds: []FeedConfig{
					{ID: "TEST/USD", Sources: []string{"unknown"}, Enabled: true},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := NeoFeedsConfig{
		Sources: []SourceConfig{
			{ID: "test", URL: "http://example.com", JSONPath: "price"},
		},
		Feeds: []FeedConfig{
			{ID: "TEST/USD", Sources: []string{"test"}},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	// Check defaults were set
	if cfg.Sources[0].Weight != 1 {
		t.Errorf("Source weight default = %d, want 1", cfg.Sources[0].Weight)
	}

	if cfg.Sources[0].Timeout != 10*time.Second {
		t.Errorf("Source timeout default = %v, want 10s", cfg.Sources[0].Timeout)
	}

	if cfg.Feeds[0].DataType != DataTypePrice {
		t.Errorf("Feed data_type default = %s, want price", cfg.Feeds[0].DataType)
	}

	if cfg.Feeds[0].Decimals != 8 {
		t.Errorf("Feed decimals default = %d, want 8", cfg.Feeds[0].Decimals)
	}

	if cfg.UpdateInterval != 60*time.Second {
		t.Errorf("UpdateInterval default = %v, want 60s", cfg.UpdateInterval)
	}
}

func TestConfigGetSource(t *testing.T) {
	cfg := DefaultConfig()

	src := cfg.GetSource("binance")
	if src == nil {
		t.Fatal("GetSource(binance) returned nil")
	}
	if src.Name != "Binance" {
		t.Errorf("Source name = %s, want Binance", src.Name)
	}

	src = cfg.GetSource("nonexistent")
	if src != nil {
		t.Error("GetSource(nonexistent) should return nil")
	}
}

func TestConfigGetFeed(t *testing.T) {
	cfg := DefaultConfig()

	feed := cfg.GetFeed("BTC/USD")
	if feed == nil {
		t.Fatal("GetFeed(BTC/USD) returned nil")
	}
	if feed.Pair != "BTCUSDT" {
		t.Errorf("Feed pair = %s, want BTCUSDT", feed.Pair)
	}

	feed = cfg.GetFeed("nonexistent")
	if feed != nil {
		t.Error("GetFeed(nonexistent) should return nil")
	}
}

func TestConfigGetEnabledFeeds(t *testing.T) {
	cfg := &NeoFeedsConfig{
		Sources: []SourceConfig{
			{ID: "test", URL: "http://example.com", JSONPath: "price"},
		},
		Feeds: []FeedConfig{
			{ID: "ENABLED", Sources: []string{"test"}, Enabled: true},
			{ID: "DISABLED", Sources: []string{"test"}, Enabled: false},
		},
	}
	_ = cfg.Validate()

	enabled := cfg.GetEnabledFeeds()
	if len(enabled) != 1 {
		t.Errorf("len(GetEnabledFeeds()) = %d, want 1", len(enabled))
	}
	if enabled[0].ID != "ENABLED" {
		t.Errorf("Enabled feed ID = %s, want ENABLED", enabled[0].ID)
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	yamlContent := `
version: "1.0"
update_interval: 30s
sources:
  - id: test
    name: Test Source
    url: "http://example.com/price?symbol={pair}"
    json_path: price
    weight: 2
feeds:
  - id: TEST/USD
    data_type: price
    pair: TESTUSDT
    sources:
      - test
    enabled: true
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() error = %v", err)
	}

	if cfg.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", cfg.Version)
	}

	if cfg.UpdateInterval != 30*time.Second {
		t.Errorf("UpdateInterval = %v, want 30s", cfg.UpdateInterval)
	}

	if len(cfg.Sources) != 1 {
		t.Errorf("len(Sources) = %d, want 1", len(cfg.Sources))
	}

	if cfg.Sources[0].ID != "test" {
		t.Errorf("Source ID = %s, want test", cfg.Sources[0].ID)
	}

	if cfg.Sources[0].Weight != 2 {
		t.Errorf("Source weight = %d, want 2", cfg.Sources[0].Weight)
	}
}

func TestLoadConfigFromJSON(t *testing.T) {
	jsonContent := `{
  "version": "1.0",
  "update_interval": "45s",
  "sources": [
    {
      "id": "json_test",
      "name": "JSON Test",
      "url": "http://example.com/api",
      "json_path": "data.price",
      "weight": 3
    }
  ],
  "feeds": [
    {
      "id": "JSON/USD",
      "data_type": "price",
      "sources": ["json_test"],
      "enabled": true
    }
  ]
}`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() error = %v", err)
	}

	if cfg.Sources[0].ID != "json_test" {
		t.Errorf("Source ID = %s, want json_test", cfg.Sources[0].ID)
	}
}

func TestLoadConfigFromFileNotFound(t *testing.T) {
	_, err := LoadConfigFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("LoadConfigFromFile() expected error for nonexistent file")
	}
}

func TestConfigToJSON(t *testing.T) {
	cfg := DefaultConfig()
	data, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("ToJSON() returned empty data")
	}
}

func TestConfigToYAML(t *testing.T) {
	cfg := DefaultConfig()
	data, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("ToYAML() returned empty data")
	}
}

func TestDataTypes(t *testing.T) {
	if DataTypePrice != "price" {
		t.Errorf("DataTypePrice = %s, want price", DataTypePrice)
	}
	if DataTypeNumber != "number" {
		t.Errorf("DataTypeNumber = %s, want number", DataTypeNumber)
	}
	if DataTypeString != "string" {
		t.Errorf("DataTypeString = %s, want string", DataTypeString)
	}
}

func TestSourceConfigWithHeaders(t *testing.T) {
	cfg := NeoFeedsConfig{
		Sources: []SourceConfig{
			{
				ID:       "with_headers",
				URL:      "http://example.com",
				JSONPath: "price",
				Headers: map[string]string{
					"Authorization": "Bearer token",
					"X-Custom":      "value",
				},
			},
		},
		Feeds: []FeedConfig{
			{ID: "TEST", Sources: []string{"with_headers"}},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(cfg.Sources[0].Headers) != 2 {
		t.Errorf("len(Headers) = %d, want 2", len(cfg.Sources[0].Headers))
	}
}

func TestFeedDefaultSources(t *testing.T) {
	cfg := NeoFeedsConfig{
		Sources: []SourceConfig{
			{ID: "src1", URL: "http://example.com", JSONPath: "price"},
			{ID: "src2", URL: "http://example.com", JSONPath: "price"},
		},
		DefaultSources: []string{"src1", "src2"},
		Feeds: []FeedConfig{
			{ID: "TEST"}, // No sources specified
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(cfg.Feeds[0].Sources) != 2 {
		t.Errorf("Feed should inherit default sources, got %d", len(cfg.Feeds[0].Sources))
	}
}
