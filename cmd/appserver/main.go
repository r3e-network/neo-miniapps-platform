package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/httpapi"
	"github.com/R3E-Network/service_layer/internal/app/services/secrets"
	"github.com/R3E-Network/service_layer/internal/app/storage/postgres"
	"github.com/R3E-Network/service_layer/internal/config"
	"github.com/R3E-Network/service_layer/internal/platform/database"
	"github.com/R3E-Network/service_layer/internal/platform/migrations"
)

func main() {
	addr := flag.String("addr", "", "HTTP listen address (defaults to config or :8080)")
	dsn := flag.String("dsn", "", "PostgreSQL DSN (overrides config/env; in-memory storage when empty)")
	configPath := flag.String("config", "", "Path to configuration file (JSON or YAML)")
	runMigrations := flag.Bool("migrate", true, "run embedded database migrations on startup (ignored for in-memory)")
	apiTokensFlag := flag.String("api-tokens", "", "comma-separated API tokens for HTTP authentication")
	flag.Parse()

	var cfg *config.Config

	if trimmed := strings.TrimSpace(*configPath); trimmed != "" {
		loaded, err := loadConfigFile(trimmed)
		if err != nil {
			log.Fatalf("load config %s: %v", trimmed, err)
		}
		cfg = loaded
	}

	stores := app.Stores{}

	var (
		db  *sql.DB
		err error
	)

	rootCtx := context.Background()

	dsnVal := resolveDSN(*dsn, cfg)

	if dsnVal != "" {
		db, err = database.Open(rootCtx, dsnVal)
		if err != nil {
			log.Fatalf("connect to postgres: %v", err)
		}
		configurePool(db, cfg)
		if *runMigrations {
			if err := migrations.Apply(rootCtx, db); err != nil {
				log.Fatalf("apply migrations: %v", err)
			}
		}
		store := postgres.New(db)
		stores = app.Stores{
			Accounts:   store,
			Functions:  store,
			Triggers:   store,
			GasBank:    store,
			Automation: store,
			PriceFeeds: store,
			Oracle:     store,
			Secrets:    store,
		}
	}

	if db != nil {
		defer db.Close()
	}

	application, err := app.New(stores, nil)
	if err != nil {
		log.Fatalf("initialise application: %v", err)
	}
	configureSecretsCipher(application.Secrets, dsnVal)

	listenAddr := determineAddr(*addr, cfg)
	tokens := resolveAPITokens(*apiTokensFlag)

	httpService := httpapi.NewService(application, listenAddr, tokens, nil)
	if err := application.Attach(httpService); err != nil {
		log.Fatalf("attach http service: %v", err)
	}

	ctx := context.Background()
	if err := application.Start(ctx); err != nil {
		log.Fatalf("start application: %v", err)
	}
	log.Printf("service layer listening on %s", listenAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Stop(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}

func determineAddr(flagAddr string, cfg *config.Config) string {
	addr := strings.TrimSpace(flagAddr)
	if addr != "" {
		return addr
	}
	if cfg != nil {
		host := strings.TrimSpace(cfg.Server.Host)
		port := cfg.Server.Port
		if port != 0 {
			if host == "" {
				host = "0.0.0.0"
			}
			return fmt.Sprintf("%s:%d", host, port)
		}
	}
	return ":8080"
}

func configurePool(db *sql.DB, cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.Database.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)
	}
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

func resolveAPITokens(flagTokens string) []string {
	var tokens []string
	tokens = append(tokens, splitTokens(flagTokens)...)
	tokens = append(tokens, splitTokens(os.Getenv("API_TOKENS"))...)
	if token := strings.TrimSpace(os.Getenv("API_TOKEN")); token != "" {
		tokens = append(tokens, token)
	}
	return tokens
}

func splitTokens(value string) []string {
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

func configureSecretsCipher(svc *secrets.Service, dsn string) {
	if svc == nil {
		return
	}
	key := strings.TrimSpace(os.Getenv("SECRET_ENCRYPTION_KEY"))
	if key == "" {
		if strings.TrimSpace(dsn) != "" {
			log.Fatal("SECRET_ENCRYPTION_KEY must be set when using persistent storage")
		}
		log.Println("WARNING: SECRET_ENCRYPTION_KEY not set; storing secrets without encryption")
		return
	}

	rawKey, err := decodeSecretKey(key)
	if err != nil {
		log.Fatalf("invalid SECRET_ENCRYPTION_KEY: %v", err)
	}

	cipher, err := secrets.NewAESCipher(rawKey)
	if err != nil {
		log.Fatalf("initialise secret cipher: %v", err)
	}
	svc.SetCipher(cipher)
}

func decodeSecretKey(value string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && validKeyLength(decoded) {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && validKeyLength(decoded) {
		return decoded, nil
	}
	raw := []byte(value)
	if validKeyLength(raw) {
		return raw, nil
	}
	return nil, fmt.Errorf("expected 16, 24, or 32 byte key")
}

func validKeyLength(key []byte) bool {
	switch len(key) {
	case 16, 24, 32:
		return true
	default:
		return false
	}
}
