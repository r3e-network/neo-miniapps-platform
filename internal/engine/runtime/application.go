package runtime

import (
	"context"
	"database/sql"
	"fmt"
	stdlog "log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	app "github.com/R3E-Network/service_layer/internal/app"
	appauth "github.com/R3E-Network/service_layer/internal/app/auth"
	"github.com/R3E-Network/service_layer/internal/app/httpapi"
	"github.com/R3E-Network/service_layer/internal/app/jam"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
	"github.com/R3E-Network/service_layer/internal/app/storage/postgres"
	"github.com/R3E-Network/service_layer/internal/config"
	engine "github.com/R3E-Network/service_layer/internal/engine"
	"github.com/R3E-Network/service_layer/internal/platform/database"
	"github.com/R3E-Network/service_layer/internal/platform/migrations"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Application wires core dependencies and manages the HTTP server lifecycle.
type Application struct {
	cfg        *config.Config
	log        *logger.Logger
	app        *app.Application
	httpSvc    *httpapi.Service
	listenAddr string
	db         *sql.DB
	engine     *engine.Engine
}

// Option customises how the runtime application is constructed.
type Option func(*builderOptions)

type builderOptions struct {
	cfg           *config.Config
	tokens        []string
	listenAddr    string
	runMigrations *bool
	slowThreshold int
}

func defaultBuilderOptions() builderOptions {
	val := true
	return builderOptions{runMigrations: &val}
}

// WithConfig injects an explicit configuration object.
func WithConfig(cfg *config.Config) Option {
	return func(opts *builderOptions) {
		if cfg != nil {
			opts.cfg = cfg
		}
	}
}

// WithAPITokens overrides the API tokens used by the HTTP service.
func WithAPITokens(tokens []string) Option {
	return func(opts *builderOptions) {
		clean := make([]string, 0, len(tokens))
		for _, token := range tokens {
			t := strings.TrimSpace(token)
			if t != "" {
				clean = append(clean, t)
			}
		}
		if len(clean) > 0 {
			opts.tokens = clean
		}
	}
}

// WithListenAddr sets the HTTP listen address explicitly.
func WithListenAddr(addr string) Option {
	return func(opts *builderOptions) {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			opts.listenAddr = addr
		}
	}
}

// WithRunMigrations controls whether embedded migrations run on startup.
func WithRunMigrations(enabled bool) Option {
	return func(opts *builderOptions) {
		opts.runMigrations = boolPtr(enabled)
	}
}

// WithSlowThresholdMS overrides the slow module threshold (ms) used by /system/status.
func WithSlowThresholdMS(ms int) Option {
	return func(opts *builderOptions) {
		if ms > 0 {
			opts.slowThreshold = ms
		}
	}
}

func boolPtr(v bool) *bool {
	value := v
	return &value
}

// NewApplication constructs a new application instance with default wiring.
func NewApplication(options ...Option) (*Application, error) {
	builder := defaultBuilderOptions()
	for _, opt := range options {
		if opt != nil {
			opt(&builder)
		}
	}

	cfg := builder.cfg
	if cfg == nil {
		loaded, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		cfg = loaded
	}

	runMigrations := true
	if builder.runMigrations != nil {
		runMigrations = *builder.runMigrations
	}
	if builder.slowThreshold <= 0 && cfg.Runtime.SlowMS > 0 {
		builder.slowThreshold = cfg.Runtime.SlowMS
	}

	logCfg := logger.LoggingConfig{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		FilePrefix: cfg.Logging.FilePrefix,
	}
	log := logger.New(logCfg)

	stores, db, err := buildStores(context.Background(), cfg, runMigrations)
	if err != nil {
		return nil, fmt.Errorf("configure stores: %w", err)
	}

	if db != nil {
		if err := assertTenantColumns(context.Background(), db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("schema validation: %w", err)
		}
	}

	application, err := app.New(stores, log, app.WithRuntimeConfig(AppRuntimeConfig(cfg)), app.WithManagerEnabled(false))
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("initialise application: %w", err)
	}

	persistent := db != nil
	secretKey := resolveSecretEncryptionKey(cfg)
	if persistent && secretKey == "" {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("secret encryption key must be configured when persistence is enabled")
	}

	if cipher, err := loadSecretsCipher(secretKey); err != nil {
		if persistent && db != nil {
			_ = db.Close()
		}
		if persistent {
			return nil, fmt.Errorf("initialise secret cipher: %w", err)
		}
		log.Warnf("failed to initialise secret cipher: %v", err)
	} else if cipher != nil && application.Secrets != nil {
		application.Secrets.SetCipher(cipher)
	} else if !persistent {
		log.Warn("secret encryption key not configured; storing secrets without encryption")
	}

	listenAddr := builder.listenAddr
	if listenAddr == "" {
		listenAddr = determineListenAddr(cfg)
	}
	tokens := builder.tokens
	if len(tokens) == 0 {
		tokens = resolveAPITokens(cfg)
	}
	if len(tokens) == 0 {
		log.Warn("no API tokens configured; prefer JWT /auth/login or wallet login")
	}

	jamCfg := jam.Config{
		Enabled:             cfg.Runtime.JAM.Enabled,
		Store:               cfg.Runtime.JAM.Store,
		PGDSN:               cfg.Runtime.JAM.PGDSN,
		AuthRequired:        cfg.Runtime.JAM.AuthRequired,
		AllowedTokens:       cfg.Runtime.JAM.AllowedTokens,
		RateLimitPerMinute:  cfg.Runtime.JAM.RateLimitPerMinute,
		MaxPreimageBytes:    cfg.Runtime.JAM.MaxPreimageBytes,
		MaxPendingPackages:  cfg.Runtime.JAM.MaxPendingPackages,
		LegacyListResponse:  cfg.Runtime.JAM.LegacyListResponse,
		AccumulatorsEnabled: cfg.Runtime.JAM.AccumulatorsEnabled,
		AccumulatorHash:     cfg.Runtime.JAM.AccumulatorHash,
	}
	jamCfg.Normalize()
	if jamCfg.Enabled {
		msg := fmt.Sprintf("JAM API enabled (store=%s, base=/jam)", jamCfg.Store)
		if jamCfg.Store == "memory" {
			msg += " — data is ephemeral"
		}
		log.Info(msg)
	}

	authMgr := buildAuthManager(cfg)
	if authMgr != nil && authMgr.HasUsers() && len(tokens) == 0 {
		log.Info("login enabled via /auth/login (JWT); API tokens also supported")
	}

	// Wire modules into the core engine for lifecycle management with explicit ordering.
	engineLogger := stdlog.New(log.Out, "", stdlog.LstdFlags)
	engineOrder := []string{
		"store-postgres",
		"core-application",
		"svc-neo-node",
		"svc-neo-indexer",
		"svc-chain-rpc",
		"svc-data-sources",
		"svc-contracts",
		"svc-crypto",
		"svc-rocketmq",
		"svc-accounts",
		"svc-functions",
		"svc-triggers",
		"svc-gasbank",
		"svc-service-bank",
		"svc-automation",
		"svc-pricefeed",
		"svc-datafeeds",
		"svc-datastreams",
		"svc-datalink",
		"svc-dta",
		"svc-confidential",
		"svc-cre",
		"svc-ccip",
		"svc-vrf",
		"svc-secrets",
		"svc-random",
		"svc-oracle",
		"runner-automation",
		"runner-pricefeed",
		"runner-oracle",
		"runner-gasbank",
		"svc-http",
	}
	eng := engine.New(engine.WithLogger(engineLogger), engine.WithOrder(engineOrder...))
	if db != nil {
		if err := eng.Register(newStoreModule(db)); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("register store module: %w", err)
		}
	}
	if err := eng.Register(newAppModule(application)); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("register core application module: %w", err)
	}
	if err := wrapServices(application, eng); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, err
	}
	if err := registerInfrastructureModules(eng, cfg); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, err
	}
	if err := validateRuntimeConfig(cfg); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("validate runtime config: %w", err)
	}
	applyDefaultModuleDeps(eng)

	modulesProvider := httpapi.EngineModuleProvider(eng)
	var busOpts []httpapi.ServiceOption
	if eng != nil {
		busOpts = append(busOpts, httpapi.WithBus(
			func(ctx context.Context, event string, payload any) error {
				err := eng.PublishEvent(ctx, event, payload)
				metrics.RecordBusFanout("event", err)
				return err
			},
			func(ctx context.Context, topic string, payload any) error {
				err := eng.PushData(ctx, topic, payload)
				metrics.RecordBusFanout("data", err)
				return err
			},
			func(ctx context.Context, payload any) ([]httpapi.ComputeResult, error) {
				results, err := eng.InvokeComputeAll(ctx, payload)
				metrics.RecordBusFanout("compute", err)
				out := make([]httpapi.ComputeResult, 0, len(results))
				for _, r := range results {
					errStr := ""
					if r.Err != nil {
						errStr = r.Err.Error()
					}
					out = append(out, httpapi.ComputeResult{
						Module: r.Module,
						Result: r.Result,
						Error:  errStr,
					})
				}
				return out, err
			},
		))
		busOpts = append(busOpts, httpapi.WithRPCEnginesOption(func() []engine.RPCEngine {
			return eng.RPCEngines()
		}))
	}
	if builder.slowThreshold > 0 {
		busOpts = append(busOpts, httpapi.WithStatusSlowThreshold(float64(builder.slowThreshold)))
		busOpts = append(busOpts, httpapi.WithRPCEnginesOption(func() []engine.RPCEngine {
			return eng.RPCEngines()
		}))
	}
	rpcBurst := cfg.Runtime.Chains.Burst
	if rpcBurst == 0 {
		rpcBurst = 3
	}
	busOpts = append(busOpts, httpapi.WithRPCPolicyOption(&httpapi.RPCPolicy{
		RequireTenant:      cfg.Runtime.Chains.RequireTenant,
		PerTenantPerMinute: cfg.Runtime.Chains.PerTenantPerMinute,
		PerTokenPerMinute:  cfg.Runtime.Chains.PerTokenPerMinute,
		Burst:              rpcBurst,
		AllowedMethods:     cfg.Runtime.Chains.AllowedMethods,
	}))
	httpSvc := httpapi.NewService(application, listenAddr, tokens, jamCfg, authMgr, log, db, modulesProvider, busOpts...)
	if err := application.Attach(httpSvc); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("attach http service: %w", err)
	}
	if err := registerModule(eng, "svc-http", "system", httpSvc, true); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("register http service: %w", err)
	}

	if err := applyRuntimeModuleConfig(eng, cfg, engineOrder, log); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, err
	}
	applyCoreModuleMetadata(eng)
	if eng != nil && log != nil {
		if missing := eng.MissingRequiredAPIs(); len(missing) > 0 {
			for mod, surfaces := range missing {
				log.Warnf("module %s requires API surfaces not provided: %v", mod, surfaces)
			}
			if cfg.Runtime.RequireAPIsStrict {
				return nil, fmt.Errorf("missing required API surfaces for modules: %v", missing)
			}
		}
	}

	return &Application{
		cfg:        cfg,
		log:        log,
		app:        application,
		httpSvc:    httpSvc,
		listenAddr: listenAddr,
		db:         db,
		engine:     eng,
	}, nil
}

// Run starts the application and blocks until the context is cancelled.
func (a *Application) Run(ctx context.Context) error {
	if a.engine != nil {
		if err := a.engine.Start(ctx); err != nil {
			return err
		}
		a.engine.ProbeReadiness(ctx)
	} else if err := a.app.Start(ctx); err != nil {
		return err
	}

	addr := a.listenAddr
	if a.httpSvc != nil {
		if actual := a.httpSvc.Addr(); actual != "" {
			addr = actual
		}
	}
	a.log.Infof("HTTP server listening on %s", addr)

	// Periodically probe readiness for modules.
	if a.engine != nil {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		go func() {
			for {
				select {
				case <-ticker.C:
					a.engine.ProbeReadiness(context.Background())
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	<-ctx.Done()
	return nil
}

// Shutdown gracefully stops the application and releases resources.
func (a *Application) Shutdown(ctx context.Context) error {
	if a.engine != nil {
		_ = a.engine.Stop(ctx)
		a.engine.MarkStopped()
	} else if err := a.app.Stop(ctx); err != nil {
		return err
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.log.WithError(err).Warn("error closing database connection")
		}
	}

	return nil
}

// App exposes the underlying application for integration harnesses.
func (a *Application) App() *app.Application {
	return a.app
}

// ListenAddr returns the resolved address the HTTP service is bound to. If the
// server has not started yet, the configured listen address is returned.
func (a *Application) ListenAddr() string {
	if a.httpSvc != nil {
		if addr := a.httpSvc.Addr(); addr != "" {
			return addr
		}
	}
	return a.listenAddr
}

func buildStores(ctx context.Context, cfg *config.Config, runMigrations bool) (app.Stores, *sql.DB, error) {
	driver := strings.TrimSpace(cfg.Database.Driver)
	dsn := strings.TrimSpace(cfg.Database.DSN)

	if driver == "" || dsn == "" {
		return app.Stores{}, nil, nil
	}

	if !strings.EqualFold(driver, "postgres") {
		return app.Stores{}, nil, fmt.Errorf("unsupported database driver %q", driver)
	}

	db, err := database.Open(ctx, dsn)
	if err != nil {
		return app.Stores{}, nil, err
	}

	configurePool(db, cfg.Database)

	if runMigrations {
		if err := migrations.Apply(ctx, db); err != nil {
			db.Close()
			return app.Stores{}, nil, fmt.Errorf("apply migrations: %w", err)
		}
	}

	store := postgres.New(db)

	return app.Stores{
		Accounts:         store,
		Functions:        store,
		Triggers:         store,
		GasBank:          store,
		Automation:       store,
		PriceFeeds:       store,
		DataFeeds:        store,
		DataStreams:      store,
		DataLink:         store,
		DTA:              store,
		Confidential:     store,
		Oracle:           store,
		Secrets:          store,
		CRE:              store,
		CCIP:             store,
		VRF:              store,
		WorkspaceWallets: store,
	}, db, nil
}

func configurePool(db *sql.DB, cfg config.DatabaseConfig) {
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}
}

// assertTenantColumns ensures the current schema includes tenant columns on critical tables.
// This prevents accidental launches against older databases that would bypass tenant scoping.
func assertTenantColumns(ctx context.Context, db *sql.DB) error {
	type check struct {
		table string
		col   string
	}
	tables := []check{
		{"app_accounts", "tenant"},
		{"app_functions", "tenant"},
		{"app_triggers", "tenant"},
		{"app_automation_jobs", "tenant"},
		{"app_price_feeds", "tenant"},
		{"app_oracle_sources", "tenant"},
		{"app_oracle_requests", "tenant"},
		{"app_vrf_keys", "tenant"},
		{"app_vrf_requests", "tenant"},
		{"app_ccip_lanes", "tenant"},
		{"app_ccip_messages", "tenant"},
		{"chainlink_datalink_channels", "tenant"},
		{"chainlink_datalink_deliveries", "tenant"},
		{"chainlink_data_feeds", "tenant"},
		{"chainlink_data_feed_updates", "tenant"},
		{"chainlink_datastreams", "tenant"},
		{"chainlink_datastream_frames", "tenant"},
		{"chainlink_dta_products", "tenant"},
		{"chainlink_dta_orders", "tenant"},
		{"confidential_enclaves", "tenant"},
		{"confidential_sealed_keys", "tenant"},
		{"confidential_attestations", "tenant"},
		{"app_cre_playbooks", "tenant"},
		{"app_cre_runs", "tenant"},
		{"app_gas_accounts", "tenant"},
		{"app_gas_transactions", "tenant"},
		{"app_gas_dead_letters", "tenant"},
		{"app_gas_withdrawal_approvals", "tenant"},
		{"app_gas_withdrawal_schedules", "tenant"},
		{"app_gas_settlement_attempts", "tenant"},
	}
	for _, tc := range tables {
		var exists bool
		if err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = $1 AND column_name = $2
			)
		`, tc.table, tc.col).Scan(&exists); err != nil {
			return fmt.Errorf("check column %s.%s: %w", tc.table, tc.col, err)
		}
		if !exists {
			return fmt.Errorf("database missing required column %s.%s for multi-tenant enforcement; run migrations", tc.table, tc.col)
		}
	}
	return nil
}

func determineListenAddr(cfg *config.Config) string {
	host := strings.TrimSpace(cfg.Server.Host)
	if host == "" {
		host = "0.0.0.0"
	}

	port := cfg.Server.Port
	if port <= 0 {
		port = 8080
	}

	return fmt.Sprintf("%s:%d", host, port)
}

func resolveAPITokens(cfg *config.Config) []string {
	var tokens []string
	if cfg != nil {
		for _, token := range cfg.Auth.Tokens {
			if t := strings.TrimSpace(token); t != "" {
				tokens = append(tokens, t)
			}
		}
	}
	tokens = append(tokens, splitAndTrim(os.Getenv("API_TOKENS"))...)
	if token := strings.TrimSpace(os.Getenv("API_TOKEN")); token != "" {
		tokens = append(tokens, token)
	}
	return tokens
}

func splitAndTrim(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			trimmed = append(trimmed, p)
		}
	}
	return trimmed
}

func resolveSecretEncryptionKey(cfg *config.Config) string {
	if value := strings.TrimSpace(os.Getenv("SECRET_ENCRYPTION_KEY")); value != "" {
		return value
	}
	if cfg != nil {
		return strings.TrimSpace(cfg.Security.SecretEncryptionKey)
	}
	return ""
}

// applyDefaultModuleDeps seeds sensible defaults so services start after core and persistence,
// and runners wait for their parent services. User config can override these afterwards.
func applyDefaultModuleDeps(eng *engine.Engine) {
	if eng == nil {
		return
	}
	has := func(name string) bool { return eng.Lookup(name) != nil }
	base := []string{}
	if has("core-application") {
		base = append(base, "core-application")
	}
	if has("store-postgres") {
		base = append(base, "store-postgres")
	}
	if len(base) == 0 {
		return
	}

	set := func(name string, deps ...string) {
		if !has(name) || len(deps) == 0 {
			return
		}
		eng.SetModuleDeps(name, deps...)
	}

	// Default every non-core/store module to depend on base.
	for _, name := range eng.Modules() {
		if name == "core-application" || name == "store-postgres" {
			continue
		}
		set(name, base...)
	}

	// Runners should start after their parent services (and base).
	set("runner-automation", append([]string{}, append(base, "svc-automation")...)...)
	set("runner-pricefeed", append([]string{}, append(base, "svc-pricefeed")...)...)
	set("runner-oracle", append([]string{}, append(base, "svc-oracle")...)...)
	set("runner-gasbank", append([]string{}, append(base, "svc-gasbank")...)...)
	set("svc-neo-node", base...)
	set("svc-neo-indexer", append(base, "svc-neo-node")...)
	set("svc-chain-rpc", base...)
	set("svc-data-sources", base...)
	set("svc-contracts", base...)
	set("svc-crypto", base...)
	set("svc-service-bank", append(base, "svc-gasbank")...)
	set("svc-rocketmq", base...)

	// HTTP transport should wait for core and store.
	set("svc-http", base...)
}

// applyCoreModuleMetadata annotates core modules with layer/capabilities for status/descriptors.
func applyCoreModuleMetadata(eng *engine.Engine) {
	if eng == nil {
		return
	}
	if eng.Lookup("store-postgres") != nil {
		eng.SetModuleLayer("store-postgres", "infra")
		eng.SetModuleCapabilities("store-postgres", "store", "postgres")
	}
	if eng.Lookup("core-application") != nil {
		eng.SetModuleLayer("core-application", "infra")
		eng.SetModuleCapabilities("core-application", "service-engine")
	}
	if eng.Lookup("svc-http") != nil {
		eng.SetModuleLayer("svc-http", "infra")
		eng.SetModuleCapabilities("svc-http", "http-edge", "api-gateway")
	}
}

func applyRuntimeModuleConfig(eng *engine.Engine, cfg *config.Config, intendedOrder []string, log *logger.Logger) error {
	if eng == nil || cfg == nil {
		return nil
	}
	modulesKnown := map[string]bool{}
	for _, name := range intendedOrder {
		if strings.TrimSpace(name) != "" {
			modulesKnown[name] = true
		}
	}
	for _, name := range eng.Modules() {
		modulesKnown[name] = true
	}
	strictUnknown := cfg.Runtime.UnknownModulesStrict
	for name, permCfg := range cfg.Runtime.BusPermissions {
		if !modulesKnown[name] {
			if strictUnknown {
				return fmt.Errorf("bus permissions configured for unknown module %q; known modules: %v", name, eng.Modules())
			}
			log.Warnf("bus permissions configured for unknown module %q; known modules: %v", name, eng.Modules())
			continue
		}
		perms := engine.BusPermissions{
			AllowEvents:  permCfg.Events == nil || *permCfg.Events,
			AllowData:    permCfg.Data == nil || *permCfg.Data,
			AllowCompute: permCfg.Compute == nil || *permCfg.Compute,
		}
		eng.SetBusPermissions(name, perms)
	}
	for name, deps := range cfg.Runtime.ModuleDeps {
		if !modulesKnown[name] {
			if strictUnknown {
				return fmt.Errorf("module deps configured for unknown module %q; known modules: %v", name, eng.Modules())
			}
			log.Warnf("module deps configured for unknown module %q; known modules: %v", name, eng.Modules())
			continue
		}
		eng.SetModuleDeps(name, deps...)
	}
	return nil
}

func buildAuthManager(cfg *config.Config) *appauth.Manager {
	if cfg == nil {
		return nil
	}
	// Allow env override for JWT secret/users.
	secret := strings.TrimSpace(cfg.Auth.JWTSecret)
	if envSecret := strings.TrimSpace(os.Getenv("AUTH_JWT_SECRET")); envSecret != "" {
		secret = envSecret
	}
	var users []appauth.User
	for _, u := range cfg.Auth.Users {
		if strings.TrimSpace(u.Username) == "" || strings.TrimSpace(u.Password) == "" {
			continue
		}
		users = append(users, appauth.User{
			Username: u.Username,
			Password: u.Password,
			Role:     u.Role,
		})
	}
	if envUsers := strings.TrimSpace(os.Getenv("AUTH_USERS")); envUsers != "" {
		for _, spec := range strings.Split(envUsers, ",") {
			parts := strings.Split(spec, ":")
			if len(parts) < 2 {
				continue
			}
			role := "user"
			if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" {
				role = strings.TrimSpace(parts[2])
			}
			users = append(users, appauth.User{
				Username: strings.TrimSpace(parts[0]),
				Password: strings.TrimSpace(parts[1]),
				Role:     role,
			})
		}
	}
	if len(users) == 0 || secret == "" {
		return nil
	}
	return appauth.NewManager(secret, users)
}
