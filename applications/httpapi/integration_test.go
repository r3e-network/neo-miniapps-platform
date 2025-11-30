package httpapi

import (
	"testing"
)

// Basic HTTP integration smoke test covering health, auth, accounts, wallets, datafeeds, secrets, oracle, audit, random, datalink.
func TestIntegrationHTTPAPI(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}
