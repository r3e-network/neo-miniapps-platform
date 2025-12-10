package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// Verifies a master-key attestation bundle hash matches the expected on-chain attestation hash.
// Bundle fields expected: pubkey, hash (sha256(pubkey)), quote (optional for this check).
func main() {
	bundleURI := flag.String("bundle", "", "Bundle URI (file:///path or https://...) containing pubkey/hash/quote")
	expected := flag.String("expected-hash", "", "Expected SHA-256(bundle) hex (on-chain attestation hash)")
	flag.Parse()

	if *bundleURI == "" || *expected == "" {
		flag.Usage()
		os.Exit(1)
	}

	data, err := fetch(*bundleURI)
	if err != nil {
		log.Fatalf("fetch bundle: %v", err)
	}

	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), trim0x(*expected)) {
		log.Fatalf("bundle hash mismatch: got %s want %s", hex.EncodeToString(sum[:]), trim0x(*expected))
	}

	var body struct {
		Hash   string `json:"hash"`
		PubKey string `json:"pubkey"`
		Quote  string `json:"quote"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		log.Fatalf("decode bundle: %v", err)
	}
	if body.PubKey == "" || body.Hash == "" {
		log.Fatalf("bundle missing pubkey/hash")
	}

	fmt.Printf("Bundle OK. PubKey=%s Hash=%s BundleHash=%s\n", body.PubKey, body.Hash, hex.EncodeToString(sum[:]))
}

func fetch(uri string) ([]byte, error) {
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		return os.ReadFile(path)
	}
	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func trim0x(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "0x") {
		return s[2:]
	}
	return s
}
