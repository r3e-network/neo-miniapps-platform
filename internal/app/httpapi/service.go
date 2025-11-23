package httpapi

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/jam"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
	"github.com/R3E-Network/service_layer/internal/app/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service exposes the HTTP API and fits into the system manager lifecycle.
type Service struct {
	addr    string
	server  *http.Server
	handler http.Handler
	log     *logger.Logger
}

func NewService(application *app.Application, addr string, tokens []string, jamCfg jam.Config, authMgr authManager, log *logger.Logger) *Service {
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
	}
	audit := newAuditLog(300, auditSink)
	handler := NewHandler(application, jamCfg, tokens, authMgr, audit)
	// Order matters: auth should see real requests, CORS should short-circuit
	// preflight OPTIONS before auth, metrics wraps the final handler.
	handler = wrapWithAuth(handler, tokens, log, authMgr)
	handler = wrapWithAudit(handler, audit)
	handler = wrapWithCORS(handler)
	handler = metrics.InstrumentHandler(handler)
	return &Service{
		addr:    addr,
		handler: handler,
		log:     log,
	}
}

var _ system.Service = (*Service)(nil)

func (s *Service) Name() string { return "http" }

func (s *Service) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      s.handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Errorf("http server error: %v", err)
		}
	}()
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
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
