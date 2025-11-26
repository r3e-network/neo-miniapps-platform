package confidential

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domainconf "github.com/R3E-Network/service_layer/internal/app/domain/confidential"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

func TestService_CreateEnclave(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	enclave, err := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE", Endpoint: "https://tee"})
	if err != nil {
		t.Fatalf("create enclave: %v", err)
	}
	if enclave.Status != domainconf.EnclaveStatusInactive {
		t.Fatalf("expected default inactive")
	}
	list, err := svc.ListEnclaves(context.Background(), acct.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list enclaves failed")
	}
}

func TestService_CreateSealedKey(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	enclave, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE", Endpoint: "https://tee"})
	key, err := svc.CreateSealedKey(context.Background(), domainconf.SealedKey{AccountID: acct.ID, EnclaveID: enclave.ID, Name: "default", Blob: []byte("blob")})
	if err != nil {
		t.Fatalf("create sealed key: %v", err)
	}
	keys, err := svc.ListSealedKeys(context.Background(), acct.ID, enclave.ID, 10)
	if err != nil {
		t.Fatalf("list sealed keys: %v", err)
	}
	if len(keys) != 1 || keys[0].ID != key.ID {
		t.Fatalf("expected one sealed key")
	}
}

func TestService_CreateAttestation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	enclave, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE", Endpoint: "https://tee"})
	att, err := svc.CreateAttestation(context.Background(), domainconf.Attestation{AccountID: acct.ID, EnclaveID: enclave.ID, Report: "quote", Status: "valid"})
	if err != nil {
		t.Fatalf("create attestation: %v", err)
	}
	list, err := svc.ListAttestations(context.Background(), acct.ID, enclave.ID, 10)
	if err != nil {
		t.Fatalf("list attestations: %v", err)
	}
	if len(list) != 1 || list[0].ID != att.ID {
		t.Fatalf("expected single attestation")
	}
}

func TestService_ListAccountAttestations(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	enclave1, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE1", Endpoint: "https://tee1"})
	enclave2, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE2", Endpoint: "https://tee2"})
	if _, err := svc.CreateAttestation(context.Background(), domainconf.Attestation{AccountID: acct.ID, EnclaveID: enclave1.ID, Report: "quote-1", Status: "valid"}); err != nil {
		t.Fatalf("create attestation 1: %v", err)
	}
	if _, err := svc.CreateAttestation(context.Background(), domainconf.Attestation{AccountID: acct.ID, EnclaveID: enclave2.ID, Report: "quote-2", Status: "valid"}); err != nil {
		t.Fatalf("create attestation 2: %v", err)
	}
	atts, err := svc.ListAccountAttestations(context.Background(), acct.ID, 10)
	if err != nil {
		t.Fatalf("list account attestations: %v", err)
	}
	if len(atts) != 2 {
		t.Fatalf("expected 2 attestations, got %d", len(atts))
	}
	seen := map[string]bool{}
	for _, att := range atts {
		seen[att.EnclaveID] = true
	}
	if len(seen) != 2 {
		t.Fatalf("expected attestations from both enclaves, got %+v", seen)
	}
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "confidential" {
		t.Fatalf("expected name confidential")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "confidential" {
		t.Fatalf("expected name confidential")
	}
}

func TestService_GetEnclave(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	enclave, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE", Endpoint: "https://tee"})

	got, err := svc.GetEnclave(context.Background(), acct.ID, enclave.ID)
	if err != nil {
		t.Fatalf("get enclave: %v", err)
	}
	if got.ID != enclave.ID {
		t.Fatalf("enclave mismatch")
	}
}

func TestService_GetEnclaveOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	enclave, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct1.ID, Name: "TEE", Endpoint: "https://tee"})

	if _, err := svc.GetEnclave(context.Background(), acct2.ID, enclave.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_EnclaveValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)

	// Missing name
	if _, err := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Endpoint: "https://tee"}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing endpoint
	if _, err := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE"}); err == nil {
		t.Fatalf("expected endpoint required error")
	}
	// Invalid status
	if _, err := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE", Endpoint: "https://tee", Status: "invalid"}); err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestService_SealedKeyValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	enclave, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE", Endpoint: "https://tee"})

	// Missing name
	if _, err := svc.CreateSealedKey(context.Background(), domainconf.SealedKey{AccountID: acct.ID, EnclaveID: enclave.ID, Blob: []byte("blob")}); err == nil {
		t.Fatalf("expected name required error")
	}
}

func TestService_AttestationValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	enclave, _ := svc.CreateEnclave(context.Background(), domainconf.Enclave{AccountID: acct.ID, Name: "TEE", Endpoint: "https://tee"})

	// Missing report
	if _, err := svc.CreateAttestation(context.Background(), domainconf.Attestation{AccountID: acct.ID, EnclaveID: enclave.ID}); err == nil {
		t.Fatalf("expected report required error")
	}
}
