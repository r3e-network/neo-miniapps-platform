package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	signerpb "github.com/R3E-Network/service_layer/api/gen/signer"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/runtime"
	"github.com/R3E-Network/service_layer/services/tee-signer/signer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var listenFunc = net.Listen

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("tee-signer: %v", err)
	}
}

func run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	m, err := marble.New(marble.Config{MarbleType: "tee-signer"})
	if err != nil {
		return fmt.Errorf("create marble: %w", err)
	}
	if err := m.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize marble: %w", err)
	}

	if (runtime.StrictIdentityMode() || m.IsEnclave()) && m.TLSConfig() == nil {
		return fmt.Errorf("CRITICAL: MarbleRun TLS credentials are required in production/SGX mode (missing MARBLE_CERT/MARBLE_KEY/MARBLE_ROOT_CA)")
	}

	masterSeed, ok := m.Secret("MASTER_KEY_SEED")
	if !ok || len(masterSeed) == 0 {
		return fmt.Errorf("CRITICAL: MASTER_KEY_SEED is required")
	}

	db, err := initSupabase(m)
	if err != nil {
		return err
	}

	auditTable := strings.TrimSpace(os.Getenv("TEE_SIGNER_AUDIT_TABLE"))
	if auditTable == "" {
		auditTable = "tee_signer_audit"
	}

	auditor := signer.NewAuditLogger(db, auditTable, signer.DefaultAuditBuffer, signer.DefaultAuditTimeout)
	auditor.Start()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = auditor.Stop(stopCtx)
	}()

	service, err := signer.NewService(signer.ServiceConfig{
		MasterKeySeed: masterSeed,
		KeyVersions:   db,
		AuditLogger:   auditor,
	})
	if err != nil {
		return err
	}

	grpcServer := newGRPCServer(m.TLSConfig())
	signerpb.RegisterTEESignerServer(grpcServer, service)

	addr := ":" + strconv.Itoa(listenPort())
	lis, err := listenFunc("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	grpcServeErr := make(chan error, 1)
	go func() {
		log.Printf("tee-signer listening on %s", addr)
		grpcServeErr <- grpcServer.Serve(lis)
	}()

	httpServer, httpServeErr := startInternalHTTPServer(service)

	select {
	case <-ctx.Done():
	case err := <-grpcServeErr:
		// Server exited without a shutdown request.
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case err := <-httpServeErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("internal http serve: %w", err)
		}
		return nil
	}

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	if httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = httpServer.Shutdown(shutdownCtx)
		cancel()
	}

	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		grpcServer.Stop()
	}

	<-grpcServeErr
	return nil
}

func listenPort() int {
	const defaultPort = 9090
	if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
		if port, err := strconv.Atoi(raw); err == nil && port >= 0 && port <= 65535 {
			return port
		}
	}
	return defaultPort
}

func internalHTTPPort() (int, bool) {
	raw := strings.TrimSpace(os.Getenv("INTERNAL_HTTP_PORT"))
	if raw == "" {
		return 0, false
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

func newGRPCServer(tlsCfg *tls.Config) *grpc.Server {
	if tlsCfg == nil {
		return grpc.NewServer()
	}
	return grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCfg)))
}

func startInternalHTTPServer(service *signer.Service) (*http.Server, <-chan error) {
	port, ok := internalHTTPPort()
	if !ok {
		return nil, nil
	}

	serveErr := make(chan error, 1)

	addr := ":" + strconv.Itoa(port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           newInternalHTTPHandler(service),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("tee-signer internal http listening on %s", addr)
		serveErr <- srv.ListenAndServe()
	}()

	return srv, serveErr
}

func newInternalHTTPHandler(service *signer.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/rotate-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			httputil.WriteErrorResponse(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
			return
		}

		res, err := service.RotateKey(r.Context())
		if err != nil {
			httputil.WriteErrorResponse(w, r, http.StatusInternalServerError, "ROTATE_FAILED", "rotation failed", map[string]any{
				"error": err.Error(),
			})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, res)
	})
	return mux
}

func initSupabase(m *marble.Marble) (*database.Repository, error) {
	supabaseURL := strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	if secret, ok := m.Secret("SUPABASE_URL"); ok && len(secret) > 0 {
		supabaseURL = strings.TrimSpace(string(secret))
	}

	supabaseServiceKey := strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_KEY"))
	if secret, ok := m.Secret("SUPABASE_SERVICE_KEY"); ok && len(secret) > 0 {
		supabaseServiceKey = strings.TrimSpace(string(secret))
	}

	client, err := database.NewClient(database.Config{
		URL:        supabaseURL,
		ServiceKey: supabaseServiceKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create supabase client: %w", err)
	}
	return database.NewRepository(client), nil
}
