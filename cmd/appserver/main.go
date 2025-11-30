package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	app "github.com/R3E-Network/service_layer/applications"
	"github.com/R3E-Network/service_layer/applications/auth"
	"github.com/R3E-Network/service_layer/applications/httpapi"
	"github.com/R3E-Network/service_layer/applications/jam"
	"github.com/R3E-Network/service_layer/pkg/blob"
	"github.com/R3E-Network/service_layer/pkg/config"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/pgnotify"
	"github.com/R3E-Network/service_layer/pkg/supabase"
	"github.com/R3E-Network/service_layer/pkg/version"
	"github.com/R3E-Network/service_layer/system/bootstrap"
	"github.com/R3E-Network/service_layer/system/platform/migrations"
)

func main() {
	addr := flag.String("addr", "", "HTTP listen address (defaults to :8080)")
	dsn := flag.String("dsn", "", "PostgreSQL DSN (or use DATABASE_URL env)")
	configPath := flag.String("config", "", "Path to configuration file")
	migrateFlag := flag.Bool("migrate", true, "Run database migrations on startup")
	apiTokens := flag.String("api-tokens", "", "Comma-separated API tokens")
	slowThreshold := flag.Int("slow-ms", 1000, "Slow query threshold in ms")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.FullVersion())
		return
	}

	// Load configuration
	var cfg *config.Config
	var err error
	if *configPath != "" {
		cfg, err = loadConfigFile(*configPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
	} else {
		cfg, err = config.Load()
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
	}

	// Resolve DSN
	dsnVal := resolveDSN(*dsn, cfg)
	if dsnVal == "" {
		log.Fatal("PostgreSQL DSN required (via --dsn, DATABASE_URL, or config)")
	}

	// Connect to database
	db, err := sql.Open("postgres", dsnVal)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	// Run migrations if requested
	if *migrateFlag {
		log.Println("Running database migrations...")
		if err := migrations.Apply(context.Background(), db); err != nil {
			log.Fatalf("run migrations: %v", err)
		}
		log.Println("Migrations completed successfully")
	}

	// Create stores - simplified: only database connection needed
	// Each service creates its own local PostgresStore
	stores := app.Stores{
		Database: db,
	}

	// Create logger
	appLogger := logger.NewDefault("service-layer")

	// Initialize Supabase client (optional, for enhanced features)
	var supabaseClient *supabase.Client
	var blobStorage *blob.Storage
	var eventBus *pgnotify.Bus

	if cfg.Supabase.ProjectURL != "" {
		var sbErr error
		supabaseClient, sbErr = supabase.New(supabase.Config{
			ProjectURL:     cfg.Supabase.ProjectURL,
			AnonKey:        cfg.Supabase.AnonKey,
			ServiceRoleKey: cfg.Supabase.ServiceRoleKey,
			JWTSecret:      cfg.Auth.SupabaseJWTSecret,
			GoTrueURL:      cfg.Auth.SupabaseGoTrueURL,
			StorageURL:     cfg.Supabase.StorageURL,
		})
		if sbErr != nil {
			log.Printf("WARNING: Supabase client init failed: %v", sbErr)
		} else {
			log.Printf("Supabase integration enabled:")
			log.Printf("  - Project URL: %s", cfg.Supabase.ProjectURL)
			log.Printf("  - Auth (GoTrue): %s", cfg.Auth.SupabaseGoTrueURL)
			log.Printf("  - Storage: %s", cfg.Supabase.StorageURL)

			// Initialize Supabase Storage for blob storage
			blobStorage = blob.NewStorage(supabaseClient, "blobs")
			log.Printf("  - Blob storage: Supabase Storage (bucket=blobs)")
		}
	}

	// Initialize event bus for database change notifications
	var busErr error
	eventBus, busErr = pgnotify.NewWithDB(db, dsnVal)
	if busErr != nil {
		log.Printf("WARNING: Event bus init failed: %v", busErr)
	} else {
		log.Println("PostgreSQL event bus (LISTEN/NOTIFY) enabled")
	}

	// Engine mode only: Android-style architecture
	log.Println("Starting Service Engine (Android-style architecture)...")
	ctx := context.Background()
	engineApp, err := app.NewEngineApplication(ctx, app.EngineAppConfig{
		Stores:         stores,
		Logger:         appLogger,
		SupabaseClient: supabaseClient,
		BlobStorage:    blobStorage,
		RealtimeClient: eventBus,
	})
	if err != nil {
		log.Fatalf("create engine application: %v", err)
	}

	// Log installed packages
	installed := engineApp.InstalledPackages()
	log.Printf("Loaded %d service packages:", len(installed))
	for _, pkg := range installed {
		log.Printf("  - %s@%s", pkg.Manifest.PackageID, pkg.Manifest.Version)
	}

	// Initialize Event System and User API (FullSystem)
	log.Println("Initializing event system and user API...")

	// Load contract type mappings from environment (format: "hash1:type1,hash2:type2")
	contractTypes := parseContractTypes(os.Getenv("CONTRACT_TYPE_MAPPINGS"))

	// Load secrets encryption key from environment (should be 32 bytes for AES-256)
	secretsKey := []byte(os.Getenv("SECRETS_ENCRYPT_KEY"))
	if len(secretsKey) == 0 {
		// Use a default key for development (NOT for production!)
		secretsKey = []byte("dev-secrets-key-32-bytes-long!!")
		log.Println("WARNING: Using default secrets encryption key. Set SECRETS_ENCRYPT_KEY in production!")
	}

	fullSystem, err := bootstrap.NewFullSystem(bootstrap.FullSystemConfig{
		DB:                db,
		Logger:            appLogger,
		ContractTypes:     contractTypes,
		SecretsEncryptKey: secretsKey,
		DispatcherWorkers: 4,
		RouterWorkers:     4,
	})
	if err != nil {
		log.Fatalf("create full system: %v", err)
	}

	// Resolve listen address
	listenAddr := *addr
	if listenAddr == "" {
		if cfg.Server.Port != 0 {
			listenAddr = fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		} else {
			listenAddr = ":8080"
		}
	}

	// Parse API tokens
	tokens := splitTokens(*apiTokens)
	if len(tokens) == 0 && len(cfg.Auth.Tokens) > 0 {
		tokens = cfg.Auth.Tokens
	}

	// Configure JAM (disabled by default)
	jamCfg := jam.Config{
		Enabled: false,
	}

	// Create auth manager
	authMgr := auth.NewManager("", nil)

	// Create JWT validator (Supabase)
	var jwtValidator httpapi.JWTValidator
	if cfg.Auth.SupabaseJWTSecret != "" {
		jwtValidator = httpapi.NewSupabaseJWTValidator(
			cfg.Auth.SupabaseJWTSecret,
			cfg.Auth.SupabaseJWTAud,
			cfg.Auth.SupabaseAdminRoles,
			cfg.Auth.SupabaseTenantClaim,
			cfg.Auth.SupabaseRoleClaim,
		)
		log.Printf("Supabase JWT authentication enabled (aud=%q, tenant_claim=%q)",
			cfg.Auth.SupabaseJWTAud, cfg.Auth.SupabaseTenantClaim)
	} else {
		log.Println("WARNING: Supabase JWT authentication not configured (set SUPABASE_JWT_SECRET)")
	}

	// Module provider from Engine
	moduleProvider := func() []httpapi.ModuleStatus {
		health := engineApp.ModulesHealth()
		var statuses []httpapi.ModuleStatus
		for _, h := range health {
			statuses = append(statuses, httpapi.ModuleStatus{
				Name:   h.Name,
				Domain: h.Domain,
			})
		}
		return statuses
	}

	// Build HTTP service options
	httpOpts := []httpapi.ServiceOption{
		httpapi.WithStatusSlowThreshold(float64(*slowThreshold)),
	}

	// Wire Supabase GoTrue URL for token refresh
	if cfg.Auth.SupabaseGoTrueURL != "" {
		httpOpts = append(httpOpts, httpapi.WithSupabaseGoTrueURL(cfg.Auth.SupabaseGoTrueURL))
		log.Printf("Supabase GoTrue configured: %s", cfg.Auth.SupabaseGoTrueURL)
	}

	// Register FullSystem API routes (User API endpoints)
	httpOpts = append(httpOpts, httpapi.WithExtraRoutesOption(fullSystem.UserAPI.Handler.RegisterRoutes))
	log.Println("User API routes registered at /api/v1/*")

	// Create HTTP service
	httpSvc := httpapi.NewService(
		engineApp,
		listenAddr,
		tokens,
		jamCfg,
		authMgr,
		jwtValidator,
		appLogger,
		db,
		moduleProvider,
		httpOpts...,
	)

	log.Printf("Service Layer %s starting on %s [Engine mode]", version.FullVersion(), listenAddr)

	// Start HTTP service
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Engine
	if err := engineApp.Start(runCtx); err != nil {
		log.Fatalf("start engine: %v", err)
	}

	// Start FullSystem (event processing and user API)
	if err := fullSystem.Start(runCtx); err != nil {
		log.Fatalf("start full system: %v", err)
	}
	log.Println("Event system and user API started")

	runErr := make(chan error, 1)
	go func() {
		runErr <- httpSvc.Start(runCtx)
	}()

	// Wait for address to be bound
	time.Sleep(100 * time.Millisecond)
	if boundAddr := httpSvc.Addr(); boundAddr != "" {
		log.Printf("HTTP server listening on %s", boundAddr)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-runErr:
		if err != nil {
			log.Fatalf("run: %v", err)
		}
		return
	case <-sigCh:
		log.Println("Shutdown signal received")
	}

	// Graceful shutdown
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop FullSystem
	fullSystem.Stop()
	log.Println("Event system and user API stopped")

	// Stop Engine
	if err := engineApp.Stop(shutdownCtx); err != nil {
		log.Printf("stop engine: %v", err)
	}

	if err := httpSvc.Stop(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func loadConfigFile(path string) (*config.Config, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return config.LoadFile(path)
	case ".json":
		return config.LoadConfig(path)
	default:
		if cfg, err := config.LoadFile(path); err == nil {
			return cfg, nil
		}
		return config.LoadConfig(path)
	}
}

func resolveDSN(flagDSN string, cfg *config.Config) string {
	if trimmed := strings.TrimSpace(flagDSN); trimmed != "" {
		return trimmed
	}
	if envDSN := strings.TrimSpace(os.Getenv("DATABASE_URL")); envDSN != "" {
		return envDSN
	}
	if cfg == nil {
		return ""
	}
	if cfg.Database.DSN != "" {
		return strings.TrimSpace(cfg.Database.DSN)
	}
	if cfg.Database.Host != "" && cfg.Database.Name != "" {
		return cfg.Database.ConnectionString()
	}
	return ""
}

func splitTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	var trimmed []string
	for _, part := range parts {
		if p := strings.TrimSpace(part); p != "" {
			trimmed = append(trimmed, p)
		}
	}
	return trimmed
}

// parseContractTypes parses contract type mappings from a string.
// Format: "hash1:type1,hash2:type2" (e.g., "0x1234:oraclehub,0x5678:vrf")
func parseContractTypes(value string) map[string]string {
	result := make(map[string]string)
	value = strings.TrimSpace(value)
	if value == "" {
		return result
	}
	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			hash := strings.TrimSpace(parts[0])
			contractType := strings.TrimSpace(parts[1])
			if hash != "" && contractType != "" {
				result[hash] = contractType
			}
		}
	}
	return result
}
