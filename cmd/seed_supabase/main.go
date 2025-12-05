package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/R3E-Network/service_layer/platform/bootstrap"
)

func readFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(path))
}

func main() {
	var (
		storagePath   = flag.String("storage-path", "./sealed_store", "Path to sealed storage directory (must match server)")
		sealingKey    = flag.String("sealing-key", "./sealing.key", "Path to sealing key file (must match server)")
		enclaveID     = flag.String("enclave-id", "local-enclave", "Enclave ID")
		mode          = flag.String("mode", "simulation", "Enclave mode: simulation|hardware")
		envFile       = flag.String("env", "./infra/supabase/.env", "Path to Supabase .env with SERVICE_ROLE_KEY")
		renderedDir   = flag.String("rendered-dir", "./infra/supabase/rendered", "Directory containing rendered Supabase configs")
		oracleConfig  = flag.String("oracle-config", "oracle_supabase.json", "Rendered oracle config filename")
		datafeedConfig = flag.String("datafeeds-config", "datafeeds_supabase.json", "Rendered datafeeds config filename")
	)
	flag.Parse()

	ctx := context.Background()

	if err := godotenv.Load(*envFile); err != nil {
		log.Fatalf("load env (%s): %v", *envFile, err)
	}

	serviceRole := os.Getenv("SERVICE_ROLE_KEY")
	if serviceRole == "" {
		log.Fatalf("SERVICE_ROLE_KEY missing in %s", *envFile)
	}

	oraclePath := filepath.Join(*renderedDir, *oracleConfig)
	datafeedsPath := filepath.Join(*renderedDir, *datafeedConfig)

	oracleCfg, err := readFile(oraclePath)
	if err != nil {
		log.Fatalf("read oracle config: %v", err)
	}
	datafeedsCfg, err := readFile(datafeedsPath)
	if err != nil {
		log.Fatalf("read datafeeds config: %v", err)
	}

	fnd, err := bootstrap.NewFoundation(bootstrap.Config{
		EnclaveID:      *enclaveID,
		Mode:           *mode,
		SealingKeyPath: *sealingKey,
		StoragePath:    *storagePath,
	})
	if err != nil {
		log.Fatalf("bootstrap foundation: %v", err)
	}
	if err := fnd.Start(ctx); err != nil {
		log.Fatalf("start foundation: %v", err)
	}
	defer fnd.Stop(ctx)

	vault := fnd.TrustRoot().Vault()

	// Secrets (flat key to avoid path conflicts with config key)
	if err := vault.Store(ctx, "oracle", "supabase_api_key", []byte(serviceRole)); err != nil {
		log.Fatalf("store oracle supabase_api_key: %v", err)
	}
	if err := vault.Store(ctx, "datafeeds", "supabase_api_key", []byte(serviceRole)); err != nil {
		log.Fatalf("store datafeeds supabase_api_key: %v", err)
	}

	// Configs
	if err := vault.Store(ctx, "oracle", "supabase", oracleCfg); err != nil {
		log.Fatalf("store oracle/supabase config: %v", err)
	}
	if err := vault.Store(ctx, "datafeeds", "supabase", datafeedsCfg); err != nil {
		log.Fatalf("store datafeeds/supabase config: %v", err)
	}

	fmt.Printf("Seeded Supabase secrets/configs into sealed storage at %s (sealing key: %s)\n", *storagePath, *sealingKey)
}
