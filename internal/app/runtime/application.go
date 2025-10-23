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

// NewApplication constructs a new application instance with default wiring.
func NewApplication() (*Application, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logCfg := logger.LoggingConfig{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		FilePrefix: cfg.Logging.FilePrefix,
	}
	log := logger.New(logCfg)

	stores, db, err := buildStores(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("configure stores: %w", err)
	}

	application, err := app.New(stores, log)
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("initialise application: %w", err)
	}

	if cipher, err := loadSecretsCipher(); err != nil {
		log.Warnf("failed to initialise secret cipher: %v", err)
	} else if cipher != nil && application.Secrets != nil {
		application.Secrets.SetCipher(cipher)
	}

	listenAddr := determineListenAddr(cfg)
	httpSvc := httpapi.NewService(application, listenAddr, loadAPITokens(cfg), log)
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

func buildStores(ctx context.Context, cfg *config.Config) (app.Stores, *sql.DB, error) {
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

	if err := migrations.Apply(ctx, db); err != nil {
		db.Close()
		return app.Stores{}, nil, fmt.Errorf("apply migrations: %w", err)
	}

	store := postgres.New(db)

	return app.Stores{
		Accounts:   store,
		Functions:  store,
		Triggers:   store,
		GasBank:    store,
		Automation: store,
		PriceFeeds: store,
		Oracle:     store,
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

func loadAPITokens(cfg *config.Config) []string {
	_ = cfg
	tokens := splitAndTrim(os.Getenv("API_TOKENS"))
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
