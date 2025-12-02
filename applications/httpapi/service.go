package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	app "github.com/R3E-Network/service_layer/applications"
	"github.com/R3E-Network/service_layer/pkg/metrics"
	"github.com/R3E-Network/service_layer/applications/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/storage/postgres"
	engine "github.com/R3E-Network/service_layer/system/engine/core"
)

// BusPublisher dispatches an event to all EventEngine implementations.
type BusPublisher func(context.Context, string, any) error

// BusPusher fan-outs a payload to all DataEngine implementations.
type BusPusher func(context.Context, string, any) error

// ComputeResult captures per-module outcomes for compute fan-out.
type ComputeResult struct {
	Module string `json:"module"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ComputeInvoker invokes every ComputeEngine with the provided payload.
type ComputeInvoker func(context.Context, any) ([]ComputeResult, error)

// Service exposes the HTTP API and fits into the system manager lifecycle.
type Service struct {
	addr              string
	server            *http.Server
	handler           http.Handler
	log               *logger.Logger
	busPub            BusPublisher
	busPush           BusPusher
	invoke            ComputeInvoker
	busMaxBytes       int64
	mu                sync.Mutex
	running           bool
	bound             string
	slowMS            float64
	rpcEng            func() []engine.RPCEngine
	rpcPol            *RPCPolicy
	supabaseGoTrueURL string
	extraRoutes       []RouteRegistrar
}

// ServiceOption customizes the HTTP service behavior.
type ServiceOption func(*Service)

// WithBus wires the engine fan-out helpers (events/data/compute) for use by /system bus endpoints.
func WithBus(publish BusPublisher, push BusPusher, invoke ComputeInvoker) ServiceOption {
	return func(s *Service) {
		s.busPub = publish
		s.busPush = push
		s.invoke = invoke
	}
}

// WithStatusSlowThreshold overrides the slow module threshold (ms) for status responses.
func WithStatusSlowThreshold(ms float64) ServiceOption {
	return func(s *Service) {
		if ms > 0 {
			s.slowMS = ms
		}
	}
}

// WithRPCEnginesOption wires the RPC engine lookup into the HTTP handler.
func WithRPCEnginesOption(fn func() []engine.RPCEngine) ServiceOption {
	return func(s *Service) {
		s.rpcEng = fn
	}
}

// WithRPCPolicyOption wires tenancy/rate limits for /system/rpc.
func WithRPCPolicyOption(policy *RPCPolicy) ServiceOption {
	return func(s *Service) {
		s.rpcPol = policy
	}
}

// WithSupabaseGoTrueURL injects the GoTrue base URL for refresh token proxying.
func WithSupabaseGoTrueURL(url string) ServiceOption {
	return func(s *Service) {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			s.supabaseGoTrueURL = trimmed
		}
	}
}

// WithBusMaxBytesOption caps /system/events|data|compute payload sizes.
func WithBusMaxBytesOption(limit int64) ServiceOption {
	return func(s *Service) {
		if limit > 0 {
			s.busMaxBytes = limit
		}
	}
}

// WithExtraRoutesOption allows registering additional routes on the HTTP service.
// This is useful for integrating external API handlers (e.g., system/api UserService).
func WithExtraRoutesOption(registrars ...RouteRegistrar) ServiceOption {
	return func(s *Service) {
		s.extraRoutes = append(s.extraRoutes, registrars...)
	}
}

func NewService(services app.ServiceProvider, addr string, tokens []string, authMgr authManager, jwtValidator JWTValidator, log *logger.Logger, db *sql.DB, modules ModuleProvider, opts ...ServiceOption) *Service {
	if log == nil {
		log = logger.NewDefault("http")
	}
	var auditSink auditSink
	if path := strings.TrimSpace(os.Getenv("AUDIT_LOG_PATH")); path != "" {
		if sink, err := newFileAuditSink(path); err == nil {
			auditSink = sink
			log.Infof("audit log persisting to %s", path)
		} else {
			log.Warnf("audit log file not configured: %v", err)
		}
	} else if db != nil {
		auditSink = newPostgresAuditSink(db)
	}
	audit := newAuditLog(300, auditSink)
	neo := newNeoReader(db, os.Getenv("NEO_SNAPSHOT_DIR"), os.Getenv("NEO_RPC_STATUS_URL"))
	svc := &Service{
		addr: addr,
		log:  log,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}

	validator := jwtValidator
	if validator == nil {
		validator = authMgr
	}

	if len(tokens) == 0 {
		log.Warn("HTTP service starting without API tokens; prefer Supabase JWT login or configure API_TOKENS")
	}

	// Build handler options
	handlerOpts := []HandlerOption{
		WithBusEndpoints(svc.busPub, svc.busPush, svc.invoke),
		WithListenAddrProvider(svc.Addr),
		WithSlowThreshold(svc.slowMS),
		WithRPCEngines(svc.rpcEng),
		WithRPCPolicy(svc.rpcPol),
	}
	if svc.busMaxBytes > 0 {
		handlerOpts = append(handlerOpts, WithBusMaxBytes(svc.busMaxBytes))
		log.Infof("bus payload limit set to %d bytes", svc.busMaxBytes)
		if svc.busMaxBytes > 10<<20 { // >10 MiB
			log.Warnf("bus payload limit set above 10 MiB (%d bytes); review edge limits and abuse risk", svc.busMaxBytes)
		}
	}
	if svc.supabaseGoTrueURL != "" {
		handlerOpts = append(handlerOpts, WithHandlerSupabaseGoTrueURL(svc.supabaseGoTrueURL))
	}

	// Add admin config store if database is available
	if db != nil {
		adminStore := postgres.New(db)
		handlerOpts = append(handlerOpts, WithAdminConfigStore(adminStore))
	}

	// Add extra routes if configured
	if len(svc.extraRoutes) > 0 {
		handlerOpts = append(handlerOpts, WithExtraRoutes(svc.extraRoutes...))
	}

	handler := NewHandler(services, tokens, authMgr, audit, neo, modules, handlerOpts...)
	// Order matters: auth should see real requests, CORS should short-circuit
	// preflight OPTIONS before auth, metrics wraps the final handler.
	handler = wrapWithAuth(handler, tokens, log, validator)
	handler = wrapWithAudit(handler, audit)
	handler = wrapWithCORS(handler)
	handler = metrics.InstrumentHandler(handler)
	svc.handler = handler
	return svc
}

var _ system.Service = (*Service)(nil)

func (s *Service) Name() string { return "http" }

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	server := &http.Server{
		Addr:         s.addr,
		Handler:      s.handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}
	s.running = true
	s.server = server
	s.bound = ln.Addr().String()
	s.mu.Unlock()

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Errorf("http server error: %v", err)
		}
		s.mu.Lock()
		if s.server == server {
			s.running = false
			s.bound = ""
		}
		s.mu.Unlock()
	}()
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	server := s.server
	s.mu.Unlock()

	if server == nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return nil
	}
	err := server.Shutdown(ctx)

	s.mu.Lock()
	if s.server == server {
		s.running = false
		s.bound = ""
	}
	s.mu.Unlock()

	return err
}

func (s *Service) Domain() string { return "system" }

// Ready reports readiness based on the running flag.
func (s *Service) Ready(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if !running {
		return fmt.Errorf("http server not running")
	}
	return nil
}

// SetReady keeps internal running state in sync with engine readiness.
func (s *Service) SetReady(status string, _ string) {
	s.mu.Lock()
	s.running = strings.EqualFold(strings.TrimSpace(status), "ready")
	s.mu.Unlock()
}

// Addr returns the bound address (after Start) or the configured address when not yet bound.
func (s *Service) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bound != "" {
		return s.bound
	}
	return s.addr
}

// wrapWithCORS allows cross-origin requests from the dashboard (localhost:8081)
// and short-circuits preflight requests.
func wrapWithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
