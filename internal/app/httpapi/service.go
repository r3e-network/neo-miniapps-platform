package httpapi

import (
	"context"
	"net/http"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
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

func NewService(application *app.Application, addr string, tokens []string, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("http")
	}
	handler := NewHandler(application)
	handler = wrapWithAuth(handler, tokens, log)
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
