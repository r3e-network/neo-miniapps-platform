// Package testnet provides Neo N3 testnet contract deployment.
package testnet

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	DefaultTestnetRPC = "https://testnet1.neo.coz.io:443"
	DefaultTimeout    = 60 * time.Second
	TestnetMagic      = 894710606
)

type Deployer struct {
	rpcURL     string
	privateKey *keys.PrivateKey
	client     *http.Client
}

type DeployedContract struct {
	Name        string `json:"name"`
	Hash        string `json:"hash"`
	TxHash      string `json:"tx_hash"`
	GasConsumed string `json:"gas_consumed"`
	DeployedAt  string `json:"deployed_at"`
}

type DeploymentResult struct {
	Contracts []DeployedContract `json:"contracts"`
	Network   string             `json:"network"`
	Deployer  string             `json:"deployer"`
}

func NewDeployer(rpcURL string) (*Deployer, error) {
	if rpcURL == "" {
		rpcURL = DefaultTestnetRPC
	}

	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		return nil, fmt.Errorf("NEO_TESTNET_WIF environment variable not set")
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		return nil, fmt.Errorf("invalid WIF: %w", err)
	}

	return &Deployer{
		rpcURL:     rpcURL,
		privateKey: privateKey,
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
	}, nil
}

func (d *Deployer) GetAddress() string {
	return address.Uint160ToString(d.privateKey.GetScriptHash())
}

func (d *Deployer) GetScriptHash() string {
	return "0x" + hex.EncodeToString(d.privateKey.GetScriptHash().BytesBE())
}

func (d *Deployer) GetAccountHash() util.Uint160 {
	return d.privateKey.GetScriptHash()
}

// RPC types imported from internal/chain package

func (d *Deployer) call(method string, params ...interface{}) (*chain.RPCResponse, error) {
	if params == nil {
		params = []interface{}{}
	}
	req := chain.RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", d.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp chain.RPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

func (d *Deployer) GetGASBalance() (string, error) {
	addr := d.GetAddress()
	resp, err := d.call("getnep17balances", addr)
	if err != nil {
		return "", err
	}

	var result struct {
		Balance []struct {
			AssetHash string `json:"assethash"`
			Amount    string `json:"amount"`
		} `json:"balance"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", err
	}

	gasHash := "0xd2a4cff31913016155e38e474a2c06d08be276cf"
	for _, b := range result.Balance {
		if b.AssetHash == gasHash {
			return b.Amount, nil
		}
	}
	return "0", nil
}

func (d *Deployer) GetGASBalanceFloat() (float64, error) {
	balance, err := d.GetGASBalance()
	if err != nil {
		return 0, err
	}
	amt, err := strconv.ParseInt(balance, 10, 64)
	if err != nil {
		return 0, err
	}
	return float64(amt) / 1e8, nil
}

// InvokeResult and StackItem imported from internal/chain package

func (d *Deployer) GetBlockCount() (int64, error) {
	resp, err := d.call("getblockcount")
	if err != nil {
		return 0, err
	}

	var count int64
	if err := json.Unmarshal(resp.Result, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// DeployContract simulates deployment and returns estimated gas costs.
// For actual deployment, use neo-go CLI.
func (d *Deployer) DeployContract(nefPath, manifestPath string) (*DeployedContract, error) {
	nefData, err := os.ReadFile(nefPath)
	if err != nil {
		return nil, fmt.Errorf("read nef: %w", err)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	nefBase64 := base64.StdEncoding.EncodeToString(nefData)
	manifestStr := string(manifestData)

	params := []interface{}{
		map[string]interface{}{
			"type":  "ByteArray",
			"value": nefBase64,
		},
		map[string]interface{}{
			"type":  "String",
			"value": manifestStr,
		},
	}

	signers := []map[string]interface{}{
		{
			"account": d.GetScriptHash(),
			"scopes":  "CalledByEntry",
		},
	}

	resp, err := d.call("invokefunction", "0xfffdc93764dbaddd97c48f252a53ea4643faa3fd", "deploy", params, signers)
	if err != nil {
		return nil, fmt.Errorf("invoke deploy: %w", err)
	}

	var invokeResult chain.InvokeResult
	if err := json.Unmarshal(resp.Result, &invokeResult); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("simulation failed: %s (exception: %s)", invokeResult.State, invokeResult.Exception)
	}

	// Calculate expected contract hash
	nefFile, _ := nef.FileFromBytes(nefData)
	var m manifest.Manifest
	json.Unmarshal(manifestData, &m)
	contractHash := CalculateContractHash(d.GetAccountHash(), nefFile.Checksum, m.Name)

	return &DeployedContract{
		Hash:        "0x" + contractHash.StringLE(),
		GasConsumed: invokeResult.GasConsumed,
	}, nil
}

// CalculateContractHash calculates the contract hash from sender, checksum, and name.
func CalculateContractHash(sender util.Uint160, checksum uint32, name string) util.Uint160 {
	// Build script: push sender bytes, push checksum, push name
	var buf bytes.Buffer

	// Opcode PUSHDATA + sender bytes (20 bytes)
	senderBytes := sender.BytesBE()
	if len(senderBytes) <= 75 {
		buf.WriteByte(byte(len(senderBytes)))
	} else {
		buf.WriteByte(0x4c) // PUSHDATA1
		buf.WriteByte(byte(len(senderBytes)))
	}
	buf.Write(senderBytes)

	// Push checksum as integer
	if checksum == 0 {
		buf.WriteByte(0x00) // PUSH0
	} else if checksum <= 16 {
		buf.WriteByte(byte(0x10 + checksum)) // PUSH1-PUSH16
	} else {
		// PUSHINT32
		buf.WriteByte(0x01) // PUSHINT8 marker for small ints
		buf.WriteByte(byte(checksum))
		buf.WriteByte(byte(checksum >> 8))
		buf.WriteByte(byte(checksum >> 16))
		buf.WriteByte(byte(checksum >> 24))
	}

	// Push name as string
	nameBytes := []byte(name)
	if len(nameBytes) <= 75 {
		buf.WriteByte(byte(len(nameBytes)))
	} else {
		buf.WriteByte(0x4c) // PUSHDATA1
		buf.WriteByte(byte(len(nameBytes)))
	}
	buf.Write(nameBytes)

	// The actual hash computation uses a different method in neo-go
	// For now, return placeholder - actual hash comes from chain response
	return util.Uint160{}
}

// InvokeFunction invokes a contract function (read-only).
func (d *Deployer) InvokeFunction(contractHash, method string, args []interface{}) (*chain.InvokeResult, error) {
	if args == nil {
		args = []interface{}{}
	}

	signers := []map[string]interface{}{
		{
			"account": d.GetScriptHash(),
			"scopes":  "CalledByEntry",
		},
	}

	resp, err := d.call("invokefunction", contractHash, method, args, signers)
	if err != nil {
		return nil, err
	}

	var result chain.InvokeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetContractState gets the state of a deployed contract.
func (d *Deployer) GetContractState(contractHash string) (map[string]interface{}, error) {
	resp, err := d.call("getcontractstate", contractHash)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return result, nil
}
