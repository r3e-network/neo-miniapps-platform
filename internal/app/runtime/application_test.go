package runtime

import (
	"os"
	"testing"

	"github.com/R3E-Network/service_layer/internal/config"
)

func TestDetermineListenAddrUsesDefaults(t *testing.T) {
	cfg := config.New()
	cfg.Server.Host = ""
	cfg.Server.Port = 0

	addr := determineListenAddr(cfg)
	if addr != "0.0.0.0:8080" {
		t.Fatalf("expected default addr, got %s", addr)
	}

	cfg.Server.Host = "127.0.0.1 "
	cfg.Server.Port = 9090
	if got := determineListenAddr(cfg); got != "127.0.0.1:9090" {
		t.Fatalf("unexpected addr %s", got)
	}
}

func TestSplitAndTrim(t *testing.T) {
	cases := map[string][]string{
		"":                 nil,
		"   ":              nil,
		"token1":           {"token1"},
		"token1, token2  ": {"token1", "token2"},
	}

	for input, expected := range cases {
		if got := splitAndTrim(input); len(got) != len(expected) {
			t.Fatalf("splitAndTrim(%q) length mismatch: %v vs %v", input, got, expected)
		}
	}
}

func TestLoadAPITokensReadsEnv(t *testing.T) {
	t.Setenv("API_TOKENS", "alpha, beta")
	t.Setenv("API_TOKEN", "gamma")
	cfg := config.New()
	tokens := loadAPITokens(cfg)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0] != "alpha" || tokens[2] != "gamma" {
		t.Fatalf("unexpected tokens %v", tokens)
	}
	os.Unsetenv("API_TOKENS")
	os.Unsetenv("API_TOKEN")
}
