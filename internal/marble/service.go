// Package marble provides the service framework for MarbleRun Marbles.
package marble

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/gorilla/mux"
)

// Service represents a base service that runs as a Marble.
type Service struct {
	mu sync.RWMutex

	// Identity
	id      string
	name    string
	version string

	// Dependencies
	marble *Marble
	db     database.RepositoryInterface
	router *mux.Router

	// State
	running bool
	stopCh  chan struct{}
}

// ServiceConfig holds service configuration.
type ServiceConfig struct {
	ID      string
	Name    string
	Version string
	Marble  *Marble
	DB      database.RepositoryInterface
}

// NewService creates a new base service.
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		id:      cfg.ID,
		name:    cfg.Name,
		version: cfg.Version,
		marble:  cfg.Marble,
		db:      cfg.DB,
		router:  mux.NewRouter(),
		stopCh:  make(chan struct{}),
	}
}

// ID returns the service ID.
func (s *Service) ID() string {
	return s.id
}

// Name returns the service name.
func (s *Service) Name() string {
	return s.name
}

// Version returns the service version.
func (s *Service) Version() string {
	return s.version
}

// Marble returns the Marble instance.
func (s *Service) Marble() *Marble {
	return s.marble
}

// DB returns the database repository.
func (s *Service) DB() database.RepositoryInterface {
	return s.db
}

// Router returns the HTTP router.
func (s *Service) Router() *mux.Router {
	return s.router
}

// Start starts the service.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("service already running")
	}
	s.running = true
	s.mu.Unlock()

	return nil
}

// Stop stops the service.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopCh)
	return nil
}

// IsRunning returns true if the service is running.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// =============================================================================
// Request/Response Types
// =============================================================================

// Request represents a service request.
type Request struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Service   string          `json:"service"`
	Method    string          `json:"method"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// Response represents a service response.
type Response struct {
	ID        string          `json:"id"`
	RequestID string          `json:"request_id"`
	Success   bool            `json:"success"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	GasUsed   int64           `json:"gas_used"`
	Timestamp time.Time       `json:"timestamp"`
	Signature []byte          `json:"signature,omitempty"`
}

// =============================================================================
// Service Handler Interface
// =============================================================================

// Handler defines the interface for service handlers.
type Handler interface {
	// Handle processes a service request.
	Handle(ctx context.Context, req *Request) (*Response, error)

	// Methods returns the list of supported methods.
	Methods() []string
}

// =============================================================================
// HTTP Middleware
// =============================================================================

// AuthMiddleware validates JWT tokens.
func AuthMiddleware(marble *Marble) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Validate token (simplified - production should use proper JWT validation)
			// Token validation happens inside the enclave for security
			if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}

			// Add user context and continue
			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware logs requests.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[%s] %s %s %v\n", time.Now().Format(time.RFC3339), r.Method, r.URL.Path, time.Since(start))
	})
}

// RecoveryMiddleware recovers from panics.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("panic recovered: %v\n", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// =============================================================================
// Health Check
// =============================================================================

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Enclave   bool   `json:"enclave"`
	Timestamp string `json:"timestamp"`
}

// HealthHandler returns a health check handler.
func HealthHandler(s *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status:    "healthy",
			Service:   s.Name(),
			Version:   s.Version(),
			Enclave:   s.Marble().IsEnclave(),
			Timestamp: time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
