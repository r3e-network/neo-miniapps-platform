// Package confidential provides confidential compute service.
//
// The Confidential Compute service allows users to execute custom JavaScript
// inside the TEE enclave with access to their secrets. This enables:
// - Privacy-preserving computation on sensitive data
// - Secure execution of business logic with verifiable results
// - Integration with external APIs using protected credentials
//
// Architecture:
// - Script execution via goja JavaScript runtime
// - Secure secret injection from user's secret store
// - Signed execution results for verification
// - Gas metering and resource limits
package confidential

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/dop251/goja"
	"github.com/google/uuid"
)

const (
	ServiceID   = "confidential"
	ServiceName = "Confidential Compute Service"
	Version     = "1.0.0"

	// Default execution timeout
	DefaultTimeout = 30 * time.Second

	// Max script size (100KB)
	MaxScriptSize = 100 * 1024

	// Gas cost per operation (simplified)
	GasPerInstruction = 1
)

// Service implements the Confidential Compute service.
type Service struct {
	*marble.Service
	masterKey []byte
}

// Config holds service configuration.
type Config struct {
	Marble *marble.Marble
	DB     *database.Repository
}

// New creates a new Confidential Compute service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{Service: base}

	if key, ok := cfg.Marble.Secret("COMPUTE_MASTER_KEY"); ok {
		s.masterKey = key
	}

	s.registerRoutes()
	return s, nil
}

func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	router.HandleFunc("/execute", s.handleExecute).Methods("POST")
	router.HandleFunc("/jobs/{id}", s.handleGetJob).Methods("GET")
	router.HandleFunc("/jobs", s.handleListJobs).Methods("GET")
}

// =============================================================================
// Request/Response Types
// =============================================================================

type ExecuteRequest struct {
	Script     string                 `json:"script"`
	EntryPoint string                 `json:"entry_point,omitempty"`
	Input      map[string]interface{} `json:"input,omitempty"`
	SecretRefs []string               `json:"secret_refs,omitempty"`
	Timeout    int                    `json:"timeout,omitempty"`
}

type ExecuteResponse struct {
	JobID     string                 `json:"job_id"`
	Status    string                 `json:"status"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Logs      []string               `json:"logs,omitempty"`
	Error     string                 `json:"error,omitempty"`
	GasUsed   int64                  `json:"gas_used"`
	StartedAt time.Time              `json:"started_at"`
	Duration  string                 `json:"duration,omitempty"`
}

// =============================================================================
// Handlers
// =============================================================================

func (s *Service) handleExecute(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Script == "" {
		http.Error(w, "script required", http.StatusBadRequest)
		return
	}

	if req.EntryPoint == "" {
		req.EntryPoint = "main"
	}

	result, err := s.Execute(r.Context(), userID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Service) handleGetJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "not found"})
}

func (s *Service) handleListJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{})
}

// =============================================================================
// Core Logic
// =============================================================================

// Execute runs code inside the TEE enclave.
func (s *Service) Execute(ctx context.Context, userID string, req *ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()
	jobID := uuid.New().String()

	response := &ExecuteResponse{
		JobID:     jobID,
		Status:    "running",
		StartedAt: startTime,
		Logs:      []string{},
	}

	// Validate script
	if req.Script == "" {
		response.Status = "failed"
		response.Error = "script cannot be empty"
		return response, nil
	}

	// Log execution start
	response.Logs = append(response.Logs,
		fmt.Sprintf("[%s] Starting execution", startTime.Format(time.RFC3339)),
		fmt.Sprintf("[%s] Entry point: %s", time.Now().Format(time.RFC3339), req.EntryPoint),
	)

	// Load secrets if referenced
	secrets := make(map[string][]byte)
	if len(req.SecretRefs) > 0 && s.DB() != nil {
		userSecrets, err := s.DB().GetSecrets(ctx, userID)
		if err != nil {
			response.Logs = append(response.Logs,
				fmt.Sprintf("[%s] Warning: failed to load secrets: %v", time.Now().Format(time.RFC3339), err),
			)
		} else {
			secretMap := make(map[string]*database.Secret)
			for i := range userSecrets {
				secretMap[userSecrets[i].Name] = &userSecrets[i]
			}

			for _, ref := range req.SecretRefs {
				secret, ok := secretMap[ref]
				if !ok {
					response.Logs = append(response.Logs,
						fmt.Sprintf("[%s] Warning: secret not found: %s", time.Now().Format(time.RFC3339), ref),
					)
					continue
				}
				if secret != nil && len(secret.EncryptedValue) > 0 {
					// Decrypt the secret value using master key
					decrypted := secret.EncryptedValue
					if s.masterKey != nil && len(s.masterKey) > 0 {
						dec, err := crypto.Decrypt(s.masterKey, secret.EncryptedValue)
						if err == nil {
							decrypted = dec
						}
					}
					secrets[ref] = decrypted
					response.Logs = append(response.Logs,
						fmt.Sprintf("[%s] Loaded secret: %s", time.Now().Format(time.RFC3339), ref),
					)
				}
			}
		}
	}

	// Execute JavaScript using goja runtime
	output, err := s.executeScript(ctx, req.Script, req.EntryPoint, req.Input, secrets)
	if err != nil {
		response.Status = "failed"
		response.Error = err.Error()
		response.Duration = time.Since(startTime).String()
		return response, nil
	}

	response.Status = "completed"
	response.Output = output
	response.GasUsed = int64(len(req.Script) * 10) // Simplified gas calculation
	response.Duration = time.Since(startTime).String()

	response.Logs = append(response.Logs,
		fmt.Sprintf("[%s] Execution completed successfully", time.Now().Format(time.RFC3339)),
	)

	return response, nil
}

// executeScript executes a JavaScript script inside the enclave using goja runtime.
func (s *Service) executeScript(ctx context.Context, script, entryPoint string, input map[string]interface{}, secrets map[string][]byte) (map[string]interface{}, error) {
	// Validate script size
	if len(script) > MaxScriptSize {
		return nil, fmt.Errorf("script exceeds maximum size of %d bytes", MaxScriptSize)
	}

	// Create goja runtime
	vm := goja.New()

	// Set up interrupt for timeout
	timeout := DefaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(timeout):
			vm.Interrupt("execution timeout")
		case <-done:
		}
	}()
	defer close(done)

	// Inject input as global 'input' object
	if input != nil {
		if err := vm.Set("input", input); err != nil {
			return nil, fmt.Errorf("failed to set input: %w", err)
		}
	} else {
		vm.Set("input", map[string]interface{}{})
	}

	// Inject secrets as global 'secrets' object (values as strings)
	secretsMap := make(map[string]string)
	for k, v := range secrets {
		secretsMap[k] = string(v)
	}
	if err := vm.Set("secrets", secretsMap); err != nil {
		return nil, fmt.Errorf("failed to set secrets: %w", err)
	}

	// Provide console.log for debugging
	console := vm.NewObject()
	logs := make([]string, 0)
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		logs = append(logs, fmt.Sprint(args...))
		return goja.Undefined()
	})
	vm.Set("console", console)

	// Provide crypto utilities
	cryptoObj := vm.NewObject()
	cryptoObj.Set("sha256", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		data := call.Arguments[0].String()
		hash := crypto.Hash256([]byte(data))
		return vm.ToValue(fmt.Sprintf("%x", hash))
	})
	cryptoObj.Set("randomBytes", func(call goja.FunctionCall) goja.Value {
		n := 32
		if len(call.Arguments) > 0 {
			n = int(call.Arguments[0].ToInteger())
		}
		if n > 1024 {
			n = 1024
		}
		bytes, err := crypto.GenerateRandomBytes(n)
		if err != nil {
			return goja.Undefined()
		}
		return vm.ToValue(fmt.Sprintf("%x", bytes))
	})
	vm.Set("crypto", cryptoObj)

	// Execute the script
	_, err := vm.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("script error: %w", err)
	}

	// Call the entry point function
	entryFn, ok := goja.AssertFunction(vm.Get(entryPoint))
	if !ok {
		return nil, fmt.Errorf("entry point '%s' is not a function", entryPoint)
	}

	result, err := entryFn(goja.Undefined())
	if err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}

	// Convert result to map
	output := make(map[string]interface{})
	if result != nil && result != goja.Undefined() && result != goja.Null() {
		exported := result.Export()
		switch v := exported.(type) {
		case map[string]interface{}:
			output = v
		default:
			output["result"] = exported
		}
	}

	// Add logs to output if any
	if len(logs) > 0 {
		output["_logs"] = logs
	}

	return output, nil
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
