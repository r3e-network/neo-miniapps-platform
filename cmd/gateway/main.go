// Package main provides the API Gateway Marble entry point.
package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
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

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	sllogging "github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/marble"
	slmetrics "github.com/R3E-Network/service_layer/internal/metrics"
	slmiddleware "github.com/R3E-Network/service_layer/internal/middleware"
	"github.com/R3E-Network/service_layer/internal/runtime"
)

var (
	jwtSecret             []byte
	jwtExpiry             = 24 * time.Hour
	headerGateAuditLogger *sllogging.Logger
)

// =============================================================================
// JWT Claims
// =============================================================================

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// =============================================================================
// Neo Signature Verification
// =============================================================================

func decodeWalletBytes(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")
	if trimmed == "" {
		return nil, fmt.Errorf("empty value")
	}

	decodedHex, hexErr := hex.DecodeString(trimmed)
	if hexErr == nil {
		return decodedHex, nil
	}

	for _, decoder := range []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	} {
		if decoded, err := decoder(trimmed); err == nil {
			return decoded, nil
		}
	}

	return nil, fmt.Errorf("unsupported encoding: %v", hexErr)
}

// verifyNeoSignature verifies a Neo N3 wallet signature.
func verifyNeoSignature(address, message, signatureHex, publicKeyHex string) bool {
	signature, err := decodeWalletBytes(signatureHex)
	if err != nil {
		log.Printf("Failed to decode signature: %v", err)
		return false
	}

	pubKeyBytes, err := decodeWalletBytes(publicKeyHex)
	if err != nil {
		log.Printf("Failed to decode public key: %v", err)
		return false
	}

	pubKey, err := crypto.PublicKeyFromBytes(pubKeyBytes)
	if err != nil {
		log.Printf("Failed to parse public key: %v", err)
		return false
	}

	derivedAddress := crypto.PublicKeyToAddress(pubKey)
	if derivedAddress != address {
		log.Printf("Address mismatch: expected %s, derived %s", address, derivedAddress)
		return false
	}

	return crypto.Verify(pubKey, []byte(message), signature)
}

// =============================================================================
// Main Entry Point
// =============================================================================

func main() {
	ctx := context.Background()

	// Initialize Marble
	m, err := marble.New(marble.Config{
		MarbleType: "gateway",
	})
	if err != nil {
		log.Fatalf("Failed to create marble: %v", err)
	}

	if initErr := m.Initialize(ctx); initErr != nil {
		log.Fatalf("Failed to initialize marble: %v", initErr)
	}

	// In production/SGX mode, require MarbleRun-injected mTLS credentials for the
	// internal service mesh, even if the public listener runs behind an ingress.
	// This prevents accidentally weakening trust boundaries by running plaintext
	// service-to-service traffic.
	if (runtime.StrictIdentityMode() || m.IsEnclave()) && m.TLSConfig() == nil {
		log.Fatalf("CRITICAL: MarbleRun TLS credentials are required in production/SGX mode (missing MARBLE_CERT/MARBLE_KEY/MARBLE_ROOT_CA)")
	}

	// Load JWT secret - REQUIRED in production
	if secret, ok := m.Secret("JWT_SECRET"); ok {
		jwtSecret = secret
	} else if envSecret := os.Getenv("JWT_SECRET"); envSecret != "" {
		jwtSecret = []byte(envSecret)
	} else {
		env := runtime.Env()
		strict := runtime.StrictIdentityMode() || m.IsEnclave()

		// In development/testing outside enclaves, allow an insecure default to keep
		// local onboarding friction low. Never allow this in production or on
		// enclave hardware.
		if !strict && (env == runtime.Development || env == runtime.Testing) {
			log.Printf("WARNING: Using insecure default JWT secret - DO NOT USE IN PRODUCTION")
			jwtSecret = []byte("development-insecure-secret-32bytes-minimum-" + string(env))
		} else {
			log.Fatalf("CRITICAL: JWT_SECRET is required in production. Set via MarbleRun secrets or JWT_SECRET env var")
		}
	}

	// Validate JWT secret length
	if len(jwtSecret) < 32 {
		log.Fatalf("CRITICAL: JWT_SECRET must be at least 32 bytes for security")
	}

	// JWT expiry (default 24h). Keep token/session/cookie TTL aligned.
	if raw := strings.TrimSpace(os.Getenv("JWT_EXPIRY")); raw != "" {
		ttl, parseErr := time.ParseDuration(raw)
		if parseErr != nil {
			log.Fatalf("CRITICAL: invalid JWT_EXPIRY %q: %v", raw, parseErr)
		}
		if ttl <= 0 {
			log.Fatalf("CRITICAL: JWT_EXPIRY must be > 0, got %s", ttl)
		}
		jwtExpiry = ttl
	}

	loadAdminAllowlistsFromEnv()

	oauthTokensKey, oauthTokensKeyOK, oauthTokensKeyErr := oauthTokensMasterKeyBytes(m)
	if oauthTokensKeyErr != nil {
		log.Fatalf("CRITICAL: %v", oauthTokensKeyErr)
	}

	googleID, googleSecret, _ := getOAuthConfig(m, "google")
	githubID, githubSecret, _ := getOAuthConfig(m, "github")
	oauthEnabled := (googleID != "" && googleSecret != "") || (githubID != "" && githubSecret != "")
	if oauthEnabled && runtime.StrictIdentityMode() && !oauthTokensKeyOK {
		log.Fatalf("CRITICAL: OAUTH_TOKENS_MASTER_KEY is required when OAuth is enabled in production/SGX mode")
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
	if oauthTokensKeyOK {
		if err := db.SetOAuthTokensMasterKey(oauthTokensKey); err != nil {
			log.Fatalf("CRITICAL: configure oauth token encryption: %v", err)
		}
	}

	// Create router and register routes
	router := mux.NewRouter()

	logger := sllogging.NewFromEnv("gateway")
	headerGateAuditLogger = logger
	router.Use(slmiddleware.LoggingMiddleware(logger))
	router.Use(slmiddleware.NewRecoveryMiddleware(logger).Handler)
	if slmetrics.Enabled() {
		metricsCollector := slmetrics.Init("gateway")
		router.Use(slmiddleware.MetricsMiddleware("gateway", metricsCollector))
		router.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	}
	router.Use(slmiddleware.NewCORSMiddleware(&slmiddleware.CORSConfig{
		AllowedOrigins:         corsAllowedOrigins(),
		AllowedMethods:         []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:         []string{"Content-Type", "Authorization", "X-API-Key", "X-Trace-ID"},
		ExposedHeaders:         []string{"X-Trace-ID"},
		AllowCredentials:       true,
		MaxAgeSeconds:          3600,
		PreflightStatus:        http.StatusOK,
		RejectDisallowedOrigin: true,
	}).Handler)

	// Cap request bodies to reduce memory/CPU DoS risk.
	// This is especially important for the public-facing gateway.
	router.Use(slmiddleware.NewBodyLimitMiddleware(0).Handler)

	rateLimiter, stopRateLimiter := newGatewayRateLimiter(logger)
	if stopRateLimiter != nil {
		defer stopRateLimiter()
	}

	headerGateSecret := strings.TrimSpace(os.Getenv("X_SHARED_SECRET"))
	if secret, ok := m.Secret("X_SHARED_SECRET"); ok && len(secret) > 0 {
		headerGateSecret = strings.TrimSpace(string(secret))
	}

	// In production/SGX mode, the Header Gate is a required defense-in-depth layer.
	if (runtime.StrictIdentityMode() || m.IsEnclave()) && headerGateSecret == "" {
		log.Fatalf("CRITICAL: X_SHARED_SECRET is required in production/SGX mode (Header Gate)")
	}
	if headerGateSecret == "" {
		log.Printf("WARNING: Header Gate disabled (X_SHARED_SECRET not set)")
	}

	registerRoutes(router, db, m, rateLimiter, headerGateSecret)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	go func() {
		tlsMode := strings.ToLower(strings.TrimSpace(os.Getenv("GATEWAY_TLS_MODE")))
		switch tlsMode {
		case "", "off", "false", "0":
			log.Printf("Gateway starting on port %s (HTTP)", port)
			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
		case "mtls":
			if m.TLSConfig() == nil {
				log.Fatalf("GATEWAY_TLS_MODE=mtls requires MarbleRun TLS credentials")
			}
			server.TLSConfig = m.TLSConfig()
			log.Printf("Gateway starting on port %s (mTLS)", port)
			if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
		case "tls":
			if m.TLSConfig() == nil {
				log.Fatalf("GATEWAY_TLS_MODE=tls requires MarbleRun TLS credentials")
			}
			cfg := m.TLSConfig().Clone()
			cfg.ClientAuth = tls.NoClientCert
			cfg.ClientCAs = nil
			server.TLSConfig = cfg
			log.Printf("Gateway starting on port %s (TLS)", port)
			if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
		default:
			log.Fatalf("Invalid GATEWAY_TLS_MODE %q (expected: off|tls|mtls)", tlsMode)
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
}

func corsAllowedOrigins() []string {
	allowed := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if allowed == "" {
		allowed = strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
	}
	if allowed == "" {
		allowed = "http://localhost:3000,http://localhost:5173"
	}

	parts := strings.Split(allowed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func oauthTokensMasterKeyBytes(m *marble.Marble) (key []byte, ok bool, err error) {
	if m != nil {
		if secret, ok := m.Secret("OAUTH_TOKENS_MASTER_KEY"); ok && len(secret) > 0 {
			if len(secret) != 32 {
				return nil, false, fmt.Errorf("OAUTH_TOKENS_MASTER_KEY must be 32 bytes (raw) or a hex-encoded 32-byte key, got %d bytes", len(secret))
			}
			return secret, true, nil
		}
	}

	raw := strings.TrimSpace(os.Getenv("OAUTH_TOKENS_MASTER_KEY"))
	if raw == "" {
		return nil, false, nil
	}

	normalized := strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X")
	keyBytes, decodeErr := hex.DecodeString(normalized)
	if decodeErr != nil {
		return nil, false, fmt.Errorf("OAUTH_TOKENS_MASTER_KEY must be hex-encoded: %w", decodeErr)
	}
	if len(keyBytes) != 32 {
		return nil, false, fmt.Errorf("OAUTH_TOKENS_MASTER_KEY must decode to 32 bytes, got %d", len(keyBytes))
	}
	return keyBytes, true, nil
}

func newGatewayRateLimiter(logger *sllogging.Logger) (limiter *slmiddleware.RateLimiter, stop func()) {
	enabledRaw := strings.TrimSpace(strings.ToLower(os.Getenv("RATE_LIMIT_ENABLED")))
	if enabledRaw == "" {
		return nil, nil
	}
	switch enabledRaw {
	case "1", "true", "yes", "on":
	default:
		return nil, nil
	}

	requests := 100
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_REQUESTS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			requests = parsed
		}
	}

	window := time.Minute
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_WINDOW")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			window = parsed
		}
	}

	burst := requests
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_BURST")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			burst = parsed
		}
	}

	rl := slmiddleware.NewRateLimiterWithWindow(requests, window, burst, logger)
	stop = rl.StartCleanup(5 * time.Minute)
	return rl, stop
}

// registerRoutes sets up all HTTP routes
func registerRoutes(router *mux.Router, db *database.Repository, m *marble.Marble, rateLimiter *slmiddleware.RateLimiter, headerGateSecret string) {
	// Health check
	router.HandleFunc("/health", healthHandler(m)).Methods("GET")
	router.HandleFunc("/ready", readyHandler(db, m)).Methods("GET")
	router.HandleFunc("/attestation", attestationHandler(m)).Methods("GET")
	var masterKey http.Handler = masterKeyHandler(m)
	if rateLimiter != nil {
		masterKey = rateLimiter.Handler(masterKey)
	}
	router.Handle("/master-key", masterKey).Methods("GET")

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()
	if headerGateSecret != "" {
		api.Use(HeaderGateMiddleware(headerGateSecret))
	}

	// Public auth routes
	public := api.PathPrefix("").Subrouter()
	if rateLimiter != nil {
		public.Use(rateLimiter.Handler)
	}
	public.HandleFunc("/auth/nonce", nonceHandler(db)).Methods("POST")
	public.HandleFunc("/auth/register", registerHandler(db)).Methods("POST")
	public.HandleFunc("/auth/login", loginHandler(db)).Methods("POST")
	public.HandleFunc("/auth/logout", logoutHandler(db)).Methods("POST")

	// OAuth routes
	public.HandleFunc("/auth/google", googleAuthHandler(m)).Methods("GET")
	public.HandleFunc("/auth/google/callback", googleCallbackHandler(db, m)).Methods("GET")
	public.HandleFunc("/auth/github", githubAuthHandler(m)).Methods("GET")
	public.HandleFunc("/auth/github/callback", githubCallbackHandler(db, m)).Methods("GET")

	// Protected routes
	protected := api.PathPrefix("").Subrouter()
	protected.Use(authMiddleware(db))
	if rateLimiter != nil {
		protected.Use(rateLimiter.Handler)
	}

	// User profile
	protected.HandleFunc("/me", meHandler(db)).Methods("GET")

	// API Key management
	protected.HandleFunc("/apikeys", listAPIKeysHandler(db)).Methods("GET")
	protected.HandleFunc("/apikeys", createAPIKeyHandler(db)).Methods("POST")
	protected.HandleFunc("/apikeys/{id}", revokeAPIKeyHandler(db)).Methods("DELETE")

	// OAuth provider management
	protected.HandleFunc("/oauth/providers", listOAuthProvidersHandler(db)).Methods("GET")
	protected.HandleFunc("/oauth/providers/{id}", unlinkOAuthProviderHandler(db)).Methods("DELETE")

	// Wallet management
	protected.HandleFunc("/wallets", listWalletsHandler(db)).Methods("GET")
	protected.HandleFunc("/wallets", addWalletHandler(db)).Methods("POST")
	protected.HandleFunc("/wallets/{id}/primary", setPrimaryWalletHandler(db)).Methods("POST")
	protected.HandleFunc("/wallets/{id}/verify", verifyWalletHandler(db)).Methods("POST")
	protected.HandleFunc("/wallets/{id}", deleteWalletHandler(db)).Methods("DELETE")

	// Gas Bank
	protected.HandleFunc("/gasbank/account", getGasBankAccountHandler(db)).Methods("GET")
	protected.HandleFunc("/gasbank/deposit", createDepositHandler(db)).Methods("POST")
	protected.HandleFunc("/gasbank/deposits", listDepositsHandler(db)).Methods("GET")
	protected.HandleFunc("/gasbank/transactions", listTransactionsHandler(db)).Methods("GET")

	// Service proxy routes
	protected.HandleFunc("/vrf/{path:.*}", proxyHandler("vrf", m)).Methods("GET", "POST")
	protected.HandleFunc("/neorand/{path:.*}", proxyHandler("neorand", m)).Methods("GET", "POST")
	protected.HandleFunc("/neovault/{path:.*}", proxyHandler("neovault", m)).Methods("GET", "POST")
	protected.HandleFunc("/neofeeds/{path:.*}", proxyHandler("neofeeds", m)).Methods("GET", "POST")
	protected.HandleFunc("/neoflow/{path:.*}", proxyHandler("neoflow", m)).Methods("GET", "POST", "PUT", "DELETE")
	protected.HandleFunc("/neocompute/{path:.*}", proxyHandler("neocompute", m)).Methods("GET", "POST")
	protected.HandleFunc("/neostore/{path:.*}", proxyHandler("neostore", m)).Methods("GET", "POST", "PUT", "DELETE")
	protected.HandleFunc("/neooracle/{path:.*}", proxyHandler("neooracle", m)).Methods("GET", "POST")

	// Backward-compatible aliases.
	protected.HandleFunc("/secrets/{path:.*}", proxyHandler("secrets", m)).Methods("GET", "POST", "PUT", "DELETE")
	protected.HandleFunc("/oracle/{path:.*}", proxyHandler("oracle", m)).Methods("GET", "POST")
}
