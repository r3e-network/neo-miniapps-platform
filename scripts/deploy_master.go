// +build ignore

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type DeployMasterInput struct {
	NEFBase64    string `json:"nef_base64"`
	ManifestJSON string `json:"manifest_json"`
}

type DeployMasterResponse struct {
	TxHash       string `json:"tx_hash"`
	ContractHash string `json:"contract_hash"`
	GasConsumed  string `json:"gas_consumed"`
	AccountID    string `json:"account_id"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run deploy_master.go <nef_file> <manifest_file>")
		os.Exit(1)
	}

	nefPath := os.Args[1]
	manifestPath := os.Args[2]

	// Read NEF file and encode to base64
	nefBytes, err := os.ReadFile(nefPath)
	if err != nil {
		fmt.Printf("Error reading NEF file: %v\n", err)
		os.Exit(1)
	}
	nefBase64 := base64.StdEncoding.EncodeToString(nefBytes)

	// Read manifest file
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("Error reading manifest file: %v\n", err)
		os.Exit(1)
	}

	// Compact the JSON
	var manifestJSON bytes.Buffer
	if err := json.Compact(&manifestJSON, manifestBytes); err != nil {
		fmt.Printf("Error compacting manifest JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("NEF size: %d bytes (base64: %d)\n", len(nefBytes), len(nefBase64))
	fmt.Printf("Manifest size: %d bytes\n", len(manifestJSON.Bytes()))

	// Build request
	input := DeployMasterInput{
		NEFBase64:    nefBase64,
		ManifestJSON: manifestJSON.String(),
	}

	reqBody, err := json.Marshal(input)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		os.Exit(1)
	}

	// Send request
	baseURL := os.Getenv("NEOACCOUNTS_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8082"
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(baseURL+"/deploy-master", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error response (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result DeployMasterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("Error parsing response: %v\nRaw: %s\n", err, string(body))
		os.Exit(1)
	}

	fmt.Printf("\n✅ Contract deployed successfully!\n")
	fmt.Printf("   TX Hash:       %s\n", result.TxHash)
	fmt.Printf("   Contract Hash: %s\n", result.ContractHash)
	fmt.Printf("   Gas Consumed:  %s\n", result.GasConsumed)
	fmt.Printf("   Account:       %s\n", result.AccountID)
}
