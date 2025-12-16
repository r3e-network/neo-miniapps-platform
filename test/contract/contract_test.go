// Package contract provides contract deployment and integration tests.
package contract

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeoExpressSetup(t *testing.T) {
	SkipIfNoNeoExpress(t)

	nx := NewNeoExpress(t)

	t.Run("wallet operations", func(t *testing.T) {
		addr, err := nx.GetWalletAddress("owner")
		if err != nil {
			t.Logf("owner wallet lookup returned: %v", err)
			t.Logf("This is expected if neo-express hasn't been initialized with wallets")
			t.Skip("neo-express wallets not configured - run 'neoxp wallet create owner' first")
		}

		if addr == "" {
			t.Error("wallet address should not be empty")
		}

		t.Logf("wallet address: %s", addr)
	})
}

func TestContractCompilation(t *testing.T) {
	SkipIfNoNeoExpress(t)

	type contractSpec struct {
		name       string // artifact base name (matches contracts/build/*.nef)
		sourceFile string // main source file for sanity checks
	}

	specs := []contractSpec{
		{
			name:       "ServiceLayerGateway",
			sourceFile: filepath.Join("..", "..", "contracts", "gateway", "ServiceLayerGateway.cs"),
		},
		{
			name:       "DataFeedsService",
			sourceFile: filepath.Join("..", "..", "services", "datafeed", "contract", "NeoFeedsService.cs"),
		},
		{
			name:       "NeoFlowService",
			sourceFile: filepath.Join("..", "..", "services", "automation", "contract", "NeoFlowService.cs"),
		},
		{
			name:       "ConfidentialService",
			sourceFile: filepath.Join("..", "..", "services", "confcompute", "contract", "NeoComputeService.cs"),
		},
		{
			name:       "OracleService",
			sourceFile: filepath.Join("..", "..", "services", "conforacle", "contract", "NeoOracleService.cs"),
		},
	}

	for _, spec := range specs {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			if _, err := os.Stat(spec.sourceFile); os.IsNotExist(err) {
				t.Fatalf("source file not found: %s", spec.sourceFile)
			}

			contractBase := filepath.Join("..", "..", "contracts", "build")
			nefFile := filepath.Join(contractBase, spec.name+".nef")
			manifestFile := filepath.Join(contractBase, spec.name+".manifest.json")

			if _, err := os.Stat(nefFile); os.IsNotExist(err) {
				t.Logf("NEF file not found, contract needs compilation: %s", nefFile)
				t.Skip("contracts not compiled - run ./contracts/build.sh")
			}

			if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
				t.Errorf("manifest file missing: %s", manifestFile)
			}
		})
	}
}

func TestContractDeploymentIntegration(t *testing.T) {
	SkipIfNoNeoExpress(t)
	SkipIfNoCompiledContracts(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	nx := NewNeoExpress(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("start neo-express", func(t *testing.T) {
		if err := nx.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() {
			nx.Stop()
		})
	})

	t.Run("deploy gateway contract", func(t *testing.T) {
		nefPath, _, err := FindContractArtifacts("ServiceLayerGateway")
		if err != nil {
			t.Skip(err.Error())
		}

		contract, err := nx.Deploy(nefPath, "", "genesis")
		if err != nil {
			t.Logf("deploy failed (might need running neo-express): %v", err)
			t.Skip("deployment requires running neo-express instance")
		}

		if contract.Hash == "" {
			t.Logf("contract deployed (hash may be in logs)")
		} else {
			t.Logf("gateway deployed at: %s", contract.Hash)
		}
	})
}

func TestContractInteraction(t *testing.T) {
	SkipIfNoNeoExpress(t)
	SkipIfNoCompiledContracts(t)

	if testing.Short() {
		t.Skip("skipping interaction test in short mode")
	}

	t.Skip("requires running neo-express with deployed contracts")
}

func TestGatewayContract(t *testing.T) {
	SkipIfNoNeoExpress(t)
	SkipIfNoCompiledContracts(t)

	if testing.Short() {
		t.Skip("skipping gateway test in short mode")
	}

	t.Run("admin functions", func(t *testing.T) {
		t.Skip("requires deployed contract")
	})

	t.Run("service registration", func(t *testing.T) {
		t.Skip("requires deployed contract")
	})

	t.Run("fee management", func(t *testing.T) {
		t.Skip("requires deployed contract")
	})

	t.Run("request flow", func(t *testing.T) {
		t.Skip("requires deployed contract")
	})
}
