// Package neoaccounts provides transaction signing for the neoaccounts service.
package neoaccounts

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/wallet"

	"github.com/R3E-Network/service_layer/infrastructure/accountpool/supabase"
	"github.com/R3E-Network/service_layer/infrastructure/chain"
	intcrypto "github.com/R3E-Network/service_layer/infrastructure/crypto"
)

// SignTransaction signs a transaction hash with an account's private key.
// The account must be locked by the requesting service.
func (s *Service) SignTransaction(ctx context.Context, serviceID, accountID string, txHash []byte) (*SignTransactionResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}

	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	signature, err := signHash(priv, txHash)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	pubBytes := intcrypto.PublicKeyToBytes(&priv.PublicKey)

	return &SignTransactionResponse{
		AccountID: accountID,
		Signature: signature,
		PublicKey: pubBytes,
	}, nil
}

// BatchSign signs multiple transaction hashes.
func (s *Service) BatchSign(ctx context.Context, serviceID string, requests []SignRequest) *BatchSignResponse {
	resp := &BatchSignResponse{
		Signatures: make([]SignTransactionResponse, 0, len(requests)),
		Errors:     make([]string, 0),
	}

	for _, req := range requests {
		sig, err := s.SignTransaction(ctx, serviceID, req.AccountID, req.TxHash)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", req.AccountID, err))
			continue
		}
		resp.Signatures = append(resp.Signatures, *sig)
	}

	return resp
}

// signHash signs a hash using ECDSA.
func signHash(priv *ecdsa.PrivateKey, hash []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash)
	if err != nil {
		return nil, err
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()

	signature := make([]byte, 64)
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// verifySignature verifies an ECDSA signature.
func verifySignature(pub *ecdsa.PublicKey, hash, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}

	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(pub, hash, r, s)
}

// Transfer transfers tokens from a pool account to a target address.
// The account must be locked by the requesting service.
//
// The transfer is executed as an on-chain NEP-17 `transfer(from,to,amount,data)` invocation
// signed by the pool account's derived private key.
func (s *Service) Transfer(ctx context.Context, serviceID, accountID, toAddress string, amount int64, tokenHash string) (string, error) {
	if s.repo == nil {
		return "", fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return "", fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return "", fmt.Errorf("account_id required")
	}
	if toAddress == "" {
		return "", fmt.Errorf("to_address required")
	}
	if amount <= 0 {
		return "", fmt.Errorf("amount must be positive")
	}

	tokenHash = strings.TrimSpace(tokenHash)
	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return "", fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return "", fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key and build a neo-go wallet account.
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	walletAccount, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return "", fmt.Errorf("create signer account: %w", err)
	}

	// Convert to address to script hash
	toU160, err := address.StringToUint160(strings.TrimSpace(toAddress))
	if err != nil {
		return "", fmt.Errorf("invalid to address %q: %w", toAddress, err)
	}

	// Use the chain client's TransferGAS method which uses the actor pattern
	txHash, err := s.chainClient.TransferGAS(ctx, walletAccount, toU160, big.NewInt(amount))
	if err != nil {
		return "", fmt.Errorf("transfer GAS: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Best-effort account metadata update; the chain tx succeeded regardless.
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).WithFields(map[string]interface{}{
			"account_id": accountID,
			"tx_hash":    txHashString,
		}).Warn("failed to update account metadata after transfer")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id": accountID,
		"to_address": toAddress,
		"amount":     amount,
		"tx_hash":    txHashString,
	}).Info("transfer completed")

	return txHashString, nil
}

// DeployContract deploys a new smart contract using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
func (s *Service) DeployContract(ctx context.Context, serviceID, accountID, nefBase64, manifestJSON string, data any) (*DeployContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if nefBase64 == "" {
		return nil, fmt.Errorf("nef_base64 required")
	}
	if manifestJSON == "" {
		return nil, fmt.Errorf("manifest_json required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Decode NEF from base64
	nefBytes, err := base64.StdEncoding.DecodeString(nefBase64)
	if err != nil {
		return nil, fmt.Errorf("decode nef base64: %w", err)
	}

	// Build deployment parameters
	params := []chain.ContractParam{
		chain.NewByteArrayParam(nefBytes),
		chain.NewStringParam(manifestJSON),
	}
	if data != nil {
		params = append(params, chain.NewAnyParam())
	}

	// ContractManagement native contract hash
	contractMgmtHash := "0xfffdc93764dbaddd97c48f252a53ea4643faa3fd"

	// Simulate deployment first
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractMgmtHash, "deploy", params, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("deployment simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("deployment simulation faulted: %s", invokeResult.Exception)
	}

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, transaction.CalledByEntry)
	if err != nil {
		return nil, fmt.Errorf("build deployment transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast deployment: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return nil, fmt.Errorf("wait for deployment execution: %w", err)
	}

	// Extract contract hash from deployment result
	contractHash := ""
	if appLog != nil && len(appLog.Executions) > 0 {
		exec := appLog.Executions[0]
		if exec.VMState != "HALT" {
			return nil, fmt.Errorf("deployment failed with state: %s", exec.VMState)
		}
		// Contract hash is typically in the first notification or stack result
		// The stack item contains the deployed contract state as a struct
		if len(exec.Stack) > 0 {
			// Try to extract hash from the stack item's Value field
			var valueMap map[string]any
			if err := json.Unmarshal(exec.Stack[0].Value, &valueMap); err == nil {
				if h, ok := valueMap["hash"].(string); ok {
					contractHash = h
				}
			}
		}
	}

	// Update account metadata
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).Warn("failed to update account metadata after deploy")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id":    accountID,
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("contract deployed")

	return &DeployContractResponse{
		TxHash:       txHashString,
		ContractHash: contractHash,
		GasConsumed:  invokeResult.GasConsumed,
		AccountID:    accountID,
	}, nil
}

// UpdateContract updates an existing smart contract using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
func (s *Service) UpdateContract(ctx context.Context, serviceID, accountID, contractHash, nefBase64, manifestJSON string, data any) (*UpdateContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if contractHash == "" {
		return nil, fmt.Errorf("contract_hash required")
	}
	if nefBase64 == "" {
		return nil, fmt.Errorf("nef_base64 required")
	}
	if manifestJSON == "" {
		return nil, fmt.Errorf("manifest_json required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Decode NEF from base64
	nefBytes, err := base64.StdEncoding.DecodeString(nefBase64)
	if err != nil {
		return nil, fmt.Errorf("decode nef base64: %w", err)
	}

	// Build update parameters - call update on the contract itself
	params := []chain.ContractParam{
		chain.NewByteArrayParam(nefBytes),
		chain.NewStringParam(manifestJSON),
	}
	if data != nil {
		params = append(params, chain.NewAnyParam())
	}

	// Simulate update
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractHash, "update", params, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("update simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("update simulation faulted: %s", invokeResult.Exception)
	}

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, transaction.CalledByEntry)
	if err != nil {
		return nil, fmt.Errorf("build update transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast update: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return nil, fmt.Errorf("wait for update execution: %w", err)
	}
	if appLog != nil && len(appLog.Executions) > 0 && appLog.Executions[0].VMState != "HALT" {
		return nil, fmt.Errorf("update failed with state: %s", appLog.Executions[0].VMState)
	}

	// Update account metadata
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).Warn("failed to update account metadata after update")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id":    accountID,
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("contract updated")

	return &UpdateContractResponse{
		TxHash:       txHashString,
		ContractHash: contractHash,
		GasConsumed:  invokeResult.GasConsumed,
		AccountID:    accountID,
	}, nil
}

// InvokeContract invokes a contract method using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
func (s *Service) InvokeContract(ctx context.Context, serviceID, accountID, contractHash, method string, params []ContractParam, scope string) (*InvokeContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if contractHash == "" {
		return nil, fmt.Errorf("contract_hash required")
	}
	if method == "" {
		return nil, fmt.Errorf("method required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Convert params to chain.ContractParam
	chainParams := make([]chain.ContractParam, len(params))
	for i, p := range params {
		chainParams[i] = convertToChainParam(p)
	}

	// Simulate invocation
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractHash, method, chainParams, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("invocation simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return &InvokeContractResponse{
			State:       invokeResult.State,
			GasConsumed: invokeResult.GasConsumed,
			Exception:   invokeResult.Exception,
			AccountID:   accountID,
		}, fmt.Errorf("invocation simulation faulted: %s", invokeResult.Exception)
	}

	// Determine transaction scope (default to CalledByEntry for safety)
	txScope := transaction.CalledByEntry
	switch strings.ToLower(scope) {
	case "global":
		txScope = transaction.Global
	case "customcontracts":
		txScope = transaction.CustomContracts
	case "customgroups":
		txScope = transaction.CustomGroups
	case "none":
		txScope = transaction.None
	case "calledbyentry", "":
		txScope = transaction.CalledByEntry
	}

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, txScope)
	if err != nil {
		return nil, fmt.Errorf("build invocation transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast invocation: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return nil, fmt.Errorf("wait for invocation execution: %w", err)
	}

	state := "HALT"
	exception := ""
	if appLog != nil && len(appLog.Executions) > 0 {
		state = appLog.Executions[0].VMState
		exception = appLog.Executions[0].Exception
	}

	// Update account metadata
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).Warn("failed to update account metadata after invoke")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id":    accountID,
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"method":        method,
		"scope":         scope,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("contract invoked")

	return &InvokeContractResponse{
		TxHash:      txHashString,
		State:       state,
		GasConsumed: invokeResult.GasConsumed,
		Exception:   exception,
		AccountID:   accountID,
	}, nil
}

// SimulateContract simulates a contract invocation without signing or broadcasting.
func (s *Service) SimulateContract(ctx context.Context, serviceID, accountID, contractHash, method string, params []ContractParam) (*SimulateContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if contractHash == "" {
		return nil, fmt.Errorf("contract_hash required")
	}
	if method == "" {
		return nil, fmt.Errorf("method required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE (only for getting script hash)
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Convert params to chain.ContractParam
	chainParams := make([]chain.ContractParam, len(params))
	for i, p := range params {
		chainParams[i] = convertToChainParam(p)
	}

	// Simulate invocation
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractHash, method, chainParams, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("simulation failed: %w", err)
	}

	return &SimulateContractResponse{
		State:       invokeResult.State,
		GasConsumed: invokeResult.GasConsumed,
		Exception:   invokeResult.Exception,
	}, nil
}

// convertToChainParam converts a ContractParam to chain.ContractParam.
func convertToChainParam(p ContractParam) chain.ContractParam {
	switch strings.ToLower(p.Type) {
	case "hash160":
		if s, ok := p.Value.(string); ok {
			// If it looks like a Neo address (starts with N), convert to script hash
			if len(s) > 0 && s[0] == 'N' {
				u160, err := address.StringToUint160(s)
				if err == nil {
					// Return as 0x-prefixed little-endian hex string
					return chain.NewHash160Param("0x" + u160.StringLE())
				}
			}
			return chain.NewHash160Param(s)
		}
	case "integer":
		switch v := p.Value.(type) {
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return chain.NewIntegerParam(big.NewInt(i))
			}
		case float64:
			return chain.NewIntegerParam(big.NewInt(int64(v)))
		case int64:
			return chain.NewIntegerParam(big.NewInt(v))
		case int:
			return chain.NewIntegerParam(big.NewInt(int64(v)))
		}
	case "string":
		if s, ok := p.Value.(string); ok {
			return chain.NewStringParam(s)
		}
	case "bytearray":
		if s, ok := p.Value.(string); ok {
			// Try base64 decode first, fall back to hex
			if bytes, err := base64.StdEncoding.DecodeString(s); err == nil {
				return chain.NewByteArrayParam(bytes)
			}
			// Try hex decode
			if bytes, err := hex.DecodeString(s); err == nil {
				return chain.NewByteArrayParam(bytes)
			}
			// Use as raw bytes
			return chain.NewByteArrayParam([]byte(s))
		}
	case "bool", "boolean":
		switch v := p.Value.(type) {
		case bool:
			return chain.NewBoolParam(v)
		case string:
			return chain.NewBoolParam(v == "true" || v == "1")
		}
	case "any":
		return chain.NewAnyParam()
	}
	// Default to any
	return chain.NewAnyParam()
}

// FundAccount transfers tokens from the master wallet (TEE_PRIVATE_KEY) to a target address.
// This is used to fund pool accounts with GAS for transaction fees.
// Unlike Transfer(), this uses the master wallet directly, not a pool account.
// After successful transfer, updates the database balance for the target account.
func (s *Service) FundAccount(ctx context.Context, toAddress string, amount int64, tokenHash string) (*FundAccountResponse, error) {
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if toAddress == "" {
		return nil, fmt.Errorf("to_address required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Load TEE_PRIVATE_KEY from environment - try WIF first, then hex
	teePrivateKey := strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
	if teePrivateKey == "" {
		teePrivateKey = strings.TrimSpace(os.Getenv("TEE_PRIVATE_KEY"))
	}
	if teePrivateKey == "" {
		teePrivateKey = strings.TrimSpace(os.Getenv("TEE_WALLET_PRIVATE_KEY"))
	}
	if teePrivateKey == "" {
		return nil, fmt.Errorf("TEE_PRIVATE_KEY not configured")
	}

	// Create signer - try WIF format first, then hex
	var walletAccount *wallet.Account
	var err error

	// Check if it looks like a WIF (starts with K, L, or 5)
	if len(teePrivateKey) > 0 && (teePrivateKey[0] == 'K' || teePrivateKey[0] == 'L' || teePrivateKey[0] == '5') {
		walletAccount, err = chain.AccountFromWIF(teePrivateKey)
	} else {
		// Remove 0x prefix if present and try hex
		teePrivateKey = strings.TrimPrefix(strings.TrimPrefix(teePrivateKey, "0x"), "0X")
		walletAccount, err = chain.AccountFromPrivateKey(teePrivateKey)
	}
	if err != nil {
		return nil, fmt.Errorf("create signer from TEE_PRIVATE_KEY: %w", err)
	}

	fromAddress := walletAccount.Address

	// Convert to address to script hash
	toU160, err := address.StringToUint160(strings.TrimSpace(toAddress))
	if err != nil {
		return nil, fmt.Errorf("invalid to address %q: %w", toAddress, err)
	}

	// Use the chain client's TransferGAS method which uses the actor pattern
	txHash, err := s.chainClient.TransferGAS(ctx, walletAccount, toU160, big.NewInt(amount))
	if err != nil {
		return nil, fmt.Errorf("transfer GAS: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Update database balance for the target pool account
	if s.repo != nil {
		acc, accErr := s.repo.GetByAddress(ctx, toAddress)
		if accErr == nil && acc != nil {
			// Get GAS script hash and decimals
			scriptHash, decimals := supabase.GetDefaultTokenConfig(TokenTypeGAS)
			// Get current balance and add the funded amount
			currentBalance := int64(0)
			if existingBal, balErr := s.repo.GetBalance(ctx, acc.ID, TokenTypeGAS); balErr == nil && existingBal != nil {
				currentBalance = existingBal.Amount
			}
			newBalance := currentBalance + amount
			if upsertErr := s.repo.UpsertBalance(ctx, acc.ID, TokenTypeGAS, scriptHash, newBalance, decimals); upsertErr != nil {
				s.Logger().WithContext(ctx).WithError(upsertErr).Warn("failed to update database balance after fund transfer")
			} else {
				s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
					"account_id":  acc.ID,
					"old_balance": currentBalance,
					"new_balance": newBalance,
					"funded":      amount,
				}).Info("database balance updated after fund transfer")
			}
		}
	}

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"from_address": fromAddress,
		"to_address":   toAddress,
		"amount":       amount,
		"tx_hash":      txHashString,
	}).Info("fund transfer completed")

	return &FundAccountResponse{
		TxHash:      txHashString,
		FromAddress: fromAddress,
		ToAddress:   toAddress,
		Amount:      amount,
	}, nil
}

// InvokeMaster invokes a contract method using the master wallet (TEE_PRIVATE_KEY).
// This is used for TEE operations like PriceFeed and RandomnessLog that require
// the caller to be a registered TEE signer in AppRegistry.
// Unlike InvokeContract(), this uses the master wallet directly, not a pool account.
func (s *Service) InvokeMaster(ctx context.Context, contractHash, method string, params []ContractParam, scope string) (*InvokeContractResponse, error) {
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if contractHash == "" {
		return nil, fmt.Errorf("contract_hash required")
	}
	if method == "" {
		return nil, fmt.Errorf("method required")
	}

	// Load TEE_PRIVATE_KEY from environment - try WIF first, then hex
	teePrivateKey := strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
	if teePrivateKey == "" {
		teePrivateKey = strings.TrimSpace(os.Getenv("TEE_PRIVATE_KEY"))
	}
	if teePrivateKey == "" {
		teePrivateKey = strings.TrimSpace(os.Getenv("TEE_WALLET_PRIVATE_KEY"))
	}
	if teePrivateKey == "" {
		return nil, fmt.Errorf("TEE_PRIVATE_KEY not configured")
	}

	// Create signer - try WIF format first, then hex
	var signer *wallet.Account
	var err error

	// Check if it looks like a WIF (starts with K, L, or 5)
	if len(teePrivateKey) > 0 && (teePrivateKey[0] == 'K' || teePrivateKey[0] == 'L' || teePrivateKey[0] == '5') {
		signer, err = chain.AccountFromWIF(teePrivateKey)
	} else {
		// Remove 0x prefix if present and try hex
		teePrivateKey = strings.TrimPrefix(strings.TrimPrefix(teePrivateKey, "0x"), "0X")
		signer, err = chain.AccountFromPrivateKey(teePrivateKey)
	}
	if err != nil {
		return nil, fmt.Errorf("create signer from TEE_PRIVATE_KEY: %w", err)
	}

	// Convert params to chain.ContractParam
	chainParams := make([]chain.ContractParam, len(params))
	for i, p := range params {
		chainParams[i] = convertToChainParam(p)
	}

	// Simulate invocation
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractHash, method, chainParams, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("invocation simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return &InvokeContractResponse{
			State:       invokeResult.State,
			GasConsumed: invokeResult.GasConsumed,
			Exception:   invokeResult.Exception,
			AccountID:   "master",
		}, fmt.Errorf("invocation simulation faulted: %s", invokeResult.Exception)
	}

	// Determine transaction scope (default to CalledByEntry for safety)
	txScope := transaction.CalledByEntry
	switch strings.ToLower(scope) {
	case "global":
		txScope = transaction.Global
	case "customcontracts":
		txScope = transaction.CustomContracts
	case "customgroups":
		txScope = transaction.CustomGroups
	case "none":
		txScope = transaction.None
	case "calledbyentry", "":
		txScope = transaction.CalledByEntry
	}

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, txScope)
	if err != nil {
		return nil, fmt.Errorf("build invocation transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast invocation: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return nil, fmt.Errorf("wait for invocation execution: %w", err)
	}

	state := "HALT"
	exception := ""
	if appLog != nil && len(appLog.Executions) > 0 {
		state = appLog.Executions[0].VMState
		exception = appLog.Executions[0].Exception
	}

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account":       "master",
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"method":        method,
		"scope":         scope,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("master contract invoked")

	return &InvokeContractResponse{
		TxHash:      txHashString,
		State:       state,
		GasConsumed: invokeResult.GasConsumed,
		Exception:   exception,
		AccountID:   "master",
	}, nil
}

// DeployMaster deploys a new smart contract using the master wallet (TEE_PRIVATE_KEY).
// This is used for deploying contracts where the master account needs to be the Admin.
// All signing happens inside TEE - private keys never leave the enclave.
func (s *Service) DeployMaster(ctx context.Context, nefBase64, manifestJSON string, data any) (*DeployMasterResponse, error) {
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if nefBase64 == "" {
		return nil, fmt.Errorf("nef_base64 required")
	}
	if manifestJSON == "" {
		return nil, fmt.Errorf("manifest_json required")
	}

	// Load TEE_PRIVATE_KEY from environment - try WIF first, then hex
	teePrivateKey := strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
	if teePrivateKey == "" {
		teePrivateKey = strings.TrimSpace(os.Getenv("TEE_PRIVATE_KEY"))
	}
	if teePrivateKey == "" {
		teePrivateKey = strings.TrimSpace(os.Getenv("TEE_WALLET_PRIVATE_KEY"))
	}
	if teePrivateKey == "" {
		return nil, fmt.Errorf("TEE_PRIVATE_KEY not configured")
	}

	// Create signer - try WIF format first, then hex
	var signer *wallet.Account
	var err error

	// Check if it looks like a WIF (starts with K, L, or 5)
	if len(teePrivateKey) > 0 && (teePrivateKey[0] == 'K' || teePrivateKey[0] == 'L' || teePrivateKey[0] == '5') {
		signer, err = chain.AccountFromWIF(teePrivateKey)
	} else {
		// Remove 0x prefix if present and try hex
		teePrivateKey = strings.TrimPrefix(strings.TrimPrefix(teePrivateKey, "0x"), "0X")
		signer, err = chain.AccountFromPrivateKey(teePrivateKey)
	}
	if err != nil {
		return nil, fmt.Errorf("create signer from TEE_PRIVATE_KEY: %w", err)
	}

	// Decode NEF from base64
	nefBytes, err := base64.StdEncoding.DecodeString(nefBase64)
	if err != nil {
		return nil, fmt.Errorf("decode nef base64: %w", err)
	}

	// Build deployment parameters
	// ContractManagement.deploy expects: (ByteArray nefFile, ByteArray manifest, Any data)
	// The manifest must be passed as ByteArray (UTF-8 bytes), not String
	params := []chain.ContractParam{
		chain.NewByteArrayParam(nefBytes),
		chain.NewByteArrayParam([]byte(manifestJSON)),
	}
	if data != nil {
		params = append(params, chain.NewAnyParam())
	}

	// ContractManagement native contract hash
	contractMgmtHash := "0xfffdc93764dbaddd97c48f252a53ea4643faa3fd"

	// Simulate deployment first
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractMgmtHash, "deploy", params, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("deployment simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("deployment simulation faulted: %s", invokeResult.Exception)
	}

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account":      "master",
		"gas_estimate": invokeResult.GasConsumed,
	}).Info("deployment simulation passed, building transaction")

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, transaction.CalledByEntry)
	if err != nil {
		return nil, fmt.Errorf("build deployment transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast deployment: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account": "master",
		"tx_hash": txHashString,
	}).Info("deployment transaction broadcast, waiting for confirmation")

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
			"tx_hash": txHashString,
		}).Error("failed to get application log")
		return nil, fmt.Errorf("wait for deployment execution (tx: %s): %w", txHashString, err)
	}

	// Extract contract hash from deployment result
	contractHash := ""
	if appLog != nil && len(appLog.Executions) > 0 {
		exec := appLog.Executions[0]
		if exec.VMState != "HALT" {
			s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
				"tx_hash":   txHashString,
				"vm_state":  exec.VMState,
				"exception": exec.Exception,
			}).Error("deployment transaction failed")
			return nil, fmt.Errorf("deployment failed (tx: %s) with state: %s, exception: %s", txHashString, exec.VMState, exec.Exception)
		}
		// Contract hash is typically in the first notification or stack result
		// The stack item contains the deployed contract state as a struct
		if len(exec.Stack) > 0 {
			// Try to extract hash from the stack item's Value field
			var valueMap map[string]any
			if err := json.Unmarshal(exec.Stack[0].Value, &valueMap); err == nil {
				if h, ok := valueMap["hash"].(string); ok {
					contractHash = h
				}
			}
		}
	}

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account":       "master",
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("contract deployed with master wallet")

	return &DeployMasterResponse{
		TxHash:       txHashString,
		ContractHash: contractHash,
		GasConsumed:  invokeResult.GasConsumed,
		AccountID:    "master",
	}, nil
}
