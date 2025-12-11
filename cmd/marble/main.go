// Package main provides the generic Marble entry point for all Neo services.
// The service type is determined by the MARBLE_TYPE environment variable.
// Each service is a separate Marble in MarbleRun, running in its own TEE enclave.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/config"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"

	// Neo service imports
	neoaccounts "github.com/R3E-Network/service_layer/services/neoaccounts/marble"
	neocompute "github.com/R3E-Network/service_layer/services/neocompute/marble"
	neofeeds "github.com/R3E-Network/service_layer/services/neofeeds/marble"
	neoflow "github.com/R3E-Network/service_layer/services/neoflow/marble"
	neooracle "github.com/R3E-Network/service_layer/services/neooracle/marble"
	neorand "github.com/R3E-Network/service_layer/services/neorand/marble"
	neostore "github.com/R3E-Network/service_layer/services/neostore/marble"
	neovault "github.com/R3E-Network/service_layer/services/neovault/marble"
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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	if err := m.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize marble: %v", err)
	}

	// Initialize database
	dbClient, err := database.NewClient(database.Config{})
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}
	db := database.NewRepository(dbClient)

	// Create service based on type
	var svc ServiceRunner
	switch serviceType {
	case "neoaccounts":
		svc, err = neoaccounts.New(neoaccounts.Config{Marble: m, DB: db})
	case "neocompute":
		svc, err = neocompute.New(neocompute.Config{Marble: m, DB: db})
	case "neofeeds":
		svc, err = neofeeds.New(neofeeds.Config{Marble: m, DB: db})
	case "neoflow":
		svc, err = neoflow.New(neoflow.Config{Marble: m, DB: db})
	case "neooracle":
		svc, err = neooracle.New(neooracle.Config{Marble: m})
	case "neorand":
		svc, err = neorand.New(neorand.Config{Marble: m, DB: db})
	case "neostore":
		svc, err = neostore.New(neostore.Config{Marble: m})
	case "neovault":
		svc, err = neovault.New(neovault.Config{Marble: m, DB: db})
	default:
		log.Fatalf("Unknown service: %s. Available: %v", serviceType, availableServices)
	}
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

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
		Addr:         ":" + port,
		Handler:      svc.Router(),
		TLSConfig:    m.TLSConfig(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
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
