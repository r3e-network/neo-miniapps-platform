// Package main provides the generic Marble entry point for all Neo services.
// The service type is determined by the MARBLE_TYPE environment variable.
// Each service is a separate Marble in MarbleRun, running in its own TEE enclave.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/config"
	"github.com/R3E-Network/service_layer/internal/database"
	sllogging "github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/marble"
	slmetrics "github.com/R3E-Network/service_layer/internal/metrics"
	slmiddleware "github.com/R3E-Network/service_layer/internal/middleware"
	"github.com/R3E-Network/service_layer/internal/runtime"

	// Neo service imports
	neoaccounts "github.com/R3E-Network/service_layer/services/neoaccounts/marble"
	neoaccountssupabase "github.com/R3E-Network/service_layer/services/neoaccounts/supabase"
	neocompute "github.com/R3E-Network/service_layer/services/neocompute/marble"
	neofeedschain "github.com/R3E-Network/service_layer/services/neofeeds/chain"
	neofeeds "github.com/R3E-Network/service_layer/services/neofeeds/marble"
	neoflow "github.com/R3E-Network/service_layer/services/neoflow/marble"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/neoflow/supabase"
	neooracle "github.com/R3E-Network/service_layer/services/neooracle/marble"
	neorand "github.com/R3E-Network/service_layer/services/neorand/marble"
	neorandsupabase "github.com/R3E-Network/service_layer/services/neorand/supabase"
	neostore "github.com/R3E-Network/service_layer/services/neostore/marble"
	neostoresupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"
	neovault "github.com/R3E-Network/service_layer/services/neovault/marble"
	neovaultsupabase "github.com/R3E-Network/service_layer/services/neovault/supabase"
)

// ServiceRunner interface for all Neo services
type ServiceRunner interface {
	Start(ctx context.Context) error
	Stop() error
	Router() *mux.Router
}

// Available Neo services
var availableServices = []string{
	"neoaccounts", "neocompute", "neofeeds", "neoflow",
	"neooracle", "neorand", "neostore", "neovault",
}

func normalizeServiceURLForMTLS(m *marble.Marble, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	useMTLS := m != nil && m.TLSConfig() != nil
	if !useMTLS {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
		return parsed.String()
	}
	return raw
}

func main() {
	ctx := context.Background()

	// Get service type from environment (injected by MarbleRun manifest)
	serviceType := os.Getenv("MARBLE_TYPE")
	if serviceType == "" {
		serviceType = os.Getenv("SERVICE_TYPE") // Fallback for local testing
	}
	if serviceType == "" {
		log.Fatalf("MARBLE_TYPE environment variable required. Available services: %v", availableServices)
	}

	log.Printf("Available services: %v", availableServices)
	log.Printf("Starting %s service...", serviceType)

	// Load services configuration
	servicesCfg := config.LoadServicesConfigOrDefault()

	// Check if service is enabled in config
	if !servicesCfg.IsEnabled(serviceType) {
		log.Printf("Service %s is disabled in configuration, exiting gracefully", serviceType)
		os.Exit(0) // Graceful exit for disabled services
	}

	// Initialize Marble
	m, err := marble.New(marble.Config{
		MarbleType: serviceType,
	})
	if err != nil {
		log.Fatalf("Failed to create marble: %v", err)
	}

	// Initialize Marble with Coordinator
	if initErr := m.Initialize(ctx); initErr != nil {
		log.Fatalf("Failed to initialize marble: %v", initErr)
	}

	// In production/SGX mode, require MarbleRun-injected mTLS credentials.
	// This ensures service-to-service identity headers can be trusted and prevents
	// accidentally deploying plaintext HTTP within the mesh.
	if (runtime.StrictIdentityMode() || m.IsEnclave()) && m.TLSConfig() == nil {
		log.Fatalf("CRITICAL: MarbleRun TLS credentials are required in production/SGX mode (missing MARBLE_CERT/MARBLE_KEY/MARBLE_ROOT_CA)")
	}

	// Initialize database
	supabaseURL := strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	if secret, ok := m.Secret("SUPABASE_URL"); ok && len(secret) > 0 {
		supabaseURL = strings.TrimSpace(string(secret))
	}
	supabaseServiceKey := strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_KEY"))
	if secret, ok := m.Secret("SUPABASE_SERVICE_KEY"); ok && len(secret) > 0 {
		supabaseServiceKey = strings.TrimSpace(string(secret))
	}

	dbClient, err := database.NewClient(database.Config{
		URL:        supabaseURL,
		ServiceKey: supabaseServiceKey,
	})
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}
	db := database.NewRepository(dbClient)

	// Initialize repositories
	neoaccountsRepo := neoaccountssupabase.NewRepository(db)
	neorandRepo := neorandsupabase.NewRepository(db)
	neovaultRepo := neovaultsupabase.NewRepository(db)
	neoflowRepo := neoflowsupabase.NewRepository(db)
	neostoreRepo := newNeoStoreRepositoryAdapter(db)

	// Chain configuration
	neoRPCURL := strings.TrimSpace(os.Getenv("NEO_RPC_URL"))
	var networkMagic uint32
	if magicStr := strings.TrimSpace(os.Getenv("NEO_NETWORK_MAGIC")); magicStr != "" {
		if magic, parseErr := strconv.ParseUint(magicStr, 10, 32); parseErr != nil {
			log.Printf("Warning: invalid NEO_NETWORK_MAGIC %q: %v", magicStr, parseErr)
		} else {
			networkMagic = uint32(magic)
		}
	}

	var chainClient *chain.Client
	if neoRPCURL == "" {
		log.Printf("Warning: NEO_RPC_URL not set; chain integration disabled")
	} else if client, clientErr := chain.NewClient(chain.Config{RPCURL: neoRPCURL, NetworkID: networkMagic, HTTPClient: m.ExternalHTTPClient()}); clientErr != nil {
		log.Printf("Warning: failed to initialize chain client: %v", clientErr)
	} else {
		chainClient = client
	}

	gatewayHash := trimHexPrefix(os.Getenv("CONTRACT_GATEWAY_HASH"))
	if chainClient != nil && gatewayHash == "" {
		log.Printf("Warning: CONTRACT_GATEWAY_HASH not set; TEE callbacks disabled")
	}
	neoFeedsHash := trimHexPrefix(os.Getenv("CONTRACT_NEOFEEDS_HASH"))
	neoflowHash := trimHexPrefix(os.Getenv("CONTRACT_NEOFLOW_HASH"))

	teePrivateKey := loadTEEPrivateKey(m)
	if chainClient != nil && teePrivateKey == "" {
		log.Printf("Warning: TEE_PRIVATE_KEY not configured; TEE fulfillments disabled")
	}

	var teeFulfiller *chain.TEEFulfiller
	if chainClient != nil && gatewayHash != "" && teePrivateKey != "" {
		if fulfiller, fulfillErr := chain.NewTEEFulfiller(chainClient, gatewayHash, teePrivateKey); fulfillErr != nil {
			log.Printf("Warning: failed to create TEE fulfiller: %v", fulfillErr)
		} else {
			teeFulfiller = fulfiller
		}
	}

	var gatewayContract *chain.GatewayContract
	if chainClient != nil && gatewayHash != "" {
		gatewayContract = chain.NewGatewayContract(chainClient, gatewayHash, nil)
	}

	var neoFeedsContract *neofeedschain.NeoFeedsContract
	if chainClient != nil && neoFeedsHash != "" {
		neoFeedsContract = neofeedschain.NewNeoFeedsContract(chainClient, neoFeedsHash, nil)
	}

	enableChainPush := chainClient != nil && teeFulfiller != nil && neoFeedsHash != ""
	enableChainExec := chainClient != nil && teeFulfiller != nil && neoflowHash != ""
	arbitrumRPC := strings.TrimSpace(os.Getenv("ARBITRUM_RPC"))
	secretsBaseURL := normalizeServiceURLForMTLS(m, strings.TrimSpace(os.Getenv("SECRETS_BASE_URL")))

	// Create service based on type
	var svc ServiceRunner
	switch serviceType {
	case "neoaccounts":
		svc, err = neoaccounts.New(neoaccounts.Config{
			Marble:          m,
			DB:              db,
			NeoAccountsRepo: neoaccountsRepo,
			ChainClient:     chainClient,
		})
	case "neocompute":
		if secretsBaseURL == "" {
			log.Printf("Warning: SECRETS_BASE_URL not set; NeoCompute cannot inject NeoStore secrets")
		}
		svc, err = neocompute.New(neocompute.Config{
			Marble:            m,
			DB:                db,
			SecretsBaseURL:    secretsBaseURL,
			SecretsHTTPClient: m.HTTPClient(),
		})
	case "neofeeds":
		svc, err = neofeeds.New(&neofeeds.Config{
			Marble:          m,
			DB:              db,
			ArbitrumRPC:     arbitrumRPC,
			ChainClient:     chainClient,
			TEEFulfiller:    teeFulfiller,
			NeoFeedsHash:    neoFeedsHash,
			EnableChainPush: enableChainPush,
		})
	case "neoflow":
		svc, err = neoflow.New(neoflow.Config{
			Marble:           m,
			DB:               db,
			NeoFlowRepo:      neoflowRepo,
			ChainClient:      chainClient,
			TEEFulfiller:     teeFulfiller,
			NeoFlowHash:      neoflowHash,
			NeoFeedsContract: neoFeedsContract,
			EnableChainExec:  enableChainExec,
		})
	case "neooracle":
		if secretsBaseURL == "" {
			log.Printf("Warning: SECRETS_BASE_URL not set; NeoOracle cannot reach NeoStore API")
		}

		oracleAllowlistRaw := strings.TrimSpace(os.Getenv("ORACLE_HTTP_ALLOWLIST"))
		oracleAllowlist := neooracle.URLAllowlist{Prefixes: splitAndTrimCSV(oracleAllowlistRaw)}
		if len(oracleAllowlist.Prefixes) == 0 {
			if runtime.StrictIdentityMode() || m.IsEnclave() {
				log.Fatalf("CRITICAL: ORACLE_HTTP_ALLOWLIST is required for NeoOracle in strict identity/SGX mode")
			}
			log.Printf("Warning: ORACLE_HTTP_ALLOWLIST not set; allowing all outbound URLs (development/testing only)")
		}

		svc, err = neooracle.New(neooracle.Config{
			Marble:            m,
			SecretsBaseURL:    secretsBaseURL,
			SecretsHTTPClient: m.HTTPClient(),
			URLAllowlist:      oracleAllowlist,
		})
	case "neorand":
		svc, err = neorand.New(neorand.Config{
			Marble:        m,
			DB:            db,
			NeoRandRepo:   neorandRepo,
			ChainClient:   chainClient,
			TEEFulfiller:  teeFulfiller,
			EventListener: nil,
		})
	case "neostore":
		svc, err = neostore.New(neostore.Config{
			Marble: m,
			DB:     neostoreRepo,
		})
	case "neovault":
		accountPoolURL := strings.TrimSpace(os.Getenv("ACCOUNTPOOL_URL"))
		if accountPoolURL == "" {
			log.Printf("Warning: ACCOUNTPOOL_URL not set; NeoVault pool integration disabled")
		}
		accountPoolURL = normalizeServiceURLForMTLS(m, accountPoolURL)

		svc, err = neovault.New(&neovault.Config{
			Marble:         m,
			DB:             db,
			NeoVaultRepo:   neovaultRepo,
			ChainClient:    chainClient,
			TEEFulfiller:   teeFulfiller,
			Gateway:        gatewayContract,
			NeoAccountsURL: accountPoolURL,
		})
	default:
		log.Fatalf("Unknown service: %s. Available: %v", serviceType, availableServices)
	}
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Standard middleware applied to all services.
	// - Logging: ensures X-Trace-ID is present and logs structured request entries.
	// - Recovery: prevents panics from crashing the process.
	logger := sllogging.NewFromEnv(serviceType)
	svc.Router().Use(slmiddleware.LoggingMiddleware(logger))
	svc.Router().Use(slmiddleware.NewRecoveryMiddleware(logger).Handler)
	if slmetrics.Enabled() {
		metricsCollector := slmetrics.Init(serviceType)
		svc.Router().Use(slmiddleware.MetricsMiddleware(serviceType, metricsCollector))
		svc.Router().Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	}
	// Cap request bodies to reduce memory/CPU DoS risk. Services are typically
	// accessed via the gateway, but this also protects internal mesh calls.
	svc.Router().Use(slmiddleware.NewBodyLimitMiddleware(0).Handler)

	// Start service
	if err := svc.Start(ctx); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}

	// Get port from config or environment
	port := os.Getenv("PORT")
	if port == "" {
		if settings := servicesCfg.GetSettings(serviceType); settings != nil && settings.Port > 0 {
			port = fmt.Sprintf("%d", settings.Port)
		} else {
			port = "8080"
		}
	}

	// Create HTTP server
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           svc.Router(),
		TLSConfig:         m.TLSConfig(),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Start server
	go func() {
		log.Printf("%s service listening on port %s", serviceType, port)
		var err error
		if m.TLSConfig() != nil {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	if err := svc.Stop(); err != nil {
		log.Printf("Service stop error: %v", err)
	}

	log.Println("Service stopped")
}

func splitAndTrimCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func trimHexPrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		prefix := strings.ToLower(value[:2])
		if prefix == "0x" {
			return value[2:]
		}
	}
	return value
}

func loadTEEPrivateKey(m *marble.Marble) string {
	if key := strings.TrimSpace(os.Getenv("TEE_PRIVATE_KEY")); key != "" {
		return trimHexPrefix(key)
	}
	if key := strings.TrimSpace(os.Getenv("TEE_WALLET_PRIVATE_KEY")); key != "" {
		return trimHexPrefix(key)
	}
	if secret, ok := m.Secret("TEE_PRIVATE_KEY"); ok && len(secret) > 0 {
		return hex.EncodeToString(secret)
	}
	if secret, ok := m.Secret("TEE_WALLET_PRIVATE_KEY"); ok && len(secret) > 0 {
		return hex.EncodeToString(secret)
	}
	return ""
}

type neoStoreRepositoryAdapter struct {
	repo neostoresupabase.RepositoryInterface
}

func newNeoStoreRepositoryAdapter(base *database.Repository) *neoStoreRepositoryAdapter {
	return &neoStoreRepositoryAdapter{repo: neostoresupabase.NewRepository(base)}
}

func (a *neoStoreRepositoryAdapter) GetSecrets(ctx context.Context, userID string) ([]neostoresupabase.Secret, error) {
	return a.repo.GetSecrets(ctx, userID)
}

func (a *neoStoreRepositoryAdapter) GetSecretByName(ctx context.Context, userID, name string) (*neostoresupabase.Secret, error) {
	return a.repo.GetSecretByName(ctx, userID, name)
}

func (a *neoStoreRepositoryAdapter) CreateSecret(ctx context.Context, secret *neostoresupabase.Secret) error {
	return a.repo.CreateSecret(ctx, secret)
}

func (a *neoStoreRepositoryAdapter) UpdateSecret(ctx context.Context, secret *neostoresupabase.Secret) error {
	return a.repo.UpdateSecret(ctx, secret)
}

func (a *neoStoreRepositoryAdapter) DeleteSecret(ctx context.Context, userID, name string) error {
	return a.repo.DeleteSecret(ctx, userID, name)
}

func (a *neoStoreRepositoryAdapter) GetAllowedServices(ctx context.Context, userID, secretName string) ([]string, error) {
	return a.repo.GetAllowedServices(ctx, userID, secretName)
}

func (a *neoStoreRepositoryAdapter) SetAllowedServices(ctx context.Context, userID, secretName string, services []string) error {
	return a.repo.SetAllowedServices(ctx, userID, secretName, services)
}

func (a *neoStoreRepositoryAdapter) CreateAuditLog(ctx context.Context, logEntry *neostoresupabase.AuditLog) error {
	return a.repo.CreateAuditLog(ctx, logEntry)
}

func (a *neoStoreRepositoryAdapter) GetAuditLogs(ctx context.Context, userID string, limit int) ([]neostoresupabase.AuditLog, error) {
	return a.repo.GetAuditLogs(ctx, userID, limit)
}

func (a *neoStoreRepositoryAdapter) GetAuditLogsForSecret(ctx context.Context, userID, secretName string, limit int) ([]neostoresupabase.AuditLog, error) {
	return a.repo.GetAuditLogsForSecret(ctx, userID, secretName, limit)
}
