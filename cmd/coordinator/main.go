// Package main implements the Coordinator server for Neo Service Layer.
// The Coordinator is the MarbleRun-based control plane that manages
// the confidential microservices mesh.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/R3E-Network/service_layer/coordinator/api"
	"github.com/R3E-Network/service_layer/coordinator/core"
	"github.com/R3E-Network/service_layer/manifest"
)

func main() {
	// Parse flags
	addr := flag.String("addr", ":4433", "Coordinator listen address")
	manifestPath := flag.String("manifest", "", "Path to manifest file")
	sealMode := flag.String("seal-mode", "ProductKey", "Seal mode: ProductKey, UniqueKey, Disabled")
	simulationMode := flag.Bool("simulation", false, "Enable SGX simulation mode")
	supabaseURL := flag.String("supabase-url", "", "Supabase URL")
	supabaseKey := flag.String("supabase-key", "", "Supabase service key")
	flag.Parse()

	// Environment variable overrides
	if v := os.Getenv("COORDINATOR_ADDR"); v != "" {
		*addr = v
	}
	if v := os.Getenv("MANIFEST_PATH"); v != "" {
		*manifestPath = v
	}
	if v := os.Getenv("SEAL_MODE"); v != "" {
		*sealMode = v
	}
	if os.Getenv("SIMULATION_MODE") == "true" {
		*simulationMode = true
	}
	if v := os.Getenv("SUPABASE_URL"); v != "" {
		*supabaseURL = v
	}
	if v := os.Getenv("SUPABASE_KEY"); v != "" {
		*supabaseKey = v
	}

	log.Println("Starting Neo Service Layer Coordinator")
	log.Printf("  Address: %s", *addr)
	log.Printf("  Seal Mode: %s", *sealMode)
	log.Printf("  Simulation: %v", *simulationMode)

	// Create Coordinator
	cfg := core.Config{
		SealMode:       core.SealMode(*sealMode),
		SimulationMode: *simulationMode,
		SupabaseURL:    *supabaseURL,
		SupabaseKey:    *supabaseKey,
		DNSNames:       []string{"coordinator", "localhost"},
	}

	coordinator, err := core.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create coordinator: %v", err)
	}

	// Start Coordinator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := coordinator.Start(ctx); err != nil {
		log.Fatalf("Failed to start coordinator: %v", err)
	}
	log.Println("Coordinator started")

	// Load manifest if provided
	if *manifestPath != "" {
		m, err := manifest.Load(*manifestPath)
		if err != nil {
			log.Fatalf("Failed to load manifest: %v", err)
		}

		if _, err := coordinator.SetManifest(ctx, m); err != nil {
			log.Fatalf("Failed to set manifest: %v", err)
		}
		log.Printf("Manifest loaded from %s", *manifestPath)
	}

	// Create API handler
	handler := api.NewHandler(coordinator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create HTTPS server
	server := &http.Server{
		Addr:    *addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Coordinator API listening on %s", *addr)
		// For development, use HTTP; for production, use TLS
		if *simulationMode {
			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
		} else {
			// TODO: Use TLS certificates from TLS manager
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

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	if err := coordinator.Stop(shutdownCtx); err != nil {
		log.Printf("Coordinator stop error: %v", err)
	}

	log.Println("Coordinator stopped")
}

func init() {
	// Set log format
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[coordinator] ")
}
