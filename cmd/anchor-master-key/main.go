package main

import (
	"context"
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
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
)

type masterKeyResponse struct {
	Hash   string `json:"hash"`
	PubKey string `json:"pubkey"`
}

func main() {
	rpc := flag.String("rpc", "", "Neo RPC URL")
	gateway := flag.String("gateway", "", "Gateway contract hash (0x-prefixed)")
	privHex := flag.String("priv", "", "Hex-encoded private key (admin)")
	pubKeyHex := flag.String("pubkey", "", "Compressed pubkey hex to anchor (optional if --neoaccounts is set)")
	pubKeyHashHex := flag.String("pubkey-hash", "", "SHA-256 hash of pubkey (hex, 32 bytes; optional if --neoaccounts is set)")
	attestHashHex := flag.String("attest-hash", "", "Attestation bundle hash/CID (hex; optional if --bundle is set)")
	neoAccounts := flag.String("neoaccounts", "", "NeoAccounts base URL; if set, fetch pubkey/hash from /master-key")
	bundleURI := flag.String("bundle", "", "Optional bundle URI (file:/// or https://) to compute attestation hash automatically")
	flag.Parse()

	if *rpc == "" || *gateway == "" || *privHex == "" || (*attestHashHex == "" && *bundleURI == "") {
		flag.Usage()
		os.Exit(1)
	}

	mk := masterKeyResponse{Hash: *pubKeyHashHex, PubKey: *pubKeyHex}
	var err error
	if *neoAccounts != "" {
		mk, err = fetchMasterKey(*neoAccounts)
		if err != nil {
			log.Fatalf("fetch master key: %v", err)
		}
	} else {
		if mk.PubKey == "" || mk.Hash == "" {
			flag.Usage()
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := chain.NewClient(chain.Config{RPCURL: *rpc})
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	fulfiller, err := chain.NewTEEFulfiller(client, trim0x(*gateway), trim0x(*privHex))
	if err != nil {
		log.Fatalf("fulfiller: %v", err)
	}

	pubKey, err := hex.DecodeString(trim0x(mk.PubKey))
	if err != nil {
		log.Fatalf("pubkey decode: %v", err)
	}
	pubKeyHash, err := hex.DecodeString(trim0x(mk.Hash))
	if err != nil || len(pubKeyHash) != sha256.Size {
		log.Fatalf("pubkey-hash decode: %v", err)
	}
	attestHex := *attestHashHex
	if attestHex == "" && *bundleURI != "" {
		bundleHash, derr := hashBundle(*bundleURI)
		if derr != nil {
			log.Fatalf("bundle hash: %v", derr)
		}
		attestHex = bundleHash
	}
	attestHash, err := hex.DecodeString(trim0x(attestHex))
	if err != nil {
		log.Fatalf("attestation-hash decode: %v", err)
	}

	txResult, err := fulfiller.SetTEEMasterKey(ctx, pubKey, pubKeyHash, attestHash)
	if err != nil {
		log.Fatalf("set master key: %v", err)
	}

	fmt.Printf("Anchored master key. Tx: %s VMState: %s\n", txResult.TxHash, txResult.VMState)
}

func fetchMasterKey(baseURL string) (masterKeyResponse, error) {
	url := strings.TrimRight(baseURL, "/") + "/master-key"
	resp, err := http.Get(url)
	if err != nil {
		return masterKeyResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return masterKeyResponse{}, fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
	}
	var body masterKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return masterKeyResponse{}, err
	}
	if body.PubKey == "" || body.Hash == "" {
		return masterKeyResponse{}, fmt.Errorf("/master-key missing pubkey/hash")
	}
	return body, nil
}

func hashBundle(uri string) (string, error) {
	data, err := fetch(uri)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
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
