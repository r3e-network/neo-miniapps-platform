package functions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/function"
)

func TestJavaScriptExamplesExecute(t *testing.T) {
	examples := []struct {
		name    string
		file    string
		params  map[string]any
		secrets map[string]string
		actions int
	}{
		{
			name:    "gasbank_topup",
			file:    "examples/functions/devpack/js/gasbank_topup.js",
			params:  map[string]any{"wallet": "NWALLET-1", "amount": 1.25},
			secrets: map[string]string{"defaultWallet": "NWALLET-1"},
			actions: 2,
		},
		{
			name:    "oracle_price_update",
			file:    "examples/functions/devpack/js/oracle_price_update.js",
			params:  map[string]any{"dataSourceId": "source-1", "symbol": "NEO"},
			actions: 1,
		},
		{
			name:    "automation_guardrail",
			file:    "examples/functions/devpack/js/automation_guardrail.js",
			params:  map[string]any{"schedule": "0 * * * *", "registerTrigger": true},
			secrets: map[string]string{"timezone": "UTC"},
			actions: 2,
		},
		{
			name:    "oracle_followup",
			file:    "examples/functions/devpack/js/oracle_followup.js",
			params:  map[string]any{"dataSourceId": "source-1", "schedule": "@hourly"},
			actions: 2,
		},
		{
			name:    "trigger_webhook_forward",
			file:    "examples/functions/devpack/js/trigger_webhook_forward.js",
			params:  map[string]any{"endpoint": "https://example.com/hook"},
			secrets: map[string]string{"webhookToken": "secret-token"},
			actions: 1,
		},
	}

	root := findModuleRoot(t)

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			sourcePath := filepath.Join(root, example.file)
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read example %s: %v", sourcePath, err)
			}

			exec := NewTEEExecutor(&teeSecretResolver{values: example.secrets})
			var secretNames []string
			for name := range example.secrets {
				secretNames = append(secretNames, name)
			}

			def := function.Definition{
				ID:        "fn-" + example.name,
				AccountID: "acct-1",
				Source:    string(source),
				Secrets:   secretNames,
			}

			result, err := exec.Execute(context.Background(), def, example.params)
			if err != nil {
				t.Fatalf("execute %s: %v", example.name, err)
			}
			if len(result.Actions) != example.actions {
				t.Fatalf("expected %d actions, got %d", example.actions, len(result.Actions))
			}
		})
	}
}

func findModuleRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatalf("could not locate module root from %s", wd)
		}
		wd = parent
	}
}
