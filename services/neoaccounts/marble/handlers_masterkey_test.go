package neoaccountsmarble

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

func TestMasterKeyEndpoint_ReturnsAttestation(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	server := httptest.NewServer(svc.Router())
	defer server.Close()

	resp, err := http.Get(server.URL + "/master-key")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if cache := resp.Header.Get("Cache-Control"); cache == "" {
		t.Fatalf("expected Cache-Control header to be set")
	}

	data, err := httputil.ReadAllStrict(resp.Body, 1<<20)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var body MasterKeyAttestation
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode json: %v", err)
	}

	summary := svc.masterKeySummary()
	if body.Hash != summary.Hash {
		t.Fatalf("hash = %q, want %q", body.Hash, summary.Hash)
	}
	if body.PubKey != summary.PubKeyHex {
		t.Fatalf("pubkey = %q, want %q", body.PubKey, summary.PubKeyHex)
	}
	if body.Source != "neoaccounts" {
		t.Fatalf("source = %q, want neoaccounts", body.Source)
	}
	if !body.Simulated {
		t.Fatalf("expected simulated=true outside enclave")
	}
}
