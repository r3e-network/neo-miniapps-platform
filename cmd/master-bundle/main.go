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
	accountPool := flag.String("accountpool", "", "AccountPool base URL (https://host:port)")
	out := flag.String("out", "", "Output file for bundle (optional)")
	flag.Parse()

	if *accountPool == "" {
		flag.Usage()
		os.Exit(1)
	}

	bundle, err := fetchMasterKey(*accountPool)
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
		if err := os.WriteFile(*out, data, 0644); err != nil {
			log.Fatalf("write bundle: %v", err)
		}
		fmt.Printf("bundle written to %s\n", *out)
	} else {
		fmt.Println(string(data))
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
