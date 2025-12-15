package main

import (
	"context"
	"crypto/sha256"
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

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// masterBundle pulls /master-key, wraps it, and prints SHA-256(bundle) for on-chain anchoring.
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
	neoAccounts := flag.String("neoaccounts", "", "NeoAccounts (or gateway) base URL (https://host:port)")
	out := flag.String("out", "", "Output file for bundle (optional)")
	flag.Parse()

	if *neoAccounts == "" {
		flag.Usage()
		os.Exit(1)
	}

	bundle, err := fetchMasterKey(*neoAccounts)
	if err != nil {
		log.Fatalf("fetch master key: %v", err)
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		log.Fatalf("marshal bundle: %v", err)
	}

	sum := sha256.Sum256(data)
	fmt.Printf("bundle_sha256=%s\n", hex.EncodeToString(sum[:]))

	if *out != "" {
		if err := os.WriteFile(*out, data, 0o600); err != nil {
			log.Fatalf("write bundle: %v", err)
		}
		fmt.Printf("bundle written to %s\n", *out)
	} else {
		fmt.Println(string(data))
	}
}

func fetchMasterKey(baseURL string) (masterKeyResponse, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/master-key")
	if err != nil {
		return masterKeyResponse{}, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return masterKeyResponse{}, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
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
