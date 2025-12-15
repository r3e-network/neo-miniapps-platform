package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/httputil"
)

type masterKeyResponse struct {
	Hash      string `json:"hash"`
	PubKey    string `json:"pubkey"`
	Quote     string `json:"quote"`
	MRENCLAVE string `json:"mrenclave"`
	MRSIGNER  string `json:"mrsigner"`
	ProdID    uint16 `json:"prod_id"`
	ISVSVN    uint16 `json:"isvsvn"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Simulated bool   `json:"simulated"`
}

func main() {
	rpc := flag.String("rpc", "", "Neo RPC URL")
	gateway := flag.String("gateway", "", "Gateway contract hash (0x-prefixed)")
	neoAccounts := flag.String("neoaccounts", "", "NeoAccounts (or gateway) base URL (https://host:port)")
	flag.Parse()

	if *rpc == "" || *gateway == "" || *neoAccounts == "" {
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// Avoid defers in CLI entrypoints that may call os.Exit/log.Fatalf.
	// Context is canceled explicitly before exit.

	mk, err := fetchMasterKey(*neoAccounts)
	if err != nil {
		cancel()
		log.Fatalf("fetch master key: %v", err)
	}

	client, err := chain.NewClient(chain.Config{RPCURL: *rpc})
	if err != nil {
		cancel()
		log.Fatalf("client: %v", err)
	}
	gw := chain.NewGatewayContract(client, trim0x(*gateway), nil)

	onchainPubKey, err := gw.GetTEEMasterPubKey(ctx)
	if err != nil {
		cancel()
		log.Fatalf("get on-chain pubkey: %v", err)
	}
	onchainHash, err := gw.GetTEEMasterPubKeyHash(ctx)
	if err != nil {
		cancel()
		log.Fatalf("get on-chain pubkey hash: %v", err)
	}
	onchainAttest, err := gw.GetTEEMasterAttestationHash(ctx)
	if err != nil {
		cancel()
		log.Fatalf("get on-chain attestation hash: %v", err)
	}

	fmt.Println("NeoAccounts /master-key:")
	fmt.Printf("  pubkey: %s\n", mk.PubKey)
	fmt.Printf("  hash:   %s\n", mk.Hash)
	fmt.Println("Gateway anchor:")
	fmt.Printf("  pubkey: %s\n", hex.EncodeToString(onchainPubKey))
	fmt.Printf("  hash:   %s\n", hex.EncodeToString(onchainHash))
	fmt.Printf("  attest: %s\n", hex.EncodeToString(onchainAttest))

	okPub := strings.EqualFold(mk.PubKey, hex.EncodeToString(onchainPubKey))
	okHash := strings.EqualFold(mk.Hash, hex.EncodeToString(onchainHash))

	fmt.Println("Checks:")
	fmt.Printf("  pubkey match: %v\n", okPub)
	fmt.Printf("  hash match:   %v\n", okHash)

	if !okPub || !okHash {
		cancel()
		os.Exit(1)
	}

	cancel()
}

func fetchMasterKey(baseURL string) (masterKeyResponse, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/master-key")
	if err != nil {
		return masterKeyResponse{}, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return masterKeyResponse{}, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return masterKeyResponse{}, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return masterKeyResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, truncated, readErr := httputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return masterKeyResponse{}, fmt.Errorf("http %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		msg := string(b)
		if truncated {
			msg += "...(truncated)"
		}
		return masterKeyResponse{}, fmt.Errorf("http %d: %s", resp.StatusCode, msg)
	}
	var body masterKeyResponse
	data, err := httputil.ReadAllStrict(resp.Body, 1<<20)
	if err != nil {
		return masterKeyResponse{}, fmt.Errorf("read /master-key response: %w", err)
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return masterKeyResponse{}, fmt.Errorf("decode /master-key response: %w", err)
	}
	if body.PubKey == "" || body.Hash == "" {
		return masterKeyResponse{}, fmt.Errorf("/master-key missing pubkey/hash")
	}
	return body, nil
}

func trim0x(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "0x") {
		return s[2:]
	}
	return s
}
