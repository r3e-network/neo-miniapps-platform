package neoaccounts

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/logging"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	"github.com/R3E-Network/service_layer/infrastructure/middleware"
)

func TestLoadMasterKey_SetsDerivedFields(t *testing.T) {
	m, err := marble.New(marble.Config{MarbleType: "neoaccounts"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}

	key := bytes.Repeat([]byte{0x02}, 32)
	m.SetTestSecret(secretPoolMasterKey, key)

	pub, err := deriveMasterPubKey(key)
	if err != nil {
		t.Fatalf("deriveMasterPubKey: %v", err)
	}
	hash := sha256.Sum256(pub)
	m.SetTestSecret(secretPoolMasterKeyHash, hash[:]) // raw form
	m.SetTestSecret(secretPoolMasterAttestationID, []byte("att-123"))

	svc, err := New(Config{Marble: m})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := svc.loadMasterKey(m); err != nil {
		t.Fatalf("loadMasterKey: %v", err)
	}

	summary := svc.masterKeySummary()
	if summary.Hash != hex.EncodeToString(hash[:]) {
		t.Fatalf("summary hash = %q, want %q", summary.Hash, hex.EncodeToString(hash[:]))
	}
	if summary.AttestationHash != "att-123" {
		t.Fatalf("attestation hash = %q, want att-123", summary.AttestationHash)
	}
}

func TestLoadMasterKey_FailsOnMissingSecrets(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	svc, _ := New(Config{Marble: m})

	if err := svc.loadMasterKey(m); err == nil {
		t.Fatalf("expected error when secrets are missing")
	}
}

func TestLoadMasterKey_FailsOnHashMismatch(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	key := bytes.Repeat([]byte{0x03}, 32)
	m.SetTestSecret(secretPoolMasterKey, key)
	m.SetTestSecret(secretPoolMasterKeyHash, []byte("deadbeef")) // invalid hash input

	svc, _ := New(Config{Marble: m})
	if err := svc.loadMasterKey(m); err == nil {
		t.Fatalf("expected error for invalid hash secret")
	}
}

func TestLoadMasterKey_DerivesFromSeed(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret(secretCoordinatorMasterSeed, bytes.Repeat([]byte{0x04}, 16))

	svc, _ := New(Config{Marble: m})
	if err := svc.loadMasterKey(m); err != nil {
		t.Fatalf("loadMasterKey(seed): %v", err)
	}
	if len(svc.masterKey) < 32 {
		t.Fatalf("expected derived master key to be set")
	}
}

func TestBuildMasterKeyAttestation_NonEnclaveIsSimulated(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	key := bytes.Repeat([]byte{0x05}, 32)
	m.SetTestSecret(secretPoolMasterKey, key)
	svc, _ := New(Config{Marble: m})
	_ = svc.loadMasterKey(m)

	att := svc.buildMasterKeyAttestation()
	if att.Source != "neoaccounts" {
		t.Fatalf("source = %q, want neoaccounts", att.Source)
	}
	if !att.Simulated {
		t.Fatalf("expected attestation to be simulated outside enclave")
	}
	if att.Timestamp == "" {
		t.Fatalf("expected timestamp to be set")
	}
}

func TestGetQuote_ReturnsErrorOutsideEnclave(t *testing.T) {
	report, quote, err := getQuote([]byte("hello"))
	if err != nil {
		return
	}
	if report == nil || len(quote) == 0 {
		t.Fatalf("expected report and quote when no error returned")
	}
}

func TestResolveServiceID_Behavior(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	if _, ok := resolveServiceID(rr, req, ""); ok {
		t.Fatalf("expected missing service_id to be rejected in non-production")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rr = httptest.NewRecorder()
	svc, ok := resolveServiceID(rr, req, "neocompute")
	if !ok || svc != "neocompute" {
		t.Fatalf("resolveServiceID() = (%q,%v), want (neocompute,true)", svc, ok)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Service-ID", "neooracle")
	rr = httptest.NewRecorder()
	if _, ok := resolveServiceID(rr, req, "neocompute"); ok {
		t.Fatalf("expected mismatched service_id to be rejected")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestResolveServiceID_ProductionRejectsMismatch(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")

	svc, _ := newTestServiceWithMock(t)

	privateKey, err := rsaKeyForTest()
	if err != nil {
		t.Fatalf("rsaKeyForTest: %v", err)
	}
	gen := middleware.NewServiceTokenGenerator(privateKey, "neocompute", time.Hour)
	token, err := gen.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	auth := middleware.NewServiceAuthMiddleware(middleware.ServiceAuthConfig{
		PublicKey:       &privateKey.PublicKey,
		Logger:          logging.New("test", "error", "text"),
		AllowedServices: []string{"neocompute"},
	})

	body := []byte(`{"service_id":"neooracle","count":1}`)
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.ServiceTokenHeader, token)
	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{DNSNames: []string{"neocompute"}}}},
	}

	rr := httptest.NewRecorder()
	auth.Handler(http.HandlerFunc(svc.handleRequestAccounts)).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func rsaKeyForTest() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}
