package runtime

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/R3E-Network/service_layer/internal/api/httpserver"
	"github.com/R3E-Network/service_layer/internal/api/httpserver/router"
	"github.com/R3E-Network/service_layer/internal/config"
	"github.com/R3E-Network/service_layer/internal/services/functions"
	"github.com/R3E-Network/service_layer/internal/services/health"
	"github.com/R3E-Network/service_layer/internal/services/secrets"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Application wires core dependencies and manages the HTTP server lifecycle.
type Application struct {
	cfg          *config.Config
	log          *logger.Logger
	httpServer   *httpserver.Server
	healthSvc    health.Service
	functionsSvc functions.Service
	secretsSvc   secrets.Service
	db           *sql.DB
}

type secretProvider struct {
	svc secrets.Service
}

func (p secretProvider) GetSecret(ctx context.Context, id string) (string, error) {
	sec, err := p.svc.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return sec.Value, nil
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

	healthSvc := health.NewService()
	funcStore, secretStore, db, err := buildStores(cfg)
	if err != nil {
		return nil, fmt.Errorf("configure stores: %w", err)
	}
	secretOpts := []secrets.Option{}
	if raw := os.Getenv("SECRET_ENCRYPTION_KEY"); raw != "" {
		if key, err := parseEncryptionKey(raw); err != nil {
			log.Warnf("SECRET_ENCRYPTION_KEY invalid: %v", err)
		} else if cipher, err := secrets.NewAESCipher(key); err != nil {
			log.Warnf("failed to initialise secret cipher: %v", err)
		} else {
			secretOpts = append(secretOpts, secrets.WithCipher(cipher))
		}
	}
	secretsSvc := secrets.NewService(secretStore, secretOpts...)
	functionsSvc := functions.NewService(funcStore, functions.WithSecretProvider(secretProvider{svc: secretsSvc}))

	mux := router.New(log, healthSvc, functionsSvc, secretsSvc)
	httpSrv := httpserver.New(cfg.Server, log, mux)

	return &Application{
		cfg:          cfg,
		log:          log,
		httpServer:   httpSrv,
		healthSvc:    healthSvc,
		functionsSvc: functionsSvc,
		secretsSvc:   secretsSvc,
		db:           db,
	}, nil
}

// Run starts the HTTP server and blocks until the context is cancelled.
func (a *Application) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.log.Infof("HTTP server listening on %s:%d", a.cfg.Server.Host, a.cfg.Server.Port)
		if err := a.httpServer.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the HTTP server.
func (a *Application) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.log.WithError(err).Warn("error closing database connection")
		}
	}

	return nil
}

func buildStores(cfg *config.Config) (functions.Store, secrets.Store, *sql.DB, error) {
	if cfg.Database.Driver == "" || cfg.Database.DSN == "" {
		return nil, nil, nil, fmt.Errorf("database configuration is required (driver and dsn)")
	}

	db, err := openDatabase(cfg.Database)
	if err != nil {
		return nil, nil, nil, err
	}

	functionStore := functions.NewSQLStore(db)
	secretStore := secrets.NewSQLStore(db)

	return functionStore, secretStore, db, nil
}

func openDatabase(cfg config.DatabaseConfig) (*sql.DB, error) {
	if cfg.Driver == "" {
		return nil, fmt.Errorf("database driver not configured")
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database dsn not configured")
	}

	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, err
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func parseEncryptionKey(value string) ([]byte, error) {
	if value == "" {
		return nil, errors.New("missing encryption key")
	}

	// raw bytes
	if l := len(value); l == 16 || l == 24 || l == 32 {
		return []byte(value), nil
	}

	// base64
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		if l := len(decoded); l == 16 || l == 24 || l == 32 {
			return decoded, nil
		}
	}

	// hex
	if decoded, err := hex.DecodeString(value); err == nil {
		if l := len(decoded); l == 16 || l == 24 || l == 32 {
			return decoded, nil
		}
	}

	return nil, errors.New("must be raw 16/24/32 byte string or base64/hex encoding of that length")
}
