package signer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"time"

	signerpb "github.com/R3E-Network/service_layer/api/gen/signer"
	"github.com/R3E-Network/service_layer/internal/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const (
	DefaultRPS   = 100
	DefaultBurst = 100
)

type ServiceConfig struct {
	MasterKeySeed []byte
	KeyVersion    string
	KeyVersions   KeyVersionRepository
	OverlapPeriod time.Duration

	RateLimiter *ClientRateLimiter
	AuditLogger *AuditLogger
	Rand        io.Reader
	Now         func() time.Time
}

type Service struct {
	signerpb.UnimplementedTEESignerServer

	keys *KeyManager

	rand io.Reader
	now  func() time.Time

	limiter *ClientRateLimiter
	audit   *AuditLogger
}

func NewService(cfg ServiceConfig) (*Service, error) {
	keyVersion := cfg.KeyVersion
	if keyVersion == "" {
		keyVersion = KeyVersionV1
	}

	keys, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     cfg.MasterKeySeed,
		Repository:        cfg.KeyVersions,
		Now:               cfg.Now,
		OverlapPeriod:     cfg.OverlapPeriod,
		InitialKeyVersion: keyVersion,
		RequireRepository: runtime.StrictIdentityMode(),
	})
	if err != nil {
		return nil, err
	}

	limiter := cfg.RateLimiter
	if limiter == nil {
		limiter = NewClientRateLimiter(DefaultRPS, DefaultBurst, time.Now)
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	randReader := cfg.Rand
	if randReader == nil {
		randReader = rand.Reader
	}

	return &Service{
		keys:    keys,
		rand:    randReader,
		now:     now,
		limiter: limiter,
		audit:   cfg.AuditLogger,
	}, nil
}

func (s *Service) Sign(ctx context.Context, req *signerpb.SignRequest) (*signerpb.SignResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	clientCN := clientCertCNFromContext(ctx)
	if !s.limiter.Allow(clientCN) {
		s.logAudit(clientCN, req.GetTxHash(), s.keys.ActiveVersion())
		return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}

	txHashCanonical, txHash, err := DecodeTxHashHex(req.GetTxHash())
	if err != nil {
		s.logAudit(clientCN, req.GetTxHash(), s.keys.ActiveVersion())
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	signingKey, usedVersion, err := s.keys.SigningKeyAt(ctx, req.GetKeyVersion(), s.now().UTC())
	if err != nil {
		// Keep the same audit behavior (best-effort) for rejected key versions.
		s.logAudit(clientCN, txHashCanonical, req.GetKeyVersion())
		if errors.Is(err, ErrKeyVersionExpired) || errors.Is(err, ErrKeyVersionNotFound) {
			return nil, status.Error(codes.InvalidArgument, "invalid key_version")
		}
		return nil, status.Error(codes.Internal, "key selection failed")
	}

	sig, err := SignHashP256(s.rand, signingKey, txHash)
	if err != nil {
		s.logAudit(clientCN, txHashCanonical, usedVersion)
		return nil, status.Error(codes.Internal, "signing failed")
	}

	s.logAudit(clientCN, txHashCanonical, usedVersion)
	return &signerpb.SignResponse{
		Signature:  sig,
		KeyVersion: usedVersion,
	}, nil
}

func (s *Service) RotateKey(ctx context.Context) (*RotationResult, error) {
	if s == nil || s.keys == nil {
		return nil, fmt.Errorf("service not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	res, err := s.keys.Rotate(ctx)
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		tx := "rotate-key"
		if res.OldVersion != "" {
			tx = "rotate-key old=" + res.OldVersion + " new=" + res.NewVersion
		}
		s.audit.Log(AuditEvent{
			Timestamp:    s.now().UTC(),
			ClientCertCN: "internal",
			TxHash:       tx,
			KeyVersion:   res.NewVersion,
		})
	}

	return res, nil
}

func (s *Service) logAudit(clientCN, txHash, keyVersion string) {
	if s.audit == nil {
		return
	}
	s.audit.Log(AuditEvent{
		Timestamp:    s.now().UTC(),
		ClientCertCN: clientCN,
		TxHash:       txHash,
		KeyVersion:   keyVersion,
	})
}

// SignWithDomain performs domain-separated signing.
// This method enforces domain separation to prevent cross-service signature reuse.
func (s *Service) SignWithDomain(ctx context.Context, req *DomainSeparatedRequest) (*DomainSeparatedResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Check rate limit
	clientCN := clientCertCNFromContext(ctx)
	if !s.limiter.Allow(clientCN) {
		s.logAudit(clientCN, req.RequestID, s.keys.ActiveVersion())
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Compute domain hash
	domainHash, err := req.ComputeDomainHash()
	if err != nil {
		s.logAudit(clientCN, req.RequestID, s.keys.ActiveVersion())
		return nil, fmt.Errorf("compute domain hash: %w", err)
	}

	// Compute final signing message
	signingMessage := ComputeSigningMessage(domainHash)

	// Get signing key
	signingKey, usedVersion, err := s.keys.SigningKeyAt(ctx, req.KeyVersion, s.now().UTC())
	if err != nil {
		s.logAudit(clientCN, req.RequestID, req.KeyVersion)
		if errors.Is(err, ErrKeyVersionExpired) || errors.Is(err, ErrKeyVersionNotFound) {
			return nil, fmt.Errorf("invalid key_version")
		}
		return nil, fmt.Errorf("key selection failed")
	}

	// Sign the message
	sig, err := SignHashP256(s.rand, signingKey, signingMessage)
	if err != nil {
		s.logAudit(clientCN, req.RequestID, usedVersion)
		return nil, fmt.Errorf("signing failed")
	}

	// Log audit
	s.logAudit(clientCN, req.RequestID, usedVersion)

	return &DomainSeparatedResponse{
		Signature:  sig,
		KeyVersion: usedVersion,
		SignedAt:   s.now().UTC(),
		DomainHash: domainHash,
	}, nil
}

func clientCertCNFromContext(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil {
		return ""
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return ""
	}

	state := tlsInfo.State
	if len(state.PeerCertificates) == 0 {
		return ""
	}
	return commonNameOrEmpty(state.PeerCertificates[0])
}

func commonNameOrEmpty(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	return cert.Subject.CommonName
}

func ClientTLSInfo(ctx context.Context) (*tls.ConnectionState, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil {
		return nil, false
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, false
	}
	return &tlsInfo.State, true
}
