package main

import (
	"context"
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
	accountPool := flag.String("accountpool", "", "AccountPool base URL (https://host:port)")
	flag.Parse()

	if *rpc == "" || *gateway == "" || *accountPool == "" {
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mk, err := fetchMasterKey(*accountPool)
	if err != nil {
		log.Fatalf("fetch master key: %v", err)
	}

	client, err := chain.NewClient(chain.Config{RPCURL: *rpc})
	if err != nil {
		log.Fatalf("client: %v", err)
	}
	gw := chain.NewGatewayContract(client, trim0x(*gateway), nil)

	onchainPubKey, err := gw.GetTEEMasterPubKey(ctx)
	if err != nil {
		log.Fatalf("get on-chain pubkey: %v", err)
	}
	onchainHash, err := gw.GetTEEMasterPubKeyHash(ctx)
	if err != nil {
		log.Fatalf("get on-chain pubkey hash: %v", err)
	}
	onchainAttest, err := gw.GetTEEMasterAttestationHash(ctx)
	if err != nil {
		log.Fatalf("get on-chain attestation hash: %v", err)
	}

	fmt.Println("AccountPool /master-key:")
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
		os.Exit(1)
	}
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

func trim0x(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "0x") {
		return s[2:]
	}
	return s
}
