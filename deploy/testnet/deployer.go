// Package testnet provides Neo N3 testnet contract deployment.
package testnet

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/httputil"
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

// DeployedContract and DeploymentResult are imported from internal/chain package

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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, truncated, readErr := httputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}
		msg := string(respBody)
		if truncated {
			msg += "...(truncated)"
		}
		return nil, fmt.Errorf("rpc http error %d: %s", resp.StatusCode, msg)
	}

	respBody, err := httputil.ReadAllStrict(resp.Body, 8<<20)
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
func (d *Deployer) DeployContract(nefPath, manifestPath string) (*chain.DeployedContract, error) {
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
	if unmarshalErr := json.Unmarshal(resp.Result, &invokeResult); unmarshalErr != nil {
		return nil, fmt.Errorf("parse result: %w", unmarshalErr)
	}

	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("simulation failed: %s (exception: %s)", invokeResult.State, invokeResult.Exception)
	}

	// Calculate expected contract hash
	nefFile, err := nef.FileFromBytes(nefData)
	if err != nil {
		return nil, fmt.Errorf("parse nef: %w", err)
	}
	var m manifest.Manifest
	if err := json.Unmarshal(manifestData, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	contractHash := state.CreateContractHash(d.GetAccountHash(), nefFile.Checksum, m.Name)

	return &chain.DeployedContract{
		Hash:        "0x" + contractHash.StringLE(),
		GasConsumed: invokeResult.GasConsumed,
	}, nil
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
