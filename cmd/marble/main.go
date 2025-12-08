// Package main provides the generic Marble entry point for all services.
// The service type is determined by the MARBLE_TYPE environment variable.
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

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	automation "github.com/R3E-Network/service_layer/services/automation/marble"
	confidential "github.com/R3E-Network/service_layer/services/confidential/marble"
	datafeeds "github.com/R3E-Network/service_layer/services/datafeeds/marble"
	mixer "github.com/R3E-Network/service_layer/services/mixer/marble"
	vrf "github.com/R3E-Network/service_layer/services/vrf/marble"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get service type from environment (injected by MarbleRun manifest)
	serviceType := os.Getenv("MARBLE_TYPE")
	if serviceType == "" {
		serviceType = os.Getenv("SERVICE_TYPE") // Fallback for local testing
	}
	if serviceType == "" {
		log.Fatal("MARBLE_TYPE environment variable required")
	}

	log.Printf("Starting %s service...", serviceType)

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
	svc, err := createService(serviceType, m, db)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Start service
	if err := svc.Start(ctx); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}

	// Get port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
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

// createService creates the appropriate service based on type.
// Available services:
// - vrf: Verifiable Random Function service
// - mixer: Privacy-preserving transaction mixing service
// - datafeeds: Aggregated price data feeds service
// - automation: Scheduled task and trigger-based automation service
// - confidential: Confidential computing service (planned)
func createService(serviceType string, m *marble.Marble, db *database.Repository) (*marble.Service, error) {
	switch serviceType {
	case "vrf":
		svc, err := vrf.New(vrf.Config{Marble: m, DB: db})
		if err != nil {
			return nil, err
		}
		return svc.Service, nil

	case "mixer":
		svc, err := mixer.New(mixer.Config{Marble: m, DB: db})
		if err != nil {
			return nil, err
		}
		return svc.Service, nil

	case "datafeeds":
		svc, err := datafeeds.New(datafeeds.Config{Marble: m, DB: db})
		if err != nil {
			return nil, err
		}
		return svc.Service, nil

	case "automation":
		svc, err := automation.New(automation.Config{Marble: m, DB: db})
		if err != nil {
			return nil, err
		}
		return svc.Service, nil

	case "confidential":
		svc, err := confidential.New(confidential.Config{Marble: m, DB: db})
		if err != nil {
			return nil, err
		}
		return svc.Service, nil

	default:
		return nil, fmt.Errorf("unknown service type: %s. Available: vrf, mixer, datafeeds, automation, confidential", serviceType)
	}
}
