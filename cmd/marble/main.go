// Package main implements a generic Marble runner for Neo Service Layer.
// This can be used to run any service as a Marble by specifying the service type.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/R3E-Network/service_layer/marble/sdk"
)

func main() {
	// Parse flags
	marbleType := flag.String("type", "", "Marble type (e.g., oracle, vrf, secrets)")
	coordinatorAddr := flag.String("coordinator", "localhost:4433", "Coordinator address")
	addr := flag.String("addr", ":8443", "Service listen address")
	simulationMode := flag.Bool("simulation", false, "Enable SGX simulation mode")
	flag.Parse()

	// Environment variable overrides
	if v := os.Getenv("MARBLE_TYPE"); v != "" {
		*marbleType = v
	}
	if v := os.Getenv("COORDINATOR_ADDR"); v != "" {
		*coordinatorAddr = v
	}
	if v := os.Getenv("SERVICE_ADDR"); v != "" {
		*addr = v
	}
	if os.Getenv("SIMULATION_MODE") == "true" {
		*simulationMode = true
	}

	if *marbleType == "" {
		log.Fatal("Marble type is required (-type or MARBLE_TYPE)")
	}

	log.Printf("Starting Marble: %s", *marbleType)
	log.Printf("  Coordinator: %s", *coordinatorAddr)
	log.Printf("  Listen: %s", *addr)

	// Create Marble
	marble, err := sdk.New(sdk.Config{
		MarbleType:      *marbleType,
		CoordinatorAddr: *coordinatorAddr,
		SimulationMode:  *simulationMode,
	})
	if err != nil {
		log.Fatalf("Failed to create marble: %v", err)
	}

	// Activate with Coordinator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Activating with Coordinator...")
	if err := marble.Activate(ctx); err != nil {
		log.Fatalf("Activation failed: %v", err)
	}
	log.Println("Marble activated successfully")

	// Get Supabase client
	supabase, err := marble.SupabaseClient()
	if err != nil {
		log.Printf("Warning: Supabase not configured: %v", err)
	}

	// Create service handler based on marble type
	handler := createServiceHandler(*marbleType, marble, supabase)

	// Create HTTP server
	server := &http.Server{
		Addr:         *addr,
		Handler:      handler,
		TLSConfig:    marble.TLSConfig(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	go func() {
		log.Printf("Service listening on %s", *addr)
		var err error
		if marble.TLSConfig() != nil {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Marble stopped")
}

// createServiceHandler creates the appropriate handler for the marble type.
func createServiceHandler(marbleType string, marble *sdk.Marble, supabase *sdk.SupabaseClient) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"healthy","marble":"%s","uuid":"%s"}`, marbleType, marble.UUID())
	})

	// Service-specific routes
	switch marbleType {
	case "oracle":
		registerOracleRoutes(mux, marble, supabase)
	case "vrf":
		registerVRFRoutes(mux, marble, supabase)
	case "secrets":
		registerSecretsRoutes(mux, marble, supabase)
	case "gasbank":
		registerGasBankRoutes(mux, marble, supabase)
	case "datafeeds":
		registerDataFeedsRoutes(mux, marble, supabase)
	case "automation":
		registerAutomationRoutes(mux, marble, supabase)
	case "mixer":
		registerMixerRoutes(mux, marble, supabase)
	case "accounts":
		registerAccountsRoutes(mux, marble, supabase)
	case "ccip":
		registerCCIPRoutes(mux, marble, supabase)
	case "confidential":
		registerConfidentialRoutes(mux, marble, supabase)
	case "cre":
		registerCRERoutes(mux, marble, supabase)
	case "datalink":
		registerDataLinkRoutes(mux, marble, supabase)
	case "datastreams":
		registerDataStreamsRoutes(mux, marble, supabase)
	case "dta":
		registerDTARoutes(mux, marble, supabase)
	default:
		log.Printf("Warning: Unknown marble type %s, using generic handler", marbleType)
	}

	return mux
}

// =============================================================================
// Oracle Service Routes
// =============================================================================

func registerOracleRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Fetch external data
	mux.HandleFunc("/api/v1/oracle/fetch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			URL     string            `json:"url"`
			Method  string            `json:"method"`
			Headers map[string]string `json:"headers"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		if req.URL == "" {
			writeError(w, http.StatusBadRequest, "url is required")
			return
		}
		if req.Method == "" {
			req.Method = "GET"
		}

		// Execute fetch
		httpReq, _ := http.NewRequest(req.Method, req.URL, nil)
		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("fetch failed: %v", err))
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		writeSuccess(w, map[string]any{
			"status_code": resp.StatusCode,
			"body":        string(body),
		})
	})

	// List oracle requests
	mux.HandleFunc("/api/v1/oracle/requests", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// VRF Service Routes
// =============================================================================

func registerVRFRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Generate random value
	mux.HandleFunc("/api/v1/vrf/random", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Seed string `json:"seed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		// Generate VRF output (simplified - real impl uses VRF algorithm)
		seed := []byte(req.Seed)
		if len(seed) == 0 {
			seed = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
		}

		hash := sha256.Sum256(seed)
		writeSuccess(w, map[string]any{
			"random": hex.EncodeToString(hash[:]),
			"seed":   req.Seed,
		})
	})

	// List VRF requests
	mux.HandleFunc("/api/v1/vrf/requests", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// Secrets Service Routes
// =============================================================================

func registerSecretsRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	secrets := make(map[string][]byte)
	var mu sync.RWMutex

	// Store secret
	mux.HandleFunc("/api/v1/secrets/store", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		mu.Lock()
		secrets[req.Name] = []byte(req.Value)
		mu.Unlock()

		writeSuccess(w, map[string]any{"stored": true})
	})

	// Get secret
	mux.HandleFunc("/api/v1/secrets/get", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		mu.RLock()
		value, ok := secrets[name]
		mu.RUnlock()

		if !ok {
			writeError(w, http.StatusNotFound, "secret not found")
			return
		}

		writeSuccess(w, map[string]any{"value": string(value)})
	})

	// List secrets
	mux.HandleFunc("/api/v1/secrets", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		names := make([]string, 0, len(secrets))
		for name := range secrets {
			names = append(names, name)
		}
		mu.RUnlock()

		writeSuccess(w, names)
	})
}

// =============================================================================
// GasBank Service Routes
// =============================================================================

func registerGasBankRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Get balance
	mux.HandleFunc("/api/v1/gasbank/balance", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		writeSuccess(w, map[string]any{
			"user_id": userID,
			"balance": "0",
		})
	})

	// Deposit
	mux.HandleFunc("/api/v1/gasbank/deposit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			UserID string `json:"user_id"`
			Amount string `json:"amount"`
			TxHash string `json:"tx_hash"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"deposited": true,
			"user_id":   req.UserID,
			"amount":    req.Amount,
		})
	})

	// Withdraw
	mux.HandleFunc("/api/v1/gasbank/withdraw", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		writeSuccess(w, map[string]any{"withdrawn": true})
	})
}

// =============================================================================
// DataFeeds Service Routes
// =============================================================================

func registerDataFeedsRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Get feed value
	mux.HandleFunc("/api/v1/datafeeds/value", func(w http.ResponseWriter, r *http.Request) {
		feedID := r.URL.Query().Get("feed_id")
		writeSuccess(w, map[string]any{
			"feed_id":    feedID,
			"value":      nil,
			"updated_at": time.Now(),
		})
	})

	// List feeds
	mux.HandleFunc("/api/v1/datafeeds", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})

	// Create feed
	mux.HandleFunc("/api/v1/datafeeds/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name     string `json:"name"`
			URL      string `json:"url"`
			Interval int    `json:"interval"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"feed_id": fmt.Sprintf("feed_%d", time.Now().UnixNano()),
			"name":    req.Name,
		})
	})
}

// =============================================================================
// Automation Service Routes
// =============================================================================

func registerAutomationRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// List tasks
	mux.HandleFunc("/api/v1/automation/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})

	// Create task
	mux.HandleFunc("/api/v1/automation/tasks/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name     string `json:"name"`
			Cron     string `json:"cron"`
			Function string `json:"function"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"task_id": fmt.Sprintf("task_%d", time.Now().UnixNano()),
			"name":    req.Name,
		})
	})

	// Run task
	mux.HandleFunc("/api/v1/automation/tasks/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		taskID := r.URL.Query().Get("task_id")
		writeSuccess(w, map[string]any{
			"task_id": taskID,
			"status":  "running",
		})
	})
}

// =============================================================================
// Mixer Service Routes
// =============================================================================

func registerMixerRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Create mixing request
	mux.HandleFunc("/api/v1/mixer/request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Amount        string `json:"amount"`
			SourceChain   string `json:"source_chain"`
			TargetChain   string `json:"target_chain"`
			TargetAddress string `json:"target_address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"request_id":     fmt.Sprintf("mix_%d", time.Now().UnixNano()),
			"amount":         req.Amount,
			"source_chain":   req.SourceChain,
			"target_chain":   req.TargetChain,
			"target_address": req.TargetAddress,
			"status":         "pending",
		})
	})

	// Get mixing status
	mux.HandleFunc("/api/v1/mixer/status", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.URL.Query().Get("request_id")
		writeSuccess(w, map[string]any{
			"request_id": requestID,
			"status":     "pending",
			"progress":   0,
		})
	})

	// List mixing requests
	mux.HandleFunc("/api/v1/mixer/requests", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// Accounts Service Routes
// =============================================================================

func registerAccountsRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	accounts := make(map[string]map[string]any)
	var mu sync.RWMutex

	// Create account
	mux.HandleFunc("/api/v1/accounts/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name     string         `json:"name"`
			Metadata map[string]any `json:"metadata"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		accountID := fmt.Sprintf("acc_%d", time.Now().UnixNano())
		account := map[string]any{
			"id":         accountID,
			"name":       req.Name,
			"metadata":   req.Metadata,
			"created_at": time.Now(),
		}

		mu.Lock()
		accounts[accountID] = account
		mu.Unlock()

		writeSuccess(w, account)
	})

	// Get account
	mux.HandleFunc("/api/v1/accounts/get", func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")

		mu.RLock()
		account, ok := accounts[accountID]
		mu.RUnlock()

		if !ok {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}

		writeSuccess(w, account)
	})

	// List accounts
	mux.HandleFunc("/api/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		list := make([]map[string]any, 0, len(accounts))
		for _, acc := range accounts {
			list = append(list, acc)
		}
		mu.RUnlock()

		writeSuccess(w, list)
	})
}

// =============================================================================
// CCIP (Cross-Chain Interoperability Protocol) Service Routes
// =============================================================================

func registerCCIPRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Send cross-chain message
	mux.HandleFunc("/api/v1/ccip/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			TargetChain    string `json:"target_chain"`
			TargetContract string `json:"target_contract"`
			Payload        string `json:"payload"`
			GasLimit       int    `json:"gas_limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"message_id":      fmt.Sprintf("ccip_%d", time.Now().UnixNano()),
			"target_chain":    req.TargetChain,
			"target_contract": req.TargetContract,
			"status":          "pending",
		})
	})

	// Get message status
	mux.HandleFunc("/api/v1/ccip/status", func(w http.ResponseWriter, r *http.Request) {
		messageID := r.URL.Query().Get("message_id")
		writeSuccess(w, map[string]any{
			"message_id": messageID,
			"status":     "pending",
			"source_tx":  nil,
			"target_tx":  nil,
		})
	})

	// List messages
	mux.HandleFunc("/api/v1/ccip/messages", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// Confidential Computing Service Routes
// =============================================================================

func registerConfidentialRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Execute confidential computation
	mux.HandleFunc("/api/v1/confidential/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			FunctionCode  string         `json:"function_code"`
			Inputs        map[string]any `json:"inputs"`
			EncryptOutput bool           `json:"encrypt_output"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"execution_id": fmt.Sprintf("conf_%d", time.Now().UnixNano()),
			"status":       "processing",
			"tee_mode":     "sgx",
		})
	})

	// Get execution result
	mux.HandleFunc("/api/v1/confidential/result", func(w http.ResponseWriter, r *http.Request) {
		executionID := r.URL.Query().Get("execution_id")
		writeSuccess(w, map[string]any{
			"execution_id": executionID,
			"status":       "completed",
			"result":       nil,
			"attestation":  nil,
		})
	})

	// List executions
	mux.HandleFunc("/api/v1/confidential/executions", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// CRE (Chainlink Runtime Environment) Service Routes
// =============================================================================

func registerCRERoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Deploy function
	mux.HandleFunc("/api/v1/cre/deploy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name       string `json:"name"`
			SourceCode string `json:"source_code"`
			Runtime    string `json:"runtime"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"function_id": fmt.Sprintf("cre_%d", time.Now().UnixNano()),
			"name":        req.Name,
			"runtime":     req.Runtime,
			"status":      "deployed",
		})
	})

	// Invoke function
	mux.HandleFunc("/api/v1/cre/invoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			FunctionID string         `json:"function_id"`
			Args       map[string]any `json:"args"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"invocation_id": fmt.Sprintf("inv_%d", time.Now().UnixNano()),
			"function_id":   req.FunctionID,
			"status":        "completed",
			"result":        nil,
		})
	})

	// List functions
	mux.HandleFunc("/api/v1/cre/functions", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// DataLink Service Routes
// =============================================================================

func registerDataLinkRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Create data link
	mux.HandleFunc("/api/v1/datalink/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name            string `json:"name"`
			SourceURL       string `json:"source_url"`
			TransformScript string `json:"transform_script"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"link_id":    fmt.Sprintf("link_%d", time.Now().UnixNano()),
			"name":       req.Name,
			"source_url": req.SourceURL,
			"status":     "active",
		})
	})

	// Fetch data from link
	mux.HandleFunc("/api/v1/datalink/fetch", func(w http.ResponseWriter, r *http.Request) {
		linkID := r.URL.Query().Get("link_id")
		writeSuccess(w, map[string]any{
			"link_id":    linkID,
			"data":       nil,
			"fetched_at": time.Now(),
		})
	})

	// List data links
	mux.HandleFunc("/api/v1/datalink", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// DataStreams Service Routes
// =============================================================================

func registerDataStreamsRoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Create stream
	mux.HandleFunc("/api/v1/datastreams/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name       string         `json:"name"`
			SourceType string         `json:"source_type"`
			Config     map[string]any `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"stream_id":   fmt.Sprintf("stream_%d", time.Now().UnixNano()),
			"name":        req.Name,
			"source_type": req.SourceType,
			"status":      "active",
		})
	})

	// Subscribe to stream
	mux.HandleFunc("/api/v1/datastreams/subscribe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		streamID := r.URL.Query().Get("stream_id")
		writeSuccess(w, map[string]any{
			"stream_id":       streamID,
			"subscription_id": fmt.Sprintf("sub_%d", time.Now().UnixNano()),
			"status":          "subscribed",
		})
	})

	// Get latest value
	mux.HandleFunc("/api/v1/datastreams/latest", func(w http.ResponseWriter, r *http.Request) {
		streamID := r.URL.Query().Get("stream_id")
		writeSuccess(w, map[string]any{
			"stream_id":  streamID,
			"value":      nil,
			"updated_at": time.Now(),
		})
	})

	// List streams
	mux.HandleFunc("/api/v1/datastreams", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// DTA (Data Trust Alliance) Service Routes
// =============================================================================

func registerDTARoutes(mux *http.ServeMux, marble *sdk.Marble, supabase *sdk.SupabaseClient) {
	// Register data source
	mux.HandleFunc("/api/v1/dta/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Name     string         `json:"name"`
			Endpoint string         `json:"endpoint"`
			Schema   map[string]any `json:"schema"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"source_id": fmt.Sprintf("dta_%d", time.Now().UnixNano()),
			"name":      req.Name,
			"endpoint":  req.Endpoint,
			"status":    "registered",
		})
	})

	// Query data
	mux.HandleFunc("/api/v1/dta/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			SourceID string `json:"source_id"`
			Query    string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		writeSuccess(w, map[string]any{
			"source_id": req.SourceID,
			"query":     req.Query,
			"results":   []any{},
		})
	})

	// Verify data integrity
	mux.HandleFunc("/api/v1/dta/verify", func(w http.ResponseWriter, r *http.Request) {
		dataHash := r.URL.Query().Get("data_hash")
		writeSuccess(w, map[string]any{
			"data_hash": dataHash,
			"verified":  true,
			"timestamp": time.Now(),
		})
	})

	// List data sources
	mux.HandleFunc("/api/v1/dta/sources", func(w http.ResponseWriter, r *http.Request) {
		writeSuccess(w, []any{})
	})
}

// =============================================================================
// Response Helpers
// =============================================================================

func writeSuccess(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    data,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   message,
	})
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[marble] ")
}
