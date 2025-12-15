package signer

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"testing"
	"time"

	signerpb "github.com/R3E-Network/service_layer/api/gen/signer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestService_Sign_SuccessAndAudits(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &auditRepoRecorder{calls: make(chan AuditEvent, 1)}
	audit := NewAuditLogger(repo, "tee_signer_audit", 10, 200*time.Millisecond)
	audit.Start()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = audit.Stop(stopCtx)
	}()

	limNow := time.Unix(0, 0)
	service, err := NewService(ServiceConfig{
		MasterKeySeed: []byte("seed"),
		RateLimiter:   NewClientRateLimiter(10, 10, func() time.Time { return limNow }),
		AuditLogger:   audit,
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := contextWithClientCN(context.Background(), "client-1")
	req := &signerpb.SignRequest{TxHash: "0x" + repeatHex("11", 32)}
	resp, err := service.Sign(ctx, req)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if resp.GetKeyVersion() != KeyVersionV1 {
		t.Fatalf("expected key_version=%q, got %q", KeyVersionV1, resp.GetKeyVersion())
	}
	if len(resp.GetSignature()) != 64 {
		t.Fatalf("expected 64-byte signature, got %d", len(resp.GetSignature()))
	}

	_, hash, _ := DecodeTxHashHex(req.TxHash)
	priv, err := DeriveP256PrivateKey([]byte("seed"), resp.GetKeyVersion())
	if err != nil {
		t.Fatalf("DeriveP256PrivateKey: %v", err)
	}
	if !verifySig(&priv.PublicKey, hash, resp.Signature) {
		t.Fatal("signature did not verify")
	}

	select {
	case evt := <-repo.calls:
		if evt.ClientCertCN != "client-1" {
			t.Fatalf("expected client CN, got %q", evt.ClientCertCN)
		}
		if evt.TxHash != "0x"+repeatHex("11", 32) {
			t.Fatalf("expected canonical tx hash, got %q", evt.TxHash)
		}
		if evt.KeyVersion != KeyVersionV1 {
			t.Fatalf("expected key version, got %q", evt.KeyVersion)
		}
		if !evt.Timestamp.Equal(now) {
			t.Fatalf("expected timestamp %s, got %s", now, evt.Timestamp)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for audit event")
	}
}

func TestService_Sign_Validation(t *testing.T) {
	service, err := NewService(ServiceConfig{
		MasterKeySeed: []byte("seed"),
		RateLimiter:   NewClientRateLimiter(100, 100, time.Now),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := service.Sign(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if _, err := service.Sign(context.Background(), &signerpb.SignRequest{TxHash: "nope"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestService_Sign_RateLimited(t *testing.T) {
	repo := &auditRepoRecorder{calls: make(chan AuditEvent, 2)}
	audit := NewAuditLogger(repo, "tee_signer_audit", 10, 200*time.Millisecond)
	audit.Start()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = audit.Stop(stopCtx)
	}()

	now := time.Unix(0, 0)
	service, err := NewService(ServiceConfig{
		MasterKeySeed: []byte("seed"),
		RateLimiter:   NewClientRateLimiter(1, 1, func() time.Time { return now }),
		AuditLogger:   audit,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := contextWithClientCN(context.Background(), "client-1")
	req := &signerpb.SignRequest{TxHash: "0x" + repeatHex("22", 32)}

	if _, err := service.Sign(ctx, req); err != nil {
		t.Fatalf("Sign #1: %v", err)
	}
	if _, err := service.Sign(ctx, req); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err)
	}
}

func TestService_Sign_InternalError(t *testing.T) {
	service, err := NewService(ServiceConfig{
		MasterKeySeed: []byte("seed"),
		RateLimiter:   NewClientRateLimiter(100, 100, time.Now),
		Rand:          errorReader{err: errors.New("boom")},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := contextWithClientCN(context.Background(), "client-1")
	req := &signerpb.SignRequest{TxHash: "0x" + repeatHex("33", 32)}
	if _, err := service.Sign(ctx, req); status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func TestClientCertCNFromContext_Branches(t *testing.T) {
	if got := clientCertCNFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty CN, got %q", got)
	}

	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: fakeAuthInfo{}})
	if got := clientCertCNFromContext(ctx); got != "" {
		t.Fatalf("expected empty CN for non-TLS auth, got %q", got)
	}

	tlsInfo := credentials.TLSInfo{}
	tlsInfo.State.PeerCertificates = nil
	ctx = peer.NewContext(context.Background(), &peer.Peer{AuthInfo: tlsInfo})
	if got := clientCertCNFromContext(ctx); got != "" {
		t.Fatalf("expected empty CN for missing peer cert, got %q", got)
	}

	if got := commonNameOrEmpty(nil); got != "" {
		t.Fatalf("expected empty CN for nil cert, got %q", got)
	}

	if _, ok := ClientTLSInfo(peer.NewContext(context.Background(), &peer.Peer{AuthInfo: fakeAuthInfo{}})); ok {
		t.Fatal("expected ClientTLSInfo to be false for non-TLS auth")
	}
}

func TestClientTLSInfo(t *testing.T) {
	ctx := contextWithClientCN(context.Background(), "cn")
	if _, ok := ClientTLSInfo(ctx); !ok {
		t.Fatal("expected TLSInfo to be available")
	}
	if _, ok := ClientTLSInfo(context.Background()); ok {
		t.Fatal("expected no TLSInfo")
	}
}

func verifySig(pub *ecdsa.PublicKey, hash []byte, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	return ecdsa.Verify(pub, hash, r, s)
}

func contextWithClientCN(ctx context.Context, cn string) context.Context {
	cert := &x509.Certificate{Subject: pkix.Name{CommonName: cn}}
	tlsInfo := credentials.TLSInfo{
		State: credentials.TLSInfo{}.State,
	}
	tlsInfo.State.PeerCertificates = []*x509.Certificate{cert}
	return peer.NewContext(ctx, &peer.Peer{AuthInfo: tlsInfo})
}

type fakeAuthInfo struct{}

func (fakeAuthInfo) AuthType() string { return "fake" }

type errorReader struct {
	err error
}

func (r errorReader) Read(_ []byte) (int, error) { return 0, r.err }

var _ io.Reader = errorReader{}
