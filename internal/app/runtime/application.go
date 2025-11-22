package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/httpapi"
	"github.com/R3E-Network/service_layer/internal/app/jam"
	"github.com/R3E-Network/service_layer/internal/app/storage/postgres"
	"github.com/R3E-Network/service_layer/internal/config"
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
}

// Option customises how the runtime application is constructed.
type Option func(*builderOptions)

type builderOptions struct {
	cfg           *config.Config
	tokens        []string
	listenAddr    string
	runMigrations *bool
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

	application, err := app.New(stores, log, app.WithRuntimeConfig(AppRuntimeConfig(cfg)))
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
		log.Warn("no API tokens configured; HTTP API will reject requests")
	}

	jamCfg := jam.Config{
		Enabled: cfg.Runtime.JAM.Enabled,
		Store:   cfg.Runtime.JAM.Store,
		PGDSN:   cfg.Runtime.JAM.PGDSN,
	}
	jamCfg.Normalize()
	if jamCfg.Enabled {
		log.Infof("JAM API enabled (store=%s)", jamCfg.Store)
	}

	httpSvc := httpapi.NewService(application, listenAddr, tokens, jamCfg, log)
	if err := application.Attach(httpSvc); err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("attach http service: %w", err)
	}

	return &Application{
		cfg:        cfg,
		log:        log,
		app:        application,
		httpSvc:    httpSvc,
		listenAddr: listenAddr,
		db:         db,
	}, nil
}

// Run starts the application and blocks until the context is cancelled.
func (a *Application) Run(ctx context.Context) error {
	if err := a.app.Start(ctx); err != nil {
		return err
	}

	a.log.Infof("HTTP server listening on %s", a.listenAddr)

	<-ctx.Done()
	return nil
}

// Shutdown gracefully stops the application and releases resources.
func (a *Application) Shutdown(ctx context.Context) error {
	if err := a.app.Stop(ctx); err != nil {
		return err
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.log.WithError(err).Warn("error closing database connection")
		}
	}

	return nil
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
