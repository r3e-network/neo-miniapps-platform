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
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
	"github.com/R3E-Network/service_layer/infrastructure/config"
	"github.com/R3E-Network/service_layer/infrastructure/database"
	sllogging "github.com/R3E-Network/service_layer/infrastructure/logging"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	slmetrics "github.com/R3E-Network/service_layer/infrastructure/metrics"
	slmiddleware "github.com/R3E-Network/service_layer/infrastructure/middleware"
	"github.com/R3E-Network/service_layer/infrastructure/runtime"
	"github.com/R3E-Network/service_layer/infrastructure/secrets"
	secretssupabase "github.com/R3E-Network/service_layer/infrastructure/secrets/supabase"
	txproxyclient "github.com/R3E-Network/service_layer/infrastructure/txproxy/client"
	txproxytypes "github.com/R3E-Network/service_layer/infrastructure/txproxy/types"

	// Neo service imports
	neoaccounts "github.com/R3E-Network/service_layer/infrastructure/accountpool/marble"
	neoaccountssupabase "github.com/R3E-Network/service_layer/infrastructure/accountpool/supabase"
	gsclient "github.com/R3E-Network/service_layer/infrastructure/globalsigner/client"
	globalsigner "github.com/R3E-Network/service_layer/infrastructure/globalsigner/marble"
	globalsignersupabase "github.com/R3E-Network/service_layer/infrastructure/globalsigner/supabase"
	neoflow "github.com/R3E-Network/service_layer/services/automation/marble"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"
	neocompute "github.com/R3E-Network/service_layer/services/confcompute/marble"
	neooracle "github.com/R3E-Network/service_layer/services/conforacle/marble"
	neofeeds "github.com/R3E-Network/service_layer/services/datafeed/marble"
	txproxy "github.com/R3E-Network/service_layer/services/txproxy/marble"
)

// ServiceRunner interface for all Neo services
type ServiceRunner interface {
	Start(ctx context.Context) error
	Stop() error
	Router() *mux.Router
}

// Available Neo services
var availableServices = []string{
	"globalsigner",
	"neoaccounts",
	"neocompute",
	"neofeeds",
	"neoflow",
	"neooracle",
	"txproxy",
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
	globalSignerRepo := globalsignersupabase.NewRepository(db)
	neoaccountsRepo := neoaccountssupabase.NewRepository(db)
	neoflowRepo := neoflowsupabase.NewRepository(db)

	// Chain configuration
	neoRPCURLs := chain.ParseEndpoints(strings.TrimSpace(os.Getenv("NEO_RPC_URLS")))
	if len(neoRPCURLs) == 0 {
		if secret, ok := m.Secret("NEO_RPC_URLS"); ok && len(secret) > 0 {
			neoRPCURLs = chain.ParseEndpoints(strings.TrimSpace(string(secret)))
		}
	}

	neoRPCURL := strings.TrimSpace(os.Getenv("NEO_RPC_URL"))
	if neoRPCURL == "" {
		if secret, ok := m.Secret("NEO_RPC_URL"); ok && len(secret) > 0 {
			neoRPCURL = strings.TrimSpace(string(secret))
		}
	}
	if neoRPCURL == "" && len(neoRPCURLs) > 0 {
		neoRPCURL = neoRPCURLs[0]
	}
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

	contracts := chain.ContractAddressesFromEnv()

	priceFeedHash := trimHexPrefix(contracts.PriceFeed)
	if priceFeedHash == "" {
		if secret, ok := m.Secret("CONTRACT_PRICEFEED_HASH"); ok && len(secret) > 0 {
			priceFeedHash = trimHexPrefix(string(secret))
		} else if secret, ok := m.Secret("CONTRACT_PRICE_FEED_HASH"); ok && len(secret) > 0 {
			priceFeedHash = trimHexPrefix(string(secret))
		}
	}

	automationAnchorHash := trimHexPrefix(contracts.AutomationAnchor)
	if automationAnchorHash == "" {
		if secret, ok := m.Secret("CONTRACT_AUTOMATIONANCHOR_HASH"); ok && len(secret) > 0 {
			automationAnchorHash = trimHexPrefix(string(secret))
		} else if secret, ok := m.Secret("CONTRACT_AUTOMATION_ANCHOR_HASH"); ok && len(secret) > 0 {
			automationAnchorHash = trimHexPrefix(string(secret))
		}
	}

	var teeSigner chain.TEESigner

	// Prefer GlobalSigner for TEE signing to avoid distributing long-lived private keys
	// to every enclave service. This keeps the active TEE signing key in one place.
	globalSignerURL := strings.TrimSpace(os.Getenv("GLOBALSIGNER_SERVICE_URL"))
	if globalSignerURL != "" && serviceType != "globalsigner" {
		client, err := gsclient.New(gsclient.Config{
			BaseURL:    globalSignerURL,
			ServiceID:  serviceType,
			HTTPClient: m.HTTPClient(),
			Timeout:    15 * time.Second,
		})
		if err != nil {
			log.Printf("Warning: failed to create GlobalSigner client: %v", err)
		} else if signer, err := chain.NewGlobalSignerSigner(ctx, client); err != nil {
			log.Printf("Warning: failed to initialize GlobalSigner signer: %v", err)
		} else {
			teeSigner = signer
			log.Printf("Using GlobalSigner for TEE signing (%s)", globalSignerURL)
		}
	}

	// Fallback to a locally injected private key (development/testing or transitional).
	if teeSigner == nil {
		teePrivateKey := loadTEEPrivateKey(m)
		if chainClient != nil && teePrivateKey == "" {
			log.Printf("Warning: TEE signer not configured (missing GLOBALSIGNER_SERVICE_URL and TEE_PRIVATE_KEY); chain fulfillments disabled")
		}
		if teePrivateKey != "" {
			if signer, err := chain.NewLocalTEESignerFromPrivateKeyHex(teePrivateKey); err != nil {
				log.Printf("Warning: failed to create local TEE signer: %v", err)
			} else {
				teeSigner = signer
			}
		}
	}

	var eventListener *chain.EventListener
	if chainClient != nil {
		startBlock := uint64(0)
		if raw := strings.TrimSpace(os.Getenv("NEO_EVENT_START_BLOCK")); raw != "" {
			if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
				startBlock = parsed
			} else {
				log.Printf("Warning: invalid NEO_EVENT_START_BLOCK %q: %v", raw, err)
			}
		} else if height, err := chainClient.GetBlockCount(ctx); err == nil && height > 0 {
			startBlock = height - 1
		}

		eventListener = chain.NewEventListener(&chain.ListenerConfig{
			Client: chainClient,
			Contracts: chain.ContractAddresses{
				PriceFeed:        priceFeedHash,
				AutomationAnchor: automationAnchorHash,
			},
			StartBlock:   startBlock,
			PollInterval: 5 * time.Second,
		})
	}

	arbitrumRPC := strings.TrimSpace(os.Getenv("ARBITRUM_RPC"))

	// TxProxy is the centralized "sign + broadcast" gatekeeper. NeoFeeds/NeoFlow
	// delegate all on-chain writes to it (single allowlist + audit surface).
	txproxyURL := strings.TrimSpace(os.Getenv("TXPROXY_URL"))
	if txproxyURL == "" {
		if secret, ok := m.Secret("TXPROXY_URL"); ok && len(secret) > 0 {
			txproxyURL = strings.TrimSpace(string(secret))
		}
	}

	var txProxyInvoker txproxytypes.Invoker
	if txproxyURL != "" && serviceType != "txproxy" {
		client, err := txproxyclient.New(txproxyclient.Config{
			BaseURL:    txproxyURL,
			ServiceID:  serviceType,
			HTTPClient: m.HTTPClient(),
			Timeout:    15 * time.Second,
		})
		if err != nil {
			log.Printf("Warning: failed to create TxProxy client: %v", err)
		} else {
			txProxyInvoker = client
			log.Printf("Using TxProxy for chain writes (%s)", txproxyURL)
		}
	}

	enablePriceFeedPush := chainClient != nil && priceFeedHash != "" && txProxyInvoker != nil
	enableChainPush := enablePriceFeedPush
	enableChainExec := chainClient != nil && automationAnchorHash != "" && txProxyInvoker != nil

	var svc ServiceRunner
	switch serviceType {
	case "globalsigner":
		svc, err = globalsigner.New(globalsigner.Config{
			Marble:     m,
			DB:         db,
			Repository: globalSignerRepo,
		})
	case "neoaccounts":
		var accountsSvc *neoaccounts.Service
		accountsSvc, err = neoaccounts.New(neoaccounts.Config{
			Marble:          m,
			DB:              db,
			NeoAccountsRepo: neoaccountsRepo,
			ChainClient:     chainClient,
		})
		svc = accountsSvc
	case "neocompute":
		svc, err = neocompute.New(neocompute.Config{
			Marble:         m,
			DB:             db,
			SecretProvider: newServiceSecretsProvider(m, db, neocompute.ServiceID),
		})
	case "neofeeds":
		var feedsSvc *neofeeds.Service
		feedsSvc, err = neofeeds.New(&neofeeds.Config{
			Marble:          m,
			DB:              db,
			ArbitrumRPC:     arbitrumRPC,
			ChainClient:     chainClient,
			PriceFeedHash:   priceFeedHash,
			TxProxy:         txProxyInvoker,
			EnableChainPush: enableChainPush,
		})
		svc = feedsSvc
	case "neoflow":
		var flowSvc *neoflow.Service
		flowSvc, err = neoflow.New(neoflow.Config{
			Marble:               m,
			DB:                   db,
			NeoFlowRepo:          neoflowRepo,
			ChainClient:          chainClient,
			PriceFeedHash:        priceFeedHash,
			AutomationAnchorHash: automationAnchorHash,
			TxProxy:              txProxyInvoker,
			EventListener:        eventListener,
			EnableChainExec:      enableChainExec,
		})
		svc = flowSvc
	case "neooracle":
		oracleAllowlistRaw := strings.TrimSpace(os.Getenv("ORACLE_HTTP_ALLOWLIST"))
		oracleAllowlist := neooracle.URLAllowlist{Prefixes: splitAndTrimCSV(oracleAllowlistRaw)}
		if len(oracleAllowlist.Prefixes) == 0 {
			if runtime.StrictIdentityMode() || m.IsEnclave() {
				log.Fatalf("CRITICAL: ORACLE_HTTP_ALLOWLIST is required for NeoOracle in strict identity/SGX mode")
			}
			log.Printf("Warning: ORACLE_HTTP_ALLOWLIST not set; allowing all outbound URLs (development/testing only)")
		}

		svc, err = neooracle.New(neooracle.Config{
			Marble:         m,
			SecretProvider: newServiceSecretsProvider(m, db, neooracle.ServiceID),
			URLAllowlist:   oracleAllowlist,
		})
	case "txproxy":
		svc, err = txproxy.New(txproxy.Config{
			Marble:      m,
			DB:          db,
			ChainClient: chainClient,
			Signer:      teeSigner,
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

func newServiceSecretsProvider(m *marble.Marble, db *database.Repository, serviceID string) secrets.Provider {
	if db == nil {
		return nil
	}

	var rawKey []byte
	if m != nil {
		if secret, ok := m.Secret(secrets.MasterKeyEnv); ok && len(secret) > 0 {
			rawKey = secret
		}
	}
	if len(rawKey) == 0 {
		rawKey = []byte(strings.TrimSpace(os.Getenv(secrets.MasterKeyEnv)))
	}

	if len(rawKey) == 0 {
		strict := runtime.StrictIdentityMode() || (m != nil && m.IsEnclave())
		if strict {
			log.Fatalf("CRITICAL: %s is required for %s secret access in production/SGX mode", secrets.MasterKeyEnv, serviceID)
		}
		return nil
	}

	repo := secretssupabase.NewRepository(db)
	manager, err := secrets.NewManager(repo, rawKey)
	if err != nil {
		log.Fatalf("CRITICAL: initialize secrets manager for %s: %v", serviceID, err)
	}
	return secrets.ServiceProvider{Manager: manager, ServiceID: serviceID}
}
