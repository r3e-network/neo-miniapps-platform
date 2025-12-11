// Package main provides the API Gateway Marble entry point.
package main

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

var jwtSecret []byte

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

// verifyNeoSignature verifies a Neo N3 wallet signature.
func verifyNeoSignature(address, message, signatureHex, publicKeyHex string) bool {
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		log.Printf("Failed to decode signature: %v", err)
		return false
	}

	pubKeyBytes, err := hex.DecodeString(publicKeyHex)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Marble
	m, err := marble.New(marble.Config{
		MarbleType: "gateway",
	})
	if err != nil {
		log.Fatalf("Failed to create marble: %v", err)
	}

	if err := m.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize marble: %v", err)
	}

	// Load JWT secret - REQUIRED in production
	if secret, ok := m.Secret("JWT_SECRET"); ok {
		jwtSecret = secret
	} else if envSecret := os.Getenv("JWT_SECRET"); envSecret != "" {
		jwtSecret = []byte(envSecret)
	} else {
		// In development mode (MARBLE_ENV=development), allow insecure default
		env := os.Getenv("MARBLE_ENV")
		if env == "development" || env == "testing" {
			log.Printf("WARNING: Using insecure default JWT secret - DO NOT USE IN PRODUCTION")
			jwtSecret = []byte("dev-only-secret-32bytes-minimum-" + env)
		} else {
			log.Fatalf("CRITICAL: JWT_SECRET is required in production. Set via MarbleRun secrets or JWT_SECRET env var")
		}
	}

	// Validate JWT secret length
	if len(jwtSecret) < 32 {
		log.Fatalf("CRITICAL: JWT_SECRET must be at least 32 bytes for security")
	}

	// Initialize database
	dbClient, err := database.NewClient(database.Config{})
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}
	db := database.NewRepository(dbClient)

	// Create router and register routes
	router := mux.NewRouter()
	router.Use(marble.LoggingMiddleware)
	router.Use(marble.RecoveryMiddleware)
	router.Use(corsMiddleware)

	registerRoutes(router, db, m)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		TLSConfig:    m.TLSConfig(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Gateway starting on port %s", port)
		if m.TLSConfig() != nil {
			if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
		} else {
			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
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

// registerRoutes sets up all HTTP routes
func registerRoutes(router *mux.Router, db *database.Repository, m *marble.Marble) {
	// Health check
	router.HandleFunc("/health", healthHandler(m)).Methods("GET")
	router.HandleFunc("/attestation", attestationHandler(m)).Methods("GET")

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Public auth routes
	api.HandleFunc("/auth/nonce", nonceHandler(db)).Methods("POST")
	api.HandleFunc("/auth/register", registerHandler(db)).Methods("POST")
	api.HandleFunc("/auth/login", loginHandler(db)).Methods("POST")
	api.HandleFunc("/auth/logout", logoutHandler(db)).Methods("POST")

	// OAuth routes
	api.HandleFunc("/auth/google", googleAuthHandler(m)).Methods("GET")
	api.HandleFunc("/auth/google/callback", googleCallbackHandler(db, m)).Methods("GET")
	api.HandleFunc("/auth/github", githubAuthHandler(m)).Methods("GET")
	api.HandleFunc("/auth/github/callback", githubCallbackHandler(db, m)).Methods("GET")

	// Protected routes
	protected := api.PathPrefix("").Subrouter()
	protected.Use(authMiddleware(db, m))

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
	protected.HandleFunc("/vrf/{path:.*}", proxyHandler("vrf")).Methods("GET", "POST")
	protected.HandleFunc("/neovault/{path:.*}", proxyHandler("neovault")).Methods("GET", "POST")
	protected.HandleFunc("/neofeeds/{path:.*}", proxyHandler("neofeeds")).Methods("GET", "POST")
	protected.HandleFunc("/neoflow/{path:.*}", proxyHandler("neoflow")).Methods("GET", "POST", "PUT", "DELETE")
	protected.HandleFunc("/neocompute/{path:.*}", proxyHandler("neocompute")).Methods("GET", "POST")
}
