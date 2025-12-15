//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	signerpb "github.com/R3E-Network/service_layer/api/gen/signer"
	"github.com/R3E-Network/service_layer/internal/headergate"
	"github.com/R3E-Network/service_layer/services/tee-signer/signer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"gopkg.in/yaml.v3"
)

// =============================================================================
// Test Helpers
// =============================================================================

func repoRoot(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file path via runtime.Caller")
	}

	// test/integration/... -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func decodeYAMLDocuments(t *testing.T, path string) []any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(false)

	var docs []any
	for {
		var doc any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if doc == nil {
			continue
		}
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		t.Fatalf("no YAML documents found in %s", path)
	}

	return docs
}

func asStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for k, v := range typed {
			key, ok := k.(string)
			if !ok {
				return nil, false
			}
			converted[key] = v
		}
		return converted, true
	default:
		return nil, false
	}
}

func nestedMap(root map[string]any, path ...string) (map[string]any, bool) {
	current := root
	for _, key := range path {
		nextRaw, ok := current[key]
		if !ok {
			return nil, false
		}
		next, ok := asStringMap(nextRaw)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func nestedString(root map[string]any, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}

	parent, ok := nestedMap(root, path[:len(path)-1]...)
	if !ok {
		return "", false
	}
	value, ok := parent[path[len(path)-1]]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

func nestedStringSlice(root map[string]any, path ...string) ([]string, bool) {
	if len(path) == 0 {
		return nil, false
	}

	parent, ok := nestedMap(root, path[:len(path)-1]...)
	if !ok {
		return nil, false
	}
	value, ok := parent[path[len(path)-1]]
	if !ok {
		return nil, false
	}

	rawSlice, ok := value.([]any)
	if !ok {
		rawIface, ok := value.([]interface{})
		if !ok {
			return nil, false
		}
		rawSlice = make([]any, 0, len(rawIface))
		for _, v := range rawIface {
			rawSlice = append(rawSlice, v)
		}
	}

	out := make([]string, 0, len(rawSlice))
	for _, item := range rawSlice {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

type auditRecorder struct {
	ch chan signer.AuditEvent
}

func (r *auditRecorder) Request(ctx context.Context, method, table string, body interface{}, query string) ([]byte, error) {
	evt, ok := body.(signer.AuditEvent)
	if !ok {
		return nil, nil
	}
	select {
	case r.ch <- evt:
	default:
	}
	return []byte("[]"), nil
}

type nopResponseWriter struct {
	header http.Header
}

func (w *nopResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *nopResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *nopResponseWriter) WriteHeader(statusCode int)  {}

// =============================================================================
// TLS Fixture (Coordinator-issued mTLS simulation)
// =============================================================================

type mtlsFixture struct {
	pool       *x509.CertPool
	serverTLS  *tls.Config
	clientTLS  *tls.Config
	clientName string
}

func newMTLSFixture(t *testing.T, clientCN string) mtlsFixture {
	t.Helper()

	caCert, caKey := newTestCA(t)
	serverCert := newLeafCert(t, caCert, caKey, "tee-signer", []string{"localhost"}, nil, false)
	clientCert := newLeafCert(t, caCert, caKey, clientCN, nil, nil, true)

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	return mtlsFixture{
		pool: pool,
		serverTLS: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{serverCert},
			ClientCAs:    pool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
		clientTLS: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			RootCAs:      pool,
			Certificates: []tls.Certificate{clientCert},
			ServerName:   "localhost",
		},
		clientName: clientCN,
	}
}

func newTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}

	now := time.Now().UTC()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "test-coordinator-ca"},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create ca cert: %v", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse ca cert: %v", err)
	}

	return cert, key
}

func newLeafCert(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, commonName string, dnsNames []string, ipAddrs []net.IP, isClient bool) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}

	now := time.Now().UTC()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}

	extUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	if isClient {
		extUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  extUsages,
		DNSNames:     dnsNames,
		IPAddresses:  ipAddrs,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("x509 key pair: %v", err)
	}
	return cert
}

func startSignerServer(t *testing.T, svc *signer.Service, tlsCfg *tls.Config) *bufconn.Listener {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })

	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCfg)))
	signerpb.RegisterTEESignerServer(grpcServer, svc)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- grpcServer.Serve(lis)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
		<-serveErr
	})

	return lis
}

func dialSigner(t *testing.T, lis *bufconn.Listener, tlsCfg *tls.Config) *grpc.ClientConn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func verifyP256Signature(t *testing.T, publicKey *ecdsa.PublicKey, hash []byte, signature []byte) {
	t.Helper()

	if len(signature) != 64 {
		t.Fatalf("signature length = %d, want 64", len(signature))
	}
	if publicKey == nil {
		t.Fatal("public key is nil")
	}

	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(publicKey, hash, r, s) {
		t.Fatal("ecdsa.Verify failed")
	}
}

// =============================================================================
// E2E Scenarios
// =============================================================================

func TestSprint2_CoordinatorToSignerFlow_mTLSAndSigningWorks(t *testing.T) {
	mTLS := newMTLSFixture(t, "coordinator")

	auditCh := make(chan signer.AuditEvent, 8)
	auditor := signer.NewAuditLogger(&auditRecorder{ch: auditCh}, "tee_signer_audit", 16, 2*time.Second)
	auditor.Start()
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = auditor.Stop(stopCtx)
	})

	now := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return now }

	svc, err := signer.NewService(signer.ServiceConfig{
		MasterKeySeed: []byte("seed"),
		RateLimiter:   signer.NewClientRateLimiter(100000, 100000, nowFn),
		AuditLogger:   auditor,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	lis := startSignerServer(t, svc, mTLS.serverTLS)
	conn := dialSigner(t, lis, mTLS.clientTLS)
	client := signerpb.NewTEESignerClient(conn)

	txHashHex := "0x" + strings.Repeat("11", 32)
	resp, err := client.Sign(context.Background(), &signerpb.SignRequest{TxHash: txHashHex})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if resp.GetKeyVersion() != "v1" {
		t.Fatalf("key_version = %q, want v1", resp.GetKeyVersion())
	}

	_, txHash, err := signer.DecodeTxHashHex(txHashHex)
	if err != nil {
		t.Fatalf("DecodeTxHashHex: %v", err)
	}

	priv, err := signer.DeriveP256PrivateKey([]byte("seed"), resp.GetKeyVersion())
	if err != nil {
		t.Fatalf("DeriveP256PrivateKey: %v", err)
	}
	verifyP256Signature(t, &priv.PublicKey, txHash, resp.GetSignature())

	select {
	case evt := <-auditCh:
		if evt.ClientCertCN != mTLS.clientName {
			t.Fatalf("audit client_cert_cn = %q, want %q", evt.ClientCertCN, mTLS.clientName)
		}
		if evt.KeyVersion != resp.GetKeyVersion() {
			t.Fatalf("audit key_version = %q, want %q", evt.KeyVersion, resp.GetKeyVersion())
		}
		if evt.TxHash == "" {
			t.Fatal("audit tx_hash is empty")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audit event")
	}

	t.Run("missing_client_certificate_rejected", func(t *testing.T) {
		badTLS := &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    mTLS.pool,
			ServerName: "localhost",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dialer := func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}

		conn, err := grpc.DialContext(ctx, "bufnet",
			grpc.WithContextDialer(dialer),
			grpc.WithTransportCredentials(credentials.NewTLS(badTLS)),
			grpc.WithBlock(),
		)
		if err == nil {
			defer conn.Close()
			_, err = signerpb.NewTEESignerClient(conn).Sign(ctx, &signerpb.SignRequest{TxHash: txHashHex})
		}
		if err == nil {
			t.Fatal("expected mTLS handshake/rpc error, got nil")
		}
	})
}

func TestSprint2_HeaderGateToAPIFlow_AllowsAuthorizedRequests(t *testing.T) {
	sharedSecret := "integration-secret"

	called := false
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := headergate.Middleware(sharedSecret)(base)

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/me", nil)
	req.Header.Set("X-Vercel-Id", "vercel-app")
	req.Header.Set("X-Shared-Secret", sharedSecret)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestSprint2_KeyRotationFlow_CronJobAndOverlap(t *testing.T) {
	t.Run("cronjob_manifest_targets_internal_rotate_endpoint", func(t *testing.T) {
		root := repoRoot(t)
		cronPath := filepath.Join(root, "k8s", "platform", "cronjob-key-rotation.yaml")

		docs := decodeYAMLDocuments(t, cronPath)
		found := false

		for _, doc := range docs {
			m, ok := asStringMap(doc)
			if !ok {
				continue
			}
			if kind, _ := m["kind"].(string); kind != "CronJob" {
				continue
			}

			name, ok := nestedString(m, "metadata", "name")
			if !ok || name != "tee-signer-key-rotation" {
				continue
			}

			found = true

			schedule, ok := nestedString(m, "spec", "schedule")
			if !ok || schedule != "0 0 1 * *" {
				t.Fatalf("spec.schedule = %q, want %q", schedule, "0 0 1 * *")
			}

			concurrency, ok := nestedString(m, "spec", "concurrencyPolicy")
			if !ok || concurrency != "Forbid" {
				t.Fatalf("spec.concurrencyPolicy = %q, want %q", concurrency, "Forbid")
			}

			args, ok := nestedStringSlice(m, "spec", "jobTemplate", "spec", "template", "spec", "containers", "0", "args")
			if ok {
				// yaml decoder doesn't index slices by string key; fall back to raw check below.
				_ = args
			}

			raw, err := os.ReadFile(cronPath)
			if err != nil {
				t.Fatalf("read %s: %v", cronPath, err)
			}
			wantURL := "http://tee-signer.service-layer.svc.cluster.local:8080/internal/rotate-key"
			if !bytes.Contains(raw, []byte(wantURL)) {
				t.Fatalf("expected CronJob to call %q", wantURL)
			}
		}

		if !found {
			t.Fatalf("tee-signer-key-rotation CronJob not found in %s", cronPath)
		}
	})

	t.Run("rotation_overlapping_key_versions_work", func(t *testing.T) {
		mTLS := newMTLSFixture(t, "rotation-client")

		current := time.Unix(1, 0).UTC()
		nowFn := func() time.Time { return current }

		svc, err := signer.NewService(signer.ServiceConfig{
			MasterKeySeed: []byte("seed"),
			RateLimiter:   signer.NewClientRateLimiter(100000, 100000, nowFn),
			OverlapPeriod: 7 * 24 * time.Hour,
			Now:           nowFn,
			KeyVersion:    signer.KeyVersionV1,
		})
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}

		lis := startSignerServer(t, svc, mTLS.serverTLS)
		conn := dialSigner(t, lis, mTLS.clientTLS)
		client := signerpb.NewTEESignerClient(conn)

		txHash := "0x" + strings.Repeat("22", 32)
		resp, err := client.Sign(context.Background(), &signerpb.SignRequest{TxHash: txHash})
		if err != nil {
			t.Fatalf("Sign(v1): %v", err)
		}
		if resp.GetKeyVersion() != "v1" {
			t.Fatalf("key_version = %q, want v1", resp.GetKeyVersion())
		}

		current = time.Unix(2, 0).UTC()
		start := time.Now()
		rotation, err := svc.RotateKey(context.Background())
		if err != nil {
			t.Fatalf("RotateKey: %v", err)
		}
		if took := time.Since(start); took > 30*time.Second {
			t.Fatalf("RotateKey took %s, want < 30s", took)
		}
		if rotation.NewVersion != "v2" || !rotation.Rotated {
			t.Fatalf("rotation new_version=%q rotated=%v, want v2 true", rotation.NewVersion, rotation.Rotated)
		}

		resp, err = client.Sign(context.Background(), &signerpb.SignRequest{TxHash: txHash})
		if err != nil {
			t.Fatalf("Sign(v2): %v", err)
		}
		if resp.GetKeyVersion() != "v2" {
			t.Fatalf("key_version = %q, want v2", resp.GetKeyVersion())
		}

		withinOverlap := time.Unix(2, 0).UTC().Add(6 * 24 * time.Hour)
		current = withinOverlap
		resp, err = client.Sign(context.Background(), &signerpb.SignRequest{
			TxHash:     "0x" + strings.Repeat("33", 32),
			KeyVersion: "v1",
		})
		if err != nil {
			t.Fatalf("Sign(v1 within overlap): %v", err)
		}
		if resp.GetKeyVersion() != "v1" {
			t.Fatalf("key_version = %q, want v1", resp.GetKeyVersion())
		}

		afterExpiry := time.Unix(2, 0).UTC().Add(8 * 24 * time.Hour)
		current = afterExpiry
		_, err = client.Sign(context.Background(), &signerpb.SignRequest{
			TxHash:     "0x" + strings.Repeat("44", 32),
			KeyVersion: "v1",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("Sign(v1 after overlap) error = %v, want InvalidArgument", err)
		}
	})
}

func TestSprint2_FullRequestPath_VercelToSignerResponse(t *testing.T) {
	mTLS := newMTLSFixture(t, "service-client")

	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	svc, err := signer.NewService(signer.ServiceConfig{
		MasterKeySeed: []byte("seed"),
		RateLimiter:   signer.NewClientRateLimiter(100000, 100000, nowFn),
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	lis := startSignerServer(t, svc, mTLS.serverTLS)
	signerConn := dialSigner(t, lis, mTLS.clientTLS)
	signerClient := signerpb.NewTEESignerClient(signerConn)

	serviceHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/sign" {
			http.NotFound(w, r)
			return
		}

		var input struct {
			TxHash string `json:"tx_hash"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		resp, err := signerClient.Sign(ctx, &signerpb.SignRequest{TxHash: input.TxHash})
		if err != nil {
			code := http.StatusBadGateway
			if status.Code(err) == codes.InvalidArgument {
				code = http.StatusBadRequest
			}
			http.Error(w, err.Error(), code)
			return
		}

		out := struct {
			Signature  string `json:"signature"`
			KeyVersion string `json:"key_version"`
		}{
			Signature:  "0x" + hex.EncodeToString(resp.GetSignature()),
			KeyVersion: resp.GetKeyVersion(),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	sharedSecret := "vercel-shared-secret"
	gatewayMux := http.NewServeMux()
	gatewayMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	gatewayMux.Handle("/api/sign", serviceHandler)
	gatewayHandler := headergate.Middleware(sharedSecret)(gatewayMux)

	t.Run("health_bypasses_header_gate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://gateway/health", nil)
		rr := httptest.NewRecorder()
		gatewayHandler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	txHash := "0x" + strings.Repeat("55", 32)
	body := []byte(`{"tx_hash":"` + txHash + `"}`)

	t.Run("missing_headers_rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://gateway/api/sign", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		gatewayHandler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("authorized_request_returns_signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://gateway/api/sign", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Vercel-Id", "vercel-app")
		req.Header.Set("X-Shared-Secret", sharedSecret)

		rr := httptest.NewRecorder()
		gatewayHandler.ServeHTTP(rr, req)

		var out struct {
			Signature  string `json:"signature"`
			KeyVersion string `json:"key_version"`
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		sigHex := strings.TrimPrefix(out.Signature, "0x")
		sig, err := hex.DecodeString(sigHex)
		if err != nil {
			t.Fatalf("decode signature: %v", err)
		}

		_, hash, err := signer.DecodeTxHashHex(txHash)
		if err != nil {
			t.Fatalf("DecodeTxHashHex: %v", err)
		}

		priv, err := signer.DeriveP256PrivateKey([]byte("seed"), out.KeyVersion)
		if err != nil {
			t.Fatalf("DeriveP256PrivateKey: %v", err)
		}
		verifyP256Signature(t, &priv.PublicKey, hash, sig)
	})
}

// =============================================================================
// Performance Baselines (opt-in)
// =============================================================================

func BenchmarkSprint2_HeaderGate_Baseline(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/me", nil)
	rw := &nopResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rw, req)
	}
}

func BenchmarkSprint2_HeaderGate_WithMiddleware(b *testing.B) {
	sharedSecret := "bench-secret"
	handler := headergate.Middleware(sharedSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/me", nil)
	req.Header.Set("X-Vercel-Id", "vercel-app")
	req.Header.Set("X-Shared-Secret", sharedSecret)

	rw := &nopResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rw, req)
	}
}

func TestSprint2_PerformanceBaselines(t *testing.T) {
	if strings.TrimSpace(os.Getenv("SPRINT2_PERF_ASSERT")) != "1" {
		t.Skip("set SPRINT2_PERF_ASSERT=1 to enable performance assertions")
	}

	t.Run("header_gate_overhead_under_1ms", func(t *testing.T) {
		base := testing.Benchmark(BenchmarkSprint2_HeaderGate_Baseline)
		withGate := testing.Benchmark(BenchmarkSprint2_HeaderGate_WithMiddleware)

		overhead := withGate.NsPerOp() - base.NsPerOp()
		if overhead < 0 {
			overhead = 0
		}

		if time.Duration(overhead) > time.Millisecond {
			t.Fatalf("header gate overhead = %s/op (baseline %s/op, with gate %s/op), want < 1ms/op",
				time.Duration(overhead), time.Duration(base.NsPerOp()), time.Duration(withGate.NsPerOp()))
		}
	})

	t.Run("tee_signer_p99_under_5ms_and_throughput_over_500rps", func(t *testing.T) {
		mTLS := newMTLSFixture(t, "perf-client")

		nowFn := time.Now
		svc, err := signer.NewService(signer.ServiceConfig{
			MasterKeySeed: []byte("seed"),
			RateLimiter:   signer.NewClientRateLimiter(1_000_000, 1_000_000, nowFn),
			Now:           nowFn,
		})
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}

		lis := startSignerServer(t, svc, mTLS.serverTLS)
		conn := dialSigner(t, lis, mTLS.clientTLS)
		client := signerpb.NewTEESignerClient(conn)

		const (
			total       = 2000
			concurrency = 32
		)

		txHashes := make([]string, total)
		for i := 0; i < total; i++ {
			fragment := fmt.Sprintf("%02x", byte(i))
			txHashes[i] = "0x" + strings.Repeat(fragment, 32)
		}

		durations := make([]time.Duration, total)
		work := make(chan int, total)
		for i := 0; i < total; i++ {
			work <- i
		}
		close(work)

		start := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		for w := 0; w < concurrency; w++ {
			go func(worker int) {
				defer wg.Done()
				for idx := range work {
					txHash := txHashes[idx]
					callStart := time.Now()
					_, err := client.Sign(context.Background(), &signerpb.SignRequest{TxHash: txHash})
					durations[idx] = time.Since(callStart)
					if err != nil {
						// Leave the error in the logs; a single error invalidates the run.
						durations[idx] = 0
					}
				}
			}(w)
		}
		wg.Wait()
		elapsed := time.Since(start)

		for i, d := range durations {
			if d == 0 {
				t.Fatalf("sign failed at index %d", i)
			}
		}

		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p99 := percentileDuration(durations, 0.99)
		throughput := float64(total) / elapsed.Seconds()

		if p99 > 5*time.Millisecond {
			t.Fatalf("tee-signer p99 = %s, want < 5ms", p99)
		}
		if throughput < 500 {
			t.Fatalf("tee-signer throughput = %.2f req/s, want > 500 req/s", throughput)
		}
	})
}

func percentileDuration(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
