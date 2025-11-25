package runtime

import (
	"testing"

	"github.com/R3E-Network/service_layer/internal/config"
)

func TestAppRuntimeConfigCopiesGasBankTuning(t *testing.T) {
	cfg := config.New()
	cfg.Runtime.GasBank.PollInterval = "45s"
	cfg.Runtime.GasBank.MaxAttempts = 7

	rc := AppRuntimeConfig(cfg)
	if rc.GasBankPollInterval != "45s" {
		t.Fatalf("expected poll interval to propagate, got %q", rc.GasBankPollInterval)
	}
	if rc.GasBankMaxAttempts != 7 {
		t.Fatalf("expected max attempts to propagate, got %d", rc.GasBankMaxAttempts)
	}
}
