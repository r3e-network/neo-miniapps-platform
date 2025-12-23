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
)

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type ContractParam struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Signer struct {
	Account string `json:"account"`
	Scopes  string `json:"scopes"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run test_deploy_simulation.go <nef_file> <manifest_file>")
		os.Exit(1)
	}

	nefPath := os.Args[1]
	manifestPath := os.Args[2]

	// Read and encode NEF
	nefBytes, err := os.ReadFile(nefPath)
	if err != nil {
		fmt.Printf("Error reading NEF: %v\n", err)
		os.Exit(1)
	}
	nefBase64 := base64.StdEncoding.EncodeToString(nefBytes)

	// Read and encode manifest
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("Error reading manifest: %v\n", err)
		os.Exit(1)
	}
	manifestBase64 := base64.StdEncoding.EncodeToString(manifestBytes)

	fmt.Printf("NEF size: %d bytes (base64: %d)\n", len(nefBytes), len(nefBase64))
	fmt.Printf("Manifest size: %d bytes (base64: %d)\n", len(manifestBytes), len(manifestBase64))

	// Master account script hash (NLtL2v28d7TyMEaXcPqtekunkFRksJ7wxu)
	// Script hash: 0x7f3c5f40d54eadc478c080bb92e92231ee6bd179
	masterScriptHash := "0x7f3c5f40d54eadc478c080bb92e92231ee6bd179"

	// Build RPC request
	params := []ContractParam{
		{Type: "ByteArray", Value: nefBase64},
		{Type: "ByteArray", Value: manifestBase64},
	}
	signers := []Signer{
		{Account: masterScriptHash, Scopes: "CalledByEntry"},
	}

	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "invokefunction",
		Params: []interface{}{
			"0xfffdc93764dbaddd97c48f252a53ea4643faa3fd", // ContractManagement
			"deploy",
			params,
			signers,
		},
		ID: 1,
	}

	reqBody, _ := json.Marshal(req)
	fmt.Printf("\nRequest body length: %d bytes\n", len(reqBody))

	// Send request
	resp, err := http.Post("https://testnet1.neo.coz.io:443", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	prettyJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("\nResponse:\n%s\n", string(prettyJSON))
}
